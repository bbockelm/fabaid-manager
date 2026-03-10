package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/db"
	"github.com/bbockelm/fabaid-manager/internal/models"
)

// maxLLMRounds limits total LLM call rounds (tool calls + retries).
const maxLLMRounds = 25

// maxRetries is how many times we retry on LLM error.
const maxRetries = 2

// ActionRecord captures a single tool call + its result for the audit trail.
type ActionRecord struct {
	ToolName  string `json:"tool_name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

// ProcessorContext holds all the state for a processing run.
type ProcessorContext struct {
	RunID      string
	EntityType string
	EntityID   string
	GrantID    string
	queries    *db.Queries
	client     *Client
	run        *models.DocumentProcessingRun

	// Existing state for LLM context
	existingPersonnel  []models.Personnel
	existingBudgets    []models.InstitutionBudget
	existingLineItems  map[string][]models.BudgetLineItem // budget_id -> items
	existingFringe     []models.InstitutionFringeRate
	existingOverhead   []models.OverheadRate

	// Grant/subaward info
	grantInfo    *models.Grant
	subawardInfo *models.Subaward

	// Actions recorded during processing
	actions []ActionRecord

	// Running token counts
	promptTokens     int
	completionTokens int

	// Optional user instructions appended to the LLM prompt
	UserPrompt string
}

// NewProcessorContext creates a processing context.
func NewProcessorContext(
	queries *db.Queries,
	client *Client,
	run *models.DocumentProcessingRun,
) *ProcessorContext {
	return &ProcessorContext{
		RunID:         run.ID,
		EntityType:    run.EntityType,
		EntityID:      run.EntityID,
		queries:       queries,
		client:        client,
		run:           run,
		existingLineItems: make(map[string][]models.BudgetLineItem),
	}
}

// Process runs the full document processing pipeline.
func (pc *ProcessorContext) Process(ctx context.Context, extractedMarkdown string) error {
	log.Info().Str("run_id", pc.RunID).Str("entity", pc.EntityType+"/"+pc.EntityID).Msg("Starting document processing")
	now := time.Now()
	pc.run.StartedAt = &now
	pc.run.Status = "extracting"
	pc.run.StatusDetail = "Loading institution context..."
	pc.updateRun(ctx)

	// Load existing institution data for context
	if err := pc.loadExistingData(ctx); err != nil {
		log.Error().Err(err).Str("run_id", pc.RunID).Msg("Failed to load existing data")
		return pc.failRun(ctx, fmt.Errorf("loading existing data: %w", err))
	}
	log.Info().Str("run_id", pc.RunID).Int("personnel", len(pc.existingPersonnel)).Int("budgets", len(pc.existingBudgets)).Msg("Loaded existing data")

	// Build the system prompt with institution context
	systemPrompt := pc.buildSystemPrompt()

	// Build the user message with extracted document data
	var userMsgBuilder strings.Builder
	userMsgBuilder.WriteString("Please process the following budget document and create/update the budget in our system.\n\n")
	if pc.UserPrompt != "" {
		userMsgBuilder.WriteString("## Additional Instructions from the User\n\n")
		userMsgBuilder.WriteString(pc.UserPrompt)
		userMsgBuilder.WriteString("\n\n")
	}
	userMsgBuilder.WriteString(extractedMarkdown)
	userMessage := userMsgBuilder.String()

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	pc.run.Status = "processing"
	pc.run.StatusDetail = "Sending to LLM for analysis..."
	pc.updateRun(ctx)
	log.Info().Str("run_id", pc.RunID).Int("system_prompt_len", len(systemPrompt)).Int("user_msg_len", len(userMessage)).Msg("Sending to LLM")

	tools := BudgetTools()
	summaryReceived := false
	retries := 0

	for round := 0; round < maxLLMRounds; round++ {
		log.Info().Str("run_id", pc.RunID).Int("round", round+1).Int("messages", len(messages)).Msg("LLM round starting")
		pc.run.StatusDetail = fmt.Sprintf("LLM round %d...", round+1)
		pc.updateRun(ctx)

		var assistantMsg *Message
		var usage *Usage
		var err error

		// Always use streaming so we can update status_detail during the LLM call.
		// The polling frontend reads status_detail from DB periodically.
		toolNames := make(map[int]string) // track tool call names by index
		assistantMsg, usage, err = pc.client.ChatCompletionStream(messages, tools, func(event StreamEvent) {
			for _, ch := range event.Choices {
				for _, tc := range ch.Delta.ToolCalls {
					if tc.Function.Name != "" {
						toolNames[tc.Index] = tc.Function.Name
						detail := fmt.Sprintf("Building call to %s...", tc.Function.Name)
						pc.run.StatusDetail = detail
						pc.updateRun(ctx)
						log.Debug().Str("run_id", pc.RunID).Str("tool", tc.Function.Name).Msg("LLM building tool call")
					}
				}
				if ch.FinishReason != nil && *ch.FinishReason == "tool_calls" {
					names := make([]string, 0, len(toolNames))
					for _, n := range toolNames {
						names = append(names, n)
					}
					detail := fmt.Sprintf("LLM finished — executing %d tool call(s): %s", len(names), strings.Join(names, ", "))
					pc.run.StatusDetail = detail
					pc.updateRun(ctx)
					log.Info().Str("run_id", pc.RunID).Strs("tools", names).Msg("LLM finished streaming, executing tool calls")
				}
			}
		})

		if err != nil {
			retries++
			log.Warn().Err(err).Int("retry", retries).Msg("LLM call failed")
			if retries > maxRetries {
				return pc.failRun(ctx, fmt.Errorf("LLM failed after %d retries: %w", retries, err))
			}
			pc.run.StatusDetail = fmt.Sprintf("LLM error (retry %d/%d): %s", retries, maxRetries, err)
			pc.updateRun(ctx)
			// Add error context to help the LLM on retry
			messages = append(messages, Message{Role: "user", Content: fmt.Sprintf("The previous request failed with error: %s. Please try again.", err)})
			continue
		}

		if usage != nil {
			pc.promptTokens += usage.PromptTokens
			pc.completionTokens += usage.CompletionTokens
		}

		// Add assistant message to conversation
		messages = append(messages, *assistantMsg)

		// If no tool calls, the LLM is done
		if len(assistantMsg.ToolCalls) == 0 {
			break
		}

		// Execute tool calls
		pc.run.Status = "applying"
		execToolNames := make([]string, 0, len(assistantMsg.ToolCalls))
		for _, tc := range assistantMsg.ToolCalls {
			execToolNames = append(execToolNames, tc.Function.Name)
		}
		pc.run.StatusDetail = fmt.Sprintf("Executing %d tool call(s): %s", len(assistantMsg.ToolCalls), strings.Join(execToolNames, ", "))
		pc.updateRun(ctx)
		log.Info().Str("run_id", pc.RunID).Strs("tools", execToolNames).Msg("Executing tool calls")

		for i, tc := range assistantMsg.ToolCalls {
			detail := fmt.Sprintf("Executing tool %d/%d: %s", i+1, len(assistantMsg.ToolCalls), tc.Function.Name)
			pc.run.StatusDetail = detail
			pc.updateRun(ctx)
			log.Info().Str("run_id", pc.RunID).Str("tool", tc.Function.Name).Int("index", i+1).Int("total", len(assistantMsg.ToolCalls)).Msg("Executing tool call")

			result, execErr := pc.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)

			action := ActionRecord{
				ToolName:  tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Result:    result,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			if execErr != nil {
				action.Error = execErr.Error()
				result = fmt.Sprintf("Error: %s", execErr)
				log.Warn().Str("run_id", pc.RunID).Str("tool", tc.Function.Name).Err(execErr).Msg("Tool call failed")
			} else {
				log.Info().Str("run_id", pc.RunID).Str("tool", tc.Function.Name).Msg("Tool call succeeded")
			}
			pc.actions = append(pc.actions, action)

			if tc.Function.Name == "report_summary" && execErr == nil {
				summaryReceived = true
			}

			// Add tool result message
			messages = append(messages, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}

	// If no summary was reported by the LLM, ask for one
	if !summaryReceived {
		pc.run.StatusDetail = "Requesting summary from LLM..."
		pc.updateRun(ctx)
		log.Info().Str("run_id", pc.RunID).Msg("Requesting summary from LLM")
		messages = append(messages, Message{
			Role:    "user",
			Content: "Please call the report_summary tool with a markdown summary of everything you did and what needs review.",
		})

		resp, err := pc.client.ChatCompletion(messages, tools)
		if err == nil && len(resp.Choices) > 0 {
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				if tc.Function.Name == "report_summary" {
					pc.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)
				}
			}
			pc.promptTokens += resp.Usage.PromptTokens
			pc.completionTokens += resp.Usage.CompletionTokens
		}
	}

	// Finalize the run
	completed := time.Now()
	pc.run.CompletedAt = &completed
	pc.run.Status = "completed"
	pc.run.StatusDetail = "Processing complete"
	log.Info().Str("run_id", pc.RunID).Int("actions", len(pc.actions)).Int("prompt_tokens", pc.promptTokens).Int("completion_tokens", pc.completionTokens).Msg("Document processing completed")
	pc.run.PromptTokens = pc.promptTokens
	pc.run.CompletionTokens = pc.completionTokens

	actionsJSON, _ := json.Marshal(pc.actions)
	pc.run.ActionsTaken = string(actionsJSON)

	convJSON, _ := json.Marshal(messages)
	pc.run.Conversation = string(convJSON)

	pc.updateRun(ctx)
	log.Info().Str("run_id", pc.RunID).Msg("Processing complete")
	return nil
}

// executeTool dispatches a tool call and returns the result string.
func (pc *ProcessorContext) executeTool(ctx context.Context, name, argsJSON string) (string, error) {
	switch name {
	case "create_budget":
		return pc.toolCreateBudget(ctx, argsJSON)
	case "create_line_item":
		return pc.toolCreateLineItem(ctx, argsJSON)
	case "update_line_item":
		return pc.toolUpdateLineItem(ctx, argsJSON)
	case "delete_line_item":
		return pc.toolDeleteLineItem(ctx, argsJSON)
	case "create_personnel":
		return pc.toolCreatePersonnel(ctx, argsJSON)
	case "update_personnel":
		return pc.toolUpdatePersonnel(ctx, argsJSON)
	case "upsert_fringe_rate":
		return pc.toolUpsertFringeRate(ctx, argsJSON)
	case "create_overhead_rate":
		return pc.toolCreateOverheadRate(ctx, argsJSON)
	case "update_overhead_rate":
		return pc.toolUpdateOverheadRate(ctx, argsJSON)
	case "update_subaward":
		return pc.toolUpdateSubaward(ctx, argsJSON)
	case "report_summary":
		return pc.toolReportSummary(ctx, argsJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Tool implementations ---

func (pc *ProcessorContext) toolCreateBudget(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		FiscalYear int     `json:"fiscal_year"`
		Budget     float64 `json:"budget"`
		Notes      string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	b := models.InstitutionBudget{
		EntityType: pc.EntityType,
		EntityID:   pc.EntityID,
		FiscalYear: args.FiscalYear,
		Budget:     args.Budget,
		Notes:      args.Notes,
		Status:     "draft",
	}
	if err := pc.queries.CreateInstitutionBudget(ctx, &b); err != nil {
		return "", fmt.Errorf("creating budget: %w", err)
	}

	return fmt.Sprintf(`{"budget_id": "%s", "fiscal_year": %d, "status": "draft"}`, b.ID, b.FiscalYear), nil
}

func (pc *ProcessorContext) toolCreateLineItem(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		FiscalYear   int     `json:"fiscal_year"`
		LineType     string  `json:"line_type"`
		Description  string  `json:"description"`
		PersonnelID  string  `json:"personnel_id"`
		EffortMonths float64 `json:"effort_months"`
		Amount       float64 `json:"amount"`
		Notes        string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Find the latest draft budget for this fiscal year
	budgetID, err := pc.findBudgetForYear(ctx, args.FiscalYear)
	if err != nil {
		return "", err
	}

	// Find the overhead rate (use first one for now)
	var overheadRateID *string
	if len(pc.existingOverhead) > 0 {
		overheadRateID = &pc.existingOverhead[0].ID
	}

	var personnelID *string
	if args.PersonnelID != "" {
		personnelID = &args.PersonnelID
	}

	item := models.BudgetLineItem{
		InstitutionBudgetID: budgetID,
		LineType:            args.LineType,
		Description:         args.Description,
		PersonnelID:         personnelID,
		EffortMonths:        args.EffortMonths,
		Amount:              args.Amount,
		OverheadRateID:      overheadRateID,
		Notes:               args.Notes,
	}

	if err := pc.queries.CreateBudgetLineItem(ctx, &item); err != nil {
		return "", fmt.Errorf("creating line item: %w", err)
	}

	return fmt.Sprintf(`{"line_item_id": "%s", "budget_id": "%s", "amount": %.2f}`, item.ID, budgetID, args.Amount), nil
}

func (pc *ProcessorContext) toolUpdateLineItem(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		LineItemID   string   `json:"line_item_id"`
		Description  *string  `json:"description"`
		PersonnelID  *string  `json:"personnel_id"`
		EffortMonths *float64 `json:"effort_months"`
		Amount       *float64 `json:"amount"`
		Notes        *string  `json:"notes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	existing, err := pc.queries.GetBudgetLineItem(ctx, args.LineItemID)
	if err != nil {
		return "", fmt.Errorf("line item not found: %w", err)
	}

	// Verify the line item belongs to a budget owned by this entity
	if !pc.ownsBudget(existing.InstitutionBudgetID) {
		return "", fmt.Errorf("line item %s does not belong to this entity's budget", args.LineItemID)
	}

	if args.Description != nil {
		existing.Description = *args.Description
	}
	if args.PersonnelID != nil {
		existing.PersonnelID = args.PersonnelID
	}
	if args.EffortMonths != nil {
		existing.EffortMonths = *args.EffortMonths
	}
	if args.Amount != nil {
		existing.Amount = *args.Amount
	}
	if args.Notes != nil {
		existing.Notes = *args.Notes
	}

	if err := pc.queries.UpdateBudgetLineItem(ctx, existing); err != nil {
		return "", fmt.Errorf("updating line item: %w", err)
	}

	return fmt.Sprintf(`{"line_item_id": "%s", "amount": %.2f, "status": "updated"}`, existing.ID, existing.Amount), nil
}

func (pc *ProcessorContext) toolDeleteLineItem(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		LineItemID string `json:"line_item_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Verify the line item belongs to a budget owned by this entity
	existing, err := pc.queries.GetBudgetLineItem(ctx, args.LineItemID)
	if err != nil {
		return "", fmt.Errorf("line item not found: %w", err)
	}
	if !pc.ownsBudget(existing.InstitutionBudgetID) {
		return "", fmt.Errorf("line item %s does not belong to this entity's budget", args.LineItemID)
	}

	if err := pc.queries.DeleteBudgetLineItem(ctx, args.LineItemID); err != nil {
		return "", fmt.Errorf("deleting line item: %w", err)
	}

	return `{"status": "deleted"}`, nil
}

func (pc *ProcessorContext) toolCreatePersonnel(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Name         string  `json:"name"`
		Role         string  `json:"role"`
		Title        string  `json:"title"`
		Institution  string  `json:"institution"`
		AnnualSalary float64 `json:"annual_salary"`
		FundedMonths float64 `json:"funded_months"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	p := models.Personnel{
		GrantID:      pc.GrantID,
		Name:         args.Name,
		Role:         args.Role,
		Title:        args.Title,
		Institution:  args.Institution,
		AnnualSalary: args.AnnualSalary,
		FundedMonths: args.FundedMonths,
	}

	if err := pc.queries.CreatePersonnel(ctx, &p); err != nil {
		return "", fmt.Errorf("creating personnel: %w", err)
	}

	return fmt.Sprintf(`{"personnel_id": "%s", "name": "%s"}`, p.ID, p.Name), nil
}

func (pc *ProcessorContext) toolUpdatePersonnel(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		PersonnelID  string   `json:"personnel_id"`
		Name         *string  `json:"name"`
		Role         *string  `json:"role"`
		Title        *string  `json:"title"`
		AnnualSalary *float64 `json:"annual_salary"`
		FundedMonths *float64 `json:"funded_months"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	existing, err := pc.queries.GetPersonnel(ctx, args.PersonnelID)
	if err != nil {
		return "", fmt.Errorf("personnel not found: %w", err)
	}

	// Verify the personnel belongs to this grant
	if existing.GrantID != pc.GrantID {
		return "", fmt.Errorf("personnel %s does not belong to this grant", args.PersonnelID)
	}

	if args.Name != nil {
		existing.Name = *args.Name
	}
	if args.Role != nil {
		existing.Role = *args.Role
	}
	if args.Title != nil {
		existing.Title = *args.Title
	}
	if args.AnnualSalary != nil {
		existing.AnnualSalary = *args.AnnualSalary
	}
	if args.FundedMonths != nil {
		existing.FundedMonths = *args.FundedMonths
	}

	if err := pc.queries.UpdatePersonnel(ctx, existing); err != nil {
		return "", fmt.Errorf("updating personnel: %w", err)
	}

	return fmt.Sprintf(`{"personnel_id": "%s", "status": "updated"}`, existing.ID), nil
}

func (pc *ProcessorContext) toolUpsertFringeRate(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		FiscalYear int     `json:"fiscal_year"`
		RateName   string  `json:"rate_name"`
		Rate       float64 `json:"rate"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	rate := models.InstitutionFringeRate{
		EntityType: pc.EntityType,
		EntityID:   pc.EntityID,
		FiscalYear: args.FiscalYear,
		RateName:   args.RateName,
		Rate:       args.Rate,
	}

	if err := pc.queries.UpsertFringeRate(ctx, &rate); err != nil {
		return "", fmt.Errorf("upserting fringe rate: %w", err)
	}

	return fmt.Sprintf(`{"rate_name": "%s", "fiscal_year": %d, "rate": %.4f}`, args.RateName, args.FiscalYear, args.Rate), nil
}

func (pc *ProcessorContext) toolCreateOverheadRate(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		RateName    string  `json:"rate_name"`
		Rate        float64 `json:"rate"`
		Description string  `json:"description"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	rate := models.OverheadRate{
		EntityType:  pc.EntityType,
		EntityID:    pc.EntityID,
		RateName:    args.RateName,
		Rate:        args.Rate,
		Description: args.Description,
	}

	if err := pc.queries.CreateOverheadRate(ctx, &rate); err != nil {
		return "", fmt.Errorf("creating overhead rate: %w", err)
	}

	return fmt.Sprintf(`{"overhead_rate_id": "%s", "rate_name": "%s"}`, rate.ID, args.RateName), nil
}

func (pc *ProcessorContext) toolUpdateOverheadRate(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		OverheadRateID string   `json:"overhead_rate_id"`
		RateName       *string  `json:"rate_name"`
		Rate           *float64 `json:"rate"`
		Description    *string  `json:"description"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	existing, err := pc.queries.GetOverheadRate(ctx, args.OverheadRateID)
	if err != nil {
		return "", fmt.Errorf("overhead rate not found: %w", err)
	}

	// Verify the overhead rate belongs to this entity
	if existing.EntityType != pc.EntityType || existing.EntityID != pc.EntityID {
		return "", fmt.Errorf("overhead rate %s does not belong to this entity", args.OverheadRateID)
	}

	if args.RateName != nil {
		existing.RateName = *args.RateName
	}
	if args.Rate != nil {
		existing.Rate = *args.Rate
	}
	if args.Description != nil {
		existing.Description = *args.Description
	}

	if err := pc.queries.UpdateOverheadRate(ctx, existing); err != nil {
		return "", fmt.Errorf("updating overhead rate: %w", err)
	}

	return fmt.Sprintf(`{"overhead_rate_id": "%s", "status": "updated"}`, existing.ID), nil
}

func (pc *ProcessorContext) toolUpdateSubaward(ctx context.Context, argsJSON string) (string, error) {
	if pc.EntityType != "subaward" || pc.subawardInfo == nil {
		return "", fmt.Errorf("cannot update subaward: entity is not a subaward")
	}

	var args struct {
		TotalAmount          *float64 `json:"total_amount"`
		SalaryEscalationRate *float64 `json:"salary_escalation_rate"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.TotalAmount != nil {
		pc.subawardInfo.TotalAmount = *args.TotalAmount
	}
	if args.SalaryEscalationRate != nil {
		pc.subawardInfo.SalaryEscalationRate = *args.SalaryEscalationRate
	}

	if err := pc.queries.UpdateSubaward(ctx, pc.subawardInfo); err != nil {
		return "", fmt.Errorf("updating subaward: %w", err)
	}

	return fmt.Sprintf(`{"total_amount": %.2f, "salary_escalation_rate": %.4f}`, pc.subawardInfo.TotalAmount, pc.subawardInfo.SalaryEscalationRate), nil
}

func (pc *ProcessorContext) toolReportSummary(_ context.Context, argsJSON string) (string, error) {
	var args struct {
		SummaryMarkdown string `json:"summary_markdown"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	pc.run.SummaryMD = args.SummaryMarkdown
	return `{"status": "summary_recorded"}`, nil
}

// --- Helpers ---

func (pc *ProcessorContext) loadExistingData(ctx context.Context) error {
	// Determine grant ID
	if pc.EntityType == "grant" {
		pc.GrantID = pc.EntityID
		grant, err := pc.queries.GetGrant(ctx, pc.EntityID)
		if err != nil {
			return fmt.Errorf("getting grant: %w", err)
		}
		pc.grantInfo = grant
	} else {
		sub, err := pc.queries.GetSubaward(ctx, pc.EntityID)
		if err != nil {
			return fmt.Errorf("getting subaward: %w", err)
		}
		pc.subawardInfo = sub
		pc.GrantID = sub.GrantID
		grant, err := pc.queries.GetGrant(ctx, sub.GrantID)
		if err != nil {
			return fmt.Errorf("getting grant: %w", err)
		}
		pc.grantInfo = grant
	}

	// Load personnel for this institution
	allPersonnel, err := pc.queries.ListPersonnel(ctx, pc.GrantID)
	if err != nil {
		return fmt.Errorf("listing personnel: %w", err)
	}
	instName := pc.institutionName()
	for _, p := range allPersonnel {
		if p.Institution == instName {
			pc.existingPersonnel = append(pc.existingPersonnel, p)
		}
	}

	// Load budgets
	budgets, err := pc.queries.ListInstitutionBudgets(ctx, pc.EntityType, pc.EntityID)
	if err != nil {
		return fmt.Errorf("listing budgets: %w", err)
	}
	pc.existingBudgets = budgets

	// Load line items for each budget
	for _, b := range budgets {
		if !b.IsLatest {
			continue
		}
		items, err := pc.queries.ListBudgetLineItems(ctx, b.ID)
		if err != nil {
			return fmt.Errorf("listing line items: %w", err)
		}
		pc.existingLineItems[b.ID] = items
	}

	// Load fringe rates
	fringe, err := pc.queries.ListFringeRates(ctx, pc.EntityType, pc.EntityID)
	if err != nil {
		return fmt.Errorf("listing fringe rates: %w", err)
	}
	pc.existingFringe = fringe

	// Load overhead rates
	overhead, err := pc.queries.ListOverheadRates(ctx, pc.EntityType, pc.EntityID)
	if err != nil {
		return fmt.Errorf("listing overhead rates: %w", err)
	}
	pc.existingOverhead = overhead

	return nil
}

func (pc *ProcessorContext) institutionName() string {
	if pc.EntityType == "grant" && pc.grantInfo != nil {
		return pc.grantInfo.Institution
	}
	if pc.subawardInfo != nil {
		return pc.subawardInfo.Institution
	}
	return ""
}

func (pc *ProcessorContext) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(`You are a budget processing assistant for the FabAID grant management system.

Your job is to read a budget document (spreadsheet or PDF) and create or update budget records in our system using the provided tools.

## Rules
1. Extract all salary, fringe, travel, equipment, supplies, and other direct cost line items for EACH fiscal year present in the document.
2. Overhead/indirect costs are NOT stored as line items — they are computed automatically from the overhead rate applied to each line item. Do NOT create line items for overhead/indirect costs.
3. For personnel salary lines, link them to existing personnel records when possible (by matching names). If a person doesn't exist yet, create them first.
4. For fringe benefit lines, set the line_type to "fringe" and link to the same personnel_id. CRITICAL: You MUST create one fringe line item per person per year. NEVER combine fringe benefits for multiple people into a single line item. If the spreadsheet shows a single "Total Fringe" row, split it proportionally across each person based on their salary proportion. Each person must have their own fringe line item linked to their personnel_id.
5. If a budget already exists for a fiscal year, update the existing line items rather than creating duplicates. Delete obsolete items.
6. Round dollar amounts to 2 decimal places.
11. CRITICAL: Cross-verify personnel base salaries against existing records BEFORE creating salary line items. Steps:
    a. Look up each person's annual_salary in the existing personnel records above.
    b. Compare it to the base salary shown in the document for that person.
    c. If the document shows a DIFFERENT base salary (more than a salary escalation adjustment of ~3-5%), this likely means the institution has changed the employee's salary. Flag this prominently in your summary as "BASE SALARY CHANGE DETECTED" requiring human review.
    d. Use the DOCUMENT's salary figures for the line items (since the document represents the latest budget), but DO NOT call update_personnel to change the stored base salary — leave that for human review.
    e. If the personnel record doesn't exist yet, create it with the salary from the document.
7. Small rounding differences of $1–3 between the document totals and the sum of individual line items are expected and acceptable. Do NOT flag these as discrepancies or request human review for them.
8. If fringe rates differ from what's stored, update them using upsert_fringe_rate.
9. Derive the salary escalation (inflation) rate by comparing the same person's salary across consecutive years. If it differs from the currently stored rate, call update_subaward (for subawards) to correct it. For example, if a senior person's salary goes from $100,000 in Y1 to $104,000 in Y2, the escalation rate is 4%.
10. When you're done with all budget changes, call report_summary with a markdown summary including:
   - What you created/updated/deleted
   - Confidence level for key figures (high/medium/low)
   - Any items that need human review
   - Totals per year (or overall verification)

## Important Context
`)

	// Grant info
	sb.WriteString(fmt.Sprintf("**Grant**: %s — %s\n", pc.grantInfo.AwardNumber, pc.grantInfo.Title))
	sb.WriteString(fmt.Sprintf("**Grant Institution**: %s\n", pc.grantInfo.Institution))
	sb.WriteString(fmt.Sprintf("**Grant Period**: %s to %s\n", pc.grantInfo.StartDate, pc.grantInfo.EndDate))
	sb.WriteString(fmt.Sprintf("**Grant Salary Escalation Rate**: %.2f%%\n\n", pc.grantInfo.SalaryEscalationRate*100))

	// Subaward info
	if pc.subawardInfo != nil {
		sb.WriteString(fmt.Sprintf("**Subaward Institution**: %s\n", pc.subawardInfo.Institution))
		sb.WriteString(fmt.Sprintf("**Subaward PI**: %s\n", pc.subawardInfo.PIName))
		sb.WriteString(fmt.Sprintf("**Subaward Total Amount**: $%.2f\n", pc.subawardInfo.TotalAmount))
		sb.WriteString(fmt.Sprintf("**Subaward Salary Escalation Rate**: %.2f%%\n", pc.subawardInfo.SalaryEscalationRate*100))
		sb.WriteString(fmt.Sprintf("**Subaward Period**: %s to %s\n\n", pc.subawardInfo.StartDate, pc.subawardInfo.EndDate))
	}

	// Entity type/ID
	sb.WriteString(fmt.Sprintf("**Entity Type**: %s\n", pc.EntityType))
	sb.WriteString(fmt.Sprintf("**Entity ID**: %s\n\n", pc.EntityID))

	// Existing personnel
	if len(pc.existingPersonnel) > 0 {
		sb.WriteString("## Existing Personnel for this Institution\n\n")
		for _, p := range pc.existingPersonnel {
			sb.WriteString(fmt.Sprintf("- **%s** (ID: `%s`): role=%s, title=%s, annual_salary=$%.2f, funded_months=%.1f\n",
				p.Name, p.ID, p.Role, p.Title, p.AnnualSalary, p.FundedMonths))
		}
		sb.WriteString("\n")
	}

	// Existing fringe rates
	if len(pc.existingFringe) > 0 {
		sb.WriteString("## Existing Fringe Rates\n\n")
		for _, f := range pc.existingFringe {
			sb.WriteString(fmt.Sprintf("- Year %d: %s = %.4f (%.2f%%)\n", f.FiscalYear, f.RateName, f.Rate, f.Rate*100))
		}
		sb.WriteString("\n")
	}

	// Existing overhead rates
	if len(pc.existingOverhead) > 0 {
		sb.WriteString("## Existing Overhead Rates\n\n")
		for _, o := range pc.existingOverhead {
			sb.WriteString(fmt.Sprintf("- **%s** (ID: `%s`): %.4f (%.2f%%) — %s\n", o.RateName, o.ID, o.Rate, o.Rate*100, o.Description))
		}
		sb.WriteString("\n")
	}

	// Existing budgets and line items
	if len(pc.existingBudgets) > 0 {
		sb.WriteString("## Existing Budgets\n\n")
		for _, b := range pc.existingBudgets {
			if !b.IsLatest {
				continue
			}
			sb.WriteString(fmt.Sprintf("### Year %d (Budget ID: `%s`, version %d, status: %s, total: $%.2f)\n\n",
				b.FiscalYear, b.ID, b.Version, b.Status, b.Budget))

			items := pc.existingLineItems[b.ID]
			if len(items) > 0 {
				for _, li := range items {
					personnelRef := ""
					if li.PersonnelID != nil {
						personnelRef = fmt.Sprintf(", personnel_id=`%s`", *li.PersonnelID)
					}
					sb.WriteString(fmt.Sprintf("- Line item `%s`: type=%s, desc=\"%s\", amount=$%.2f, effort_months=%.2f%s, notes=\"%s\"\n",
						li.ID, li.LineType, li.Description, li.Amount, li.EffortMonths, personnelRef, li.Notes))
				}
			} else {
				sb.WriteString("  (no line items)\n")
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// ownsBudget checks whether a budget ID belongs to the current entity's budgets.
func (pc *ProcessorContext) ownsBudget(budgetID string) bool {
	for _, b := range pc.existingBudgets {
		if b.ID == budgetID {
			return true
		}
	}
	return false
}

func (pc *ProcessorContext) findBudgetForYear(ctx context.Context, fiscalYear int) (string, error) {
	for _, b := range pc.existingBudgets {
		if b.FiscalYear == fiscalYear && b.IsLatest {
			return b.ID, nil
		}
	}

	// Budget doesn't exist yet — create one
	b := models.InstitutionBudget{
		EntityType: pc.EntityType,
		EntityID:   pc.EntityID,
		FiscalYear: fiscalYear,
		Status:     "draft",
	}
	if err := pc.queries.CreateInstitutionBudget(ctx, &b); err != nil {
		return "", fmt.Errorf("creating budget for year %d: %w", fiscalYear, err)
	}
	pc.existingBudgets = append(pc.existingBudgets, b)
	return b.ID, nil
}

func (pc *ProcessorContext) failRun(ctx context.Context, err error) error {
	pc.run.Status = "failed"
	pc.run.ErrorMsg = err.Error()
	completed := time.Now()
	pc.run.CompletedAt = &completed
	pc.run.PromptTokens = pc.promptTokens
	pc.run.CompletionTokens = pc.completionTokens

	actionsJSON, _ := json.Marshal(pc.actions)
	pc.run.ActionsTaken = string(actionsJSON)

	pc.updateRun(ctx)
	return err
}

func (pc *ProcessorContext) updateRun(ctx context.Context) {
	if err := pc.queries.UpdateDocumentProcessingRun(ctx, pc.run); err != nil {
		log.Error().Err(err).Str("run_id", pc.run.ID).Msg("Failed to update processing run")
	
	}
}