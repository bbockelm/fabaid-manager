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

// InvoiceCodingContext holds state for an AI invoice-coding run. The agent reads
// an extracted invoice document and proposes a *draft* coding: a set of expense
// lines, each mapped to a budget category and split across WBS areas. It never
// finalizes — finalizing is a deliberate human action.
type InvoiceCodingContext struct {
	RunID   string
	queries *db.Queries
	client  *Client
	run     *models.DocumentProcessingRun
	invoice *models.Invoice

	grantID   string
	wbsAreas  []models.WBSArea
	personnel []models.Personnel
	budgetLI  []models.BudgetLineItem

	actions          []ActionRecord
	promptTokens     int
	completionTokens int
	UserPrompt       string
}

// NewInvoiceCodingContext creates an invoice-coding processing context.
func NewInvoiceCodingContext(queries *db.Queries, client *Client, run *models.DocumentProcessingRun, invoice *models.Invoice) *InvoiceCodingContext {
	return &InvoiceCodingContext{
		RunID:   run.ID,
		queries: queries,
		client:  client,
		run:     run,
		invoice: invoice,
	}
}

// expenseCategories are the valid line_type values the agent may assign.
var expenseCategories = []string{
	"personnel", "fringe", "travel", "equipment", "supplies", "contractual",
	"participant_support", "tuition", "indirect", "other", "uncategorized",
}

// Process runs the invoice-coding pipeline: load context, ask the LLM to break the
// invoice into coded expense lines (draft), and record a summary.
func (ic *InvoiceCodingContext) Process(ctx context.Context, extractedMarkdown string) error {
	log.Info().Str("run_id", ic.RunID).Str("invoice", ic.invoice.ID).Msg("Starting invoice coding")
	now := time.Now()
	ic.run.StartedAt = &now
	ic.run.Status = "processing"
	ic.run.StatusDetail = "Loading budget context..."
	ic.updateRun(ctx)

	if err := ic.loadContext(ctx); err != nil {
		return ic.failRun(ctx, fmt.Errorf("loading context: %w", err))
	}

	// Fresh proposal: clear any previously-proposed expense lines for this invoice.
	existing, _ := ic.queries.ListInvoiceExpenses(ctx, ic.invoice.ID)
	for _, e := range existing {
		_ = ic.queries.DeleteInvoiceExpense(ctx, e.ID)
	}
	// Mark the invoice as draft-coded up front; the agent only ever produces drafts.
	_ = ic.queries.SetInvoiceCodingStatus(ctx, ic.invoice.ID, "draft")

	messages := []Message{
		{Role: "system", Content: ic.buildSystemPrompt()},
		{Role: "user", Content: ic.buildUserMessage(extractedMarkdown)},
	}

	tools := InvoiceCodingTools()
	summaryReceived := false
	retries := 0

	for round := 0; round < maxLLMRounds; round++ {
		ic.run.StatusDetail = fmt.Sprintf("LLM round %d...", round+1)
		ic.updateRun(ctx)

		assistantMsg, usage, err := ic.client.ChatCompletionStream(messages, tools, func(event StreamEvent) {
			for _, ch := range event.Choices {
				for _, tc := range ch.Delta.ToolCalls {
					if tc.Function.Name != "" {
						ic.run.StatusDetail = fmt.Sprintf("Building call to %s...", tc.Function.Name)
						ic.updateRun(ctx)
					}
				}
			}
		})
		if err != nil {
			retries++
			if retries > maxRetries {
				return ic.failRun(ctx, fmt.Errorf("LLM failed after %d retries: %w", retries, err))
			}
			messages = append(messages, Message{Role: "user", Content: fmt.Sprintf("The previous request failed: %s. Please try again.", err)})
			continue
		}
		if usage != nil {
			ic.promptTokens += usage.PromptTokens
			ic.completionTokens += usage.CompletionTokens
		}
		messages = append(messages, *assistantMsg)
		if len(assistantMsg.ToolCalls) == 0 {
			break
		}

		ic.run.Status = "applying"
		for i, tc := range assistantMsg.ToolCalls {
			ic.run.StatusDetail = fmt.Sprintf("Executing tool %d/%d: %s", i+1, len(assistantMsg.ToolCalls), tc.Function.Name)
			ic.updateRun(ctx)

			result, execErr := ic.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)
			action := ActionRecord{
				ToolName: tc.Function.Name, Arguments: tc.Function.Arguments,
				Result: result, Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			if execErr != nil {
				action.Error = execErr.Error()
				result = fmt.Sprintf("Error: %s", execErr)
			}
			ic.actions = append(ic.actions, action)
			if tc.Function.Name == "report_invoice_summary" && execErr == nil {
				summaryReceived = true
			}
			messages = append(messages, Message{Role: "tool", ToolCallID: tc.ID, Content: result})
		}
	}

	if !summaryReceived {
		messages = append(messages, Message{Role: "user", Content: "Please call report_invoice_summary with a markdown summary of the proposed coding and anything left uncategorized."})
		if resp, err := ic.client.ChatCompletion(messages, tools); err == nil && len(resp.Choices) > 0 {
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				if tc.Function.Name == "report_invoice_summary" {
					_, _ = ic.executeTool(ctx, tc.Function.Name, tc.Function.Arguments)
				}
			}
			ic.promptTokens += resp.Usage.PromptTokens
			ic.completionTokens += resp.Usage.CompletionTokens
		}
	}

	completed := time.Now()
	ic.run.CompletedAt = &completed
	ic.run.Status = "completed"
	ic.run.StatusDetail = "Coding proposal complete (draft)"
	ic.run.PromptTokens = ic.promptTokens
	ic.run.CompletionTokens = ic.completionTokens
	actionsJSON, _ := json.Marshal(ic.actions)
	ic.run.ActionsTaken = string(actionsJSON)
	convJSON, _ := json.Marshal(messages)
	ic.run.Conversation = string(convJSON)
	ic.updateRun(ctx)
	log.Info().Str("run_id", ic.RunID).Int("actions", len(ic.actions)).Msg("Invoice coding completed")
	return nil
}

func (ic *InvoiceCodingContext) loadContext(ctx context.Context) error {
	// Resolve the grant ID (WBS areas and personnel are grant-scoped).
	ic.grantID = ic.invoice.EntityID
	if ic.invoice.EntityType == "subaward" {
		sub, err := ic.queries.GetSubaward(ctx, ic.invoice.EntityID)
		if err != nil {
			return fmt.Errorf("getting subaward: %w", err)
		}
		ic.grantID = sub.GrantID
	}
	var err error
	if ic.wbsAreas, err = ic.queries.ListWBSAreas(ctx, ic.grantID); err != nil {
		return err
	}
	if ic.personnel, err = ic.queries.ListPersonnel(ctx, ic.grantID); err != nil {
		return err
	}
	// Latest budget line items for this entity (planned categories to match against).
	budgets, err := ic.queries.ListInstitutionBudgets(ctx, ic.invoice.EntityType, ic.invoice.EntityID)
	if err == nil {
		for _, b := range budgets {
			if b.IsLatest {
				if items, err := ic.queries.ListBudgetLineItems(ctx, b.ID); err == nil {
					ic.budgetLI = append(ic.budgetLI, items...)
				}
			}
		}
	}
	return nil
}

func (ic *InvoiceCodingContext) buildSystemPrompt() string {
	var b strings.Builder
	b.WriteString("You are a grants-accounting assistant that codes an institution's invoice against a project budget.\n\n")
	b.WriteString("Your job: read the invoice document and break it into billed expense lines. For EACH line call add_expense_line with:\n")
	b.WriteString("- line_type: one of " + strings.Join(expenseCategories, ", ") + "\n")
	b.WriteString("- amount, description\n")
	b.WriteString("- wbs_allocations: split the line across WBS areas by percent (may be partial — leave a remainder rather than guessing)\n")
	b.WriteString("- budget_line_item_id when the line clearly corresponds to a planned budget line\n")
	b.WriteString("- personnel_id for salary/fringe lines tied to a known person\n")
	b.WriteString("Use line_type='equipment' for capital equipment purchases (these are excluded from recurring burn rate).\n")
	b.WriteString("Also call set_invoice_details once to record the invoice number, date, and billing period if the document shows them.\n\n")
	b.WriteString("CRITICAL RULES:\n")
	b.WriteString("1. You produce a DRAFT proposal only. You CANNOT finalize a coding — a human reviews and finalizes.\n")
	b.WriteString("2. When you are NOT confident how to categorize a cost, use line_type='uncategorized' and/or omit the WBS allocation. A visible uncategorized amount is far safer than a confident wrong coding.\n")
	b.WriteString("3. The sum of expense line amounts should equal the invoice total. If it doesn't reconcile, still record the lines and call out the discrepancy in your summary.\n")
	b.WriteString("4. Indirect/F&A costs use line_type='indirect'.\n\n")

	fmt.Fprintf(&b, "## Invoice being coded\nInstitution entity: %s (%s)\nInvoice total: $%.2f\n", ic.invoice.EntityID, ic.invoice.EntityType, ic.invoice.Amount)
	if ic.invoice.PeriodStart != nil && ic.invoice.PeriodEnd != nil {
		fmt.Fprintf(&b, "Period: %s to %s\n", *ic.invoice.PeriodStart, *ic.invoice.PeriodEnd)
	}

	b.WriteString("\n## WBS areas (use these IDs for wbs_allocations)\n")
	if len(ic.wbsAreas) == 0 {
		b.WriteString("(none defined)\n")
	}
	for _, a := range ic.wbsAreas {
		fmt.Fprintf(&b, "- %s | code %s | %s\n", a.ID, a.Code, a.Name)
	}

	if len(ic.personnel) > 0 {
		b.WriteString("\n## Personnel (use these IDs for personnel_id)\n")
		for _, p := range ic.personnel {
			fmt.Fprintf(&b, "- %s | %s (%s)\n", p.ID, p.Name, p.Role)
		}
	}

	if len(ic.budgetLI) > 0 {
		b.WriteString("\n## Planned budget line items (use these IDs for budget_line_item_id when a billed line matches)\n")
		for _, li := range ic.budgetLI {
			fmt.Fprintf(&b, "- %s | %s | %s | $%.2f\n", li.ID, li.LineType, li.Description, li.Amount)
		}
	}

	b.WriteString("\nWhen finished, call report_invoice_summary with a markdown summary noting the total coded, any uncategorized amount, and items needing human review.\n")
	return b.String()
}

func (ic *InvoiceCodingContext) buildUserMessage(extractedMarkdown string) string {
	var b strings.Builder
	b.WriteString("Code the following invoice into expense lines.\n\n")
	if ic.UserPrompt != "" {
		b.WriteString("## Additional instructions\n" + ic.UserPrompt + "\n\n")
	}
	b.WriteString(extractedMarkdown)
	return b.String()
}

func (ic *InvoiceCodingContext) executeTool(ctx context.Context, name, argsJSON string) (string, error) {
	switch name {
	case "set_invoice_details":
		return ic.toolSetInvoiceDetails(ctx, argsJSON)
	case "add_expense_line":
		return ic.toolAddExpenseLine(ctx, argsJSON)
	case "report_invoice_summary":
		return ic.toolReportSummary(argsJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (ic *InvoiceCodingContext) toolSetInvoiceDetails(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		InvoiceNumber *string `json:"invoice_number"`
		InvoiceDate   *string `json:"invoice_date"`
		PeriodStart   *string `json:"period_start"`
		PeriodEnd     *string `json:"period_end"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.InvoiceNumber != nil {
		ic.invoice.InvoiceNumber = *args.InvoiceNumber
	}
	if args.InvoiceDate != nil && *args.InvoiceDate != "" {
		ic.invoice.InvoiceDate = *args.InvoiceDate
	}
	if args.PeriodStart != nil && *args.PeriodStart != "" {
		ic.invoice.PeriodStart = args.PeriodStart
	}
	if args.PeriodEnd != nil && *args.PeriodEnd != "" {
		ic.invoice.PeriodEnd = args.PeriodEnd
	}
	if err := ic.queries.UpdateInvoice(ctx, ic.invoice); err != nil {
		return "", fmt.Errorf("updating invoice details: %w", err)
	}
	return `{"status":"invoice_details_updated"}`, nil
}

func (ic *InvoiceCodingContext) toolAddExpenseLine(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		LineType         string  `json:"line_type"`
		Description      string  `json:"description"`
		Amount           float64 `json:"amount"`
		PersonnelID      string  `json:"personnel_id"`
		BudgetLineItemID string  `json:"budget_line_item_id"`
		Notes            string  `json:"notes"`
		WBSAllocations   []struct {
			WBSAreaID string  `json:"wbs_area_id"`
			Percent   float64 `json:"percent"`
		} `json:"wbs_allocations"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	lineType := args.LineType
	if !contains(expenseCategories, lineType) {
		lineType = "uncategorized"
	}
	e := &models.InvoiceExpense{
		InvoiceID:   ic.invoice.ID,
		LineType:    lineType,
		Description: args.Description,
		Amount:      args.Amount,
		Notes:       args.Notes,
		SortOrder:   len(ic.actions),
	}
	if args.PersonnelID != "" {
		e.PersonnelID = &args.PersonnelID
	}
	if args.BudgetLineItemID != "" {
		e.BudgetLineItemID = &args.BudgetLineItemID
	}
	if err := ic.queries.CreateInvoiceExpense(ctx, e); err != nil {
		return "", fmt.Errorf("creating expense: %w", err)
	}
	// WBS allocations (may be partial; remainder is uncategorized).
	var allocs []models.InvoiceExpenseWBS
	for _, w := range args.WBSAllocations {
		if w.WBSAreaID == "" || w.Percent <= 0 {
			continue
		}
		allocs = append(allocs, models.InvoiceExpenseWBS{WBSAreaID: w.WBSAreaID, AllocationPercent: w.Percent})
	}
	if len(allocs) > 0 {
		if err := ic.queries.SetInvoiceExpenseWBS(ctx, e.ID, allocs); err != nil {
			return "", fmt.Errorf("setting expense WBS: %w", err)
		}
	}
	return fmt.Sprintf(`{"status":"expense_added","expense_id":%q,"line_type":%q,"amount":%.2f}`, e.ID, lineType, args.Amount), nil
}

func (ic *InvoiceCodingContext) toolReportSummary(argsJSON string) (string, error) {
	var args struct {
		SummaryMarkdown string `json:"summary_markdown"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	ic.run.SummaryMD = args.SummaryMarkdown
	return `{"status":"summary_recorded"}`, nil
}

func (ic *InvoiceCodingContext) updateRun(ctx context.Context) {
	if err := ic.queries.UpdateDocumentProcessingRun(ctx, ic.run); err != nil {
		log.Error().Err(err).Str("run_id", ic.run.ID).Msg("Failed to update invoice coding run")
	}
}

func (ic *InvoiceCodingContext) failRun(ctx context.Context, err error) error {
	ic.run.Status = "failed"
	ic.run.ErrorMsg = err.Error()
	completed := time.Now()
	ic.run.CompletedAt = &completed
	ic.run.PromptTokens = ic.promptTokens
	ic.run.CompletionTokens = ic.completionTokens
	actionsJSON, _ := json.Marshal(ic.actions)
	ic.run.ActionsTaken = string(actionsJSON)
	ic.updateRun(ctx)
	return err
}

// InvoiceCodingTools returns the tool definitions for the invoice-coding agent.
// Note: there is deliberately no tool to finalize a coding — the agent only drafts.
func InvoiceCodingTools() []Tool {
	return []Tool{
		makeTool("set_invoice_details", "Set the invoice header fields read from the document (invoice number, invoice/received date, and billing period). Call once if the document shows them.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"invoice_number": map[string]any{"type": "string", "description": "Invoice number/identifier"},
				"invoice_date":   map[string]any{"type": "string", "description": "Invoice/received date, YYYY-MM-DD"},
				"period_start":   map[string]any{"type": "string", "description": "Billing period start, YYYY-MM-DD"},
				"period_end":     map[string]any{"type": "string", "description": "Billing period end, YYYY-MM-DD"},
			},
			"required": []string{},
		})),
		makeTool("add_expense_line", "Add one coded expense line to the invoice (draft). Call once per billed line.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"line_type":           map[string]any{"type": "string", "enum": expenseCategories, "description": "Budget category; use 'uncategorized' when unsure, 'indirect' for F&A"},
				"description":         map[string]any{"type": "string", "description": "What this billed line is"},
				"amount":              map[string]any{"type": "number", "description": "Dollar amount of this line"},
				"personnel_id":        map[string]any{"type": "string", "description": "Personnel UUID for salary/fringe lines (optional)"},
				"budget_line_item_id": map[string]any{"type": "string", "description": "Planned budget line UUID this matches (optional)"},
				"notes":               map[string]any{"type": "string", "description": "Optional notes / rationale"},
				"wbs_allocations": map[string]any{
					"type":        "array",
					"description": "WBS split by percent; may be partial (remainder is uncategorized). Omit if unsure.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"wbs_area_id": map[string]any{"type": "string", "description": "WBS area UUID"},
							"percent":     map[string]any{"type": "number", "description": "Percent of this line's amount (0-100)"},
						},
						"required": []string{"wbs_area_id", "percent"},
					},
				},
			},
			"required": []string{"line_type", "amount"},
		})),
		makeTool("report_invoice_summary", "Report a markdown summary of the proposed coding. Call last.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary_markdown": map[string]any{"type": "string", "description": "Markdown: total coded, uncategorized amount, and items needing human review"},
			},
			"required": []string{"summary_markdown"},
		})),
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
