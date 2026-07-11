package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// --- SOW Config handlers ---

func (h *Handler) GetSOWConfig(w http.ResponseWriter, r *http.Request) {
	grantID := chi.URLParam(r, "grantID")
	cfg, err := h.queries.GetSOWConfig(r.Context(), grantID)
	if err != nil {
		// Return empty config if none exists
		respondJSON(w, http.StatusOK, models.SOWConfig{GrantID: grantID, ConcurrenceSigners: "[]"})
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

func (h *Handler) UpsertSOWConfig(w http.ResponseWriter, r *http.Request) {
	var c models.SOWConfig
	if err := decodeJSON(r, &c); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	c.GrantID = chi.URLParam(r, "grantID")

	// Validate concurrence_signers is valid JSON array
	if c.ConcurrenceSigners == "" {
		c.ConcurrenceSigners = "[]"
	}
	if !json.Valid([]byte(c.ConcurrenceSigners)) {
		respondError(w, http.StatusBadRequest, "concurrence_signers must be valid JSON")
		return
	}

	if err := h.queries.UpsertSOWConfig(r.Context(), &c); err != nil {
		log.Error().Err(err).Msg("Failed to upsert SOW config")
		respondError(w, http.StatusInternalServerError, "Failed to save SOW config")
		return
	}
	respondJSON(w, http.StatusOK, c)
}

// --- SOW Personnel Description handlers ---

func (h *Handler) ListSOWPersonnelDescriptions(w http.ResponseWriter, r *http.Request) {
	sowID := chi.URLParam(r, "sowID")
	items, err := h.queries.ListSOWPersonnelDescriptions(r.Context(), sowID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list personnel descriptions")
		respondError(w, http.StatusInternalServerError, "Failed to list personnel descriptions")
		return
	}
	if items == nil {
		items = []models.SOWPersonnelDescription{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) UpsertSOWPersonnelDescription(w http.ResponseWriter, r *http.Request) {
	var d models.SOWPersonnelDescription
	if err := decodeJSON(r, &d); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	d.SOWID = chi.URLParam(r, "sowID")
	if err := h.queries.UpsertSOWPersonnelDescription(r.Context(), &d); err != nil {
		log.Error().Err(err).Msg("Failed to upsert SOW personnel description")
		respondError(w, http.StatusInternalServerError, "Failed to save personnel description")
		return
	}
	respondJSON(w, http.StatusOK, d)
}

func (h *Handler) DeleteSOWPersonnelDescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "descID")
	if err := h.queries.DeleteSOWPersonnelDescription(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete personnel description")
		respondError(w, http.StatusInternalServerError, "Failed to delete personnel description")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- SOW Line Item Description handlers ---

func (h *Handler) ListSOWLineItemDescriptions(w http.ResponseWriter, r *http.Request) {
	sowID := chi.URLParam(r, "sowID")
	items, err := h.queries.ListSOWLineItemDescriptions(r.Context(), sowID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list line item descriptions")
		respondError(w, http.StatusInternalServerError, "Failed to list line item descriptions")
		return
	}
	if items == nil {
		items = []models.SOWLineItemDescription{}
	}
	respondJSON(w, http.StatusOK, items)
}

func (h *Handler) UpsertSOWLineItemDescription(w http.ResponseWriter, r *http.Request) {
	var d models.SOWLineItemDescription
	if err := decodeJSON(r, &d); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}
	d.SOWID = chi.URLParam(r, "sowID")
	if err := h.queries.UpsertSOWLineItemDescription(r.Context(), &d); err != nil {
		log.Error().Err(err).Msg("Failed to upsert SOW line item description")
		respondError(w, http.StatusInternalServerError, "Failed to save line item description")
		return
	}
	respondJSON(w, http.StatusOK, d)
}

func (h *Handler) DeleteSOWLineItemDescription(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "descID")
	if err := h.queries.DeleteSOWLineItemDescription(r.Context(), id); err != nil {
		log.Error().Err(err).Msg("Failed to delete line item description")
		respondError(w, http.StatusInternalServerError, "Failed to delete line item description")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- SOW Document Generation ---

// sowPersonnelRow represents one row in the SOW personnel table.
type sowPersonnelRow struct {
	Name            string
	JobType         string
	FullyLoadedCost float64
	Area            string
	FTEPercent      int
	WBSCode         string
}

// sowBudgetRow represents one row in the SOW budget summary table.
type sowBudgetRow struct {
	WBSCode     string
	Description string
	Budget      float64
	Effort      string // e.g. "42% FTE" for personnel, empty for non-personnel
}

// sowSignerBlock is one signature block at the end of the SOW.
type sowSignerBlock struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Affiliation string `json:"affiliation"`
}

// sowRenderData is the full data passed to the SOW HTML template.
type sowRenderData struct {
	// Header
	HeaderTitle    string
	HeaderSubtitle string
	ProjectName    string
	YearLabel      string

	// Intro
	IntroHTML template.HTML

	// Institution / subaward info
	Institution string
	PIName      string

	// Personnel table
	PersonnelRows []sowPersonnelRow

	// Description of Work
	PersonnelDescriptions []sowPersonnelDescBlock
	BudgetRows            []sowBudgetRow
	TotalBudget           float64

	// Atypical line item descriptions
	LineItemDescriptions []sowLineItemDescBlock

	// Costs and Funding
	CostsHTML   template.HTML
	PeriodStart string
	PeriodEnd   string

	// Concurrence
	Signers []sowSignerBlock

	// Footer
	DateString string
	DocTitle   string
}

type sowPersonnelDescBlock struct {
	Name        string
	JobType     string
	IsPIOrLead  bool
	Description template.HTML
}

type sowLineItemDescBlock struct {
	Description    string
	DescriptionMD  template.HTML
}

// isTypicalLineType returns true for budget line types that don't need a
// free-form SOW description (salary, fringe, travel, supplies).
func isTypicalLineType(lt string) bool {
	switch lt {
	case "personnel", "fringe", "travel", "supplies":
		return true
	default:
		return false
	}
}

// RenderSOW generates an HTML Statement of Work document for a given SOW record.
func (h *Handler) RenderSOW(w http.ResponseWriter, r *http.Request) {
	subawardID := chi.URLParam(r, "subawardID")
	sowID := chi.URLParam(r, "sowID")
	ctx := r.Context()

	// Load SOW record
	sow, err := h.queries.GetStatementOfWork(ctx, sowID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Statement of work not found")
		return
	}

	// Load subaward
	sub, err := h.queries.GetSubaward(ctx, subawardID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Subaward not found")
		return
	}

	// Load grant
	grant, err := h.queries.GetGrant(ctx, sub.GrantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get grant")
		respondError(w, http.StatusInternalServerError, "Failed to get grant")
		return
	}

	// Load SOW config (optional - use defaults if not set)
	sowCfg, err := h.queries.GetSOWConfig(ctx, grant.ID)
	if err != nil {
		sowCfg = &models.SOWConfig{
			GrantID:            grant.ID,
			ConcurrenceSigners: "[]",
		}
	}

	// Load all personnel for the grant
	allPersonnel, err := h.queries.ListPersonnel(ctx, grant.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list personnel")
		respondError(w, http.StatusInternalServerError, "Failed to list personnel")
		return
	}
	personnelMap := make(map[string]*models.Personnel)
	for i := range allPersonnel {
		personnelMap[allPersonnel[i].ID] = &allPersonnel[i]
	}

	// Load WBS areas for the grant
	wbsAreas, err := h.queries.ListWBSAreas(ctx, grant.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list WBS areas")
		respondError(w, http.StatusInternalServerError, "Failed to list WBS areas")
		return
	}
	wbsMap := make(map[string]*models.WBSArea)
	for i := range wbsAreas {
		wbsMap[wbsAreas[i].ID] = &wbsAreas[i]
	}

	// Load overhead rates for the subaward
	rates, err := h.queries.ListOverheadRates(ctx, "subaward", sub.ID)
	if err != nil {
		rates = nil
	}
	rateMap := make(map[string]*models.OverheadRate)
	for i := range rates {
		rateMap[rates[i].ID] = &rates[i]
	}

	// Find matching budget: prefer budget_id FK, fall back to fiscal year match
	budgets, err := h.queries.ListLatestInstitutionBudgets(ctx, "subaward", sub.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list budgets")
		respondError(w, http.StatusInternalServerError, "Failed to list budgets")
		return
	}
	var matchingBudget *models.InstitutionBudget
	if sow.BudgetID != nil {
		for i := range budgets {
			if budgets[i].ID == *sow.BudgetID {
				matchingBudget = &budgets[i]
				break
			}
		}
	}
	if matchingBudget == nil {
		for i := range budgets {
			if budgets[i].FiscalYear == sow.FiscalYear {
				matchingBudget = &budgets[i]
				break
			}
		}
	}

	// Load budget line items
	var lineItems []models.BudgetLineItem
	if matchingBudget != nil {
		lineItems, err = h.queries.ListBudgetLineItems(ctx, matchingBudget.ID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list budget line items for SOW")
		}
	}

	// Load WBS allocations for budget line items
	lineItemWBS := make(map[string][]models.BudgetLineItemWBS)
	for _, li := range lineItems {
		allocs, err := h.queries.ListLineItemWBS(ctx, li.ID)
		if err == nil {
			lineItemWBS[li.ID] = allocs
		}
	}

	// Load personnel descriptions for this SOW
	persDescs, err := h.queries.ListSOWPersonnelDescriptions(ctx, sowID)
	if err != nil {
		persDescs = nil
	}
	persDescMap := make(map[string]string) // personnel_id -> markdown
	for _, pd := range persDescs {
		persDescMap[pd.PersonnelID] = pd.DescriptionMD
	}

	// Load line item descriptions for this SOW
	liDescs, err := h.queries.ListSOWLineItemDescriptions(ctx, sowID)
	if err != nil {
		liDescs = nil
	}
	liDescMap := make(map[string]string) // line_item_id -> markdown
	for _, ld := range liDescs {
		liDescMap[ld.LineItemID] = ld.DescriptionMD
	}

	// Helper: get indirect rate for a line item
	getIndirectRate := func(li models.BudgetLineItem) float64 {
		if li.OverheadRateID == nil {
			return 0
		}
		rate, ok := rateMap[*li.OverheadRateID]
		if !ok {
			return 0
		}
		return rate.Rate
	}

	// Group line items: personnel/fringe by person, everything else separate
	type personItemSet struct {
		SalaryItems []models.BudgetLineItem
		FringeItems []models.BudgetLineItem
	}
	personItems := make(map[string]*personItemSet)
	var nonPersonnelItems []models.BudgetLineItem

	for _, li := range lineItems {
		if li.PersonnelID != nil && (li.LineType == "personnel" || li.LineType == "fringe") {
			pid := *li.PersonnelID
			if _, ok := personItems[pid]; !ok {
				personItems[pid] = &personItemSet{}
			}
			if li.LineType == "personnel" {
				personItems[pid].SalaryItems = append(personItems[pid].SalaryItems, li)
			} else {
				personItems[pid].FringeItems = append(personItems[pid].FringeItems, li)
			}
		} else {
			nonPersonnelItems = append(nonPersonnelItems, li)
		}
	}

	// Accumulate fully-loaded costs per person per WBS area
	type wbsPersonKey struct {
		PersonID string
		WBSID    string
	}
	type personWBSData struct {
		PersonID       string
		Name           string
		JobType        string
		WBSCode        string
		WBSName        string
		SalaryAmount   float64
		FringeAmount   float64
		IndirectAmount float64
		EffortMonths   float64
	}
	personWBSAccum := make(map[wbsPersonKey]*personWBSData)

	for pid, items := range personItems {
		p := personnelMap[pid]
		if p == nil {
			continue
		}
		// Salary items
		for _, si := range items.SalaryItems {
			allocs := lineItemWBS[si.ID]
			indRate := getIndirectRate(si)
			if len(allocs) == 0 {
				key := wbsPersonKey{PersonID: pid}
				if _, ok := personWBSAccum[key]; !ok {
					personWBSAccum[key] = &personWBSData{PersonID: pid, Name: p.Name, JobType: p.Title}
				}
				d := personWBSAccum[key]
				d.SalaryAmount += si.Amount
				d.IndirectAmount += si.Amount * indRate
				d.EffortMonths += si.EffortMonths
			} else {
				for _, alloc := range allocs {
					wbs := wbsMap[alloc.WBSAreaID]
					code, name := "", ""
					if wbs != nil {
						code, name = wbs.Code, wbs.Name
					}
					key := wbsPersonKey{PersonID: pid, WBSID: alloc.WBSAreaID}
					if _, ok := personWBSAccum[key]; !ok {
						personWBSAccum[key] = &personWBSData{PersonID: pid, Name: p.Name, JobType: p.Title, WBSCode: code, WBSName: name}
					}
					d := personWBSAccum[key]
					pctAmt := si.Amount * alloc.AllocationPercent / 100.0
					d.SalaryAmount += pctAmt
					d.IndirectAmount += pctAmt * indRate
					d.EffortMonths += si.EffortMonths * alloc.AllocationPercent / 100.0
				}
			}
		}
		// Fringe items
		for _, fi := range items.FringeItems {
			allocs := lineItemWBS[fi.ID]
			indRate := getIndirectRate(fi)
			if len(allocs) == 0 {
				key := wbsPersonKey{PersonID: pid}
				if _, ok := personWBSAccum[key]; !ok {
					personWBSAccum[key] = &personWBSData{PersonID: pid, Name: p.Name, JobType: p.Title}
				}
				d := personWBSAccum[key]
				d.FringeAmount += fi.Amount
				d.IndirectAmount += fi.Amount * indRate
			} else {
				for _, alloc := range allocs {
					wbs := wbsMap[alloc.WBSAreaID]
					code, name := "", ""
					if wbs != nil {
						code, name = wbs.Code, wbs.Name
					}
					key := wbsPersonKey{PersonID: pid, WBSID: alloc.WBSAreaID}
					if _, ok := personWBSAccum[key]; !ok {
						personWBSAccum[key] = &personWBSData{PersonID: pid, Name: p.Name, JobType: p.Title, WBSCode: code, WBSName: name}
					}
					d := personWBSAccum[key]
					pctAmt := fi.Amount * alloc.AllocationPercent / 100.0
					d.FringeAmount += pctAmt
					d.IndirectAmount += pctAmt * indRate
				}
			}
		}
	}

	// Build Personnel table rows (Section 2) — fully loaded cost, integer %FTE
	var personnelRows []sowPersonnelRow
	for _, d := range personWBSAccum {
		fullyLoaded := d.SalaryAmount + d.FringeAmount + d.IndirectAmount
		ftePct := int(math.Round(d.EffortMonths / 12.0 * 100))
		personnelRows = append(personnelRows, sowPersonnelRow{
			Name:            d.Name,
			JobType:         d.JobType,
			FullyLoadedCost: math.Round(fullyLoaded*100) / 100,
			Area:            d.WBSName,
			FTEPercent:      ftePct,
			WBSCode:         d.WBSCode,
		})
	}
	sort.Slice(personnelRows, func(i, j int) bool {
		if personnelRows[i].WBSCode != personnelRows[j].WBSCode {
			return personnelRows[i].WBSCode < personnelRows[j].WBSCode
		}
		return personnelRows[i].Name < personnelRows[j].Name
	})

	// Build budget table rows (Section 3) — combined salary+fringe+indirect per person, sorted by WBS
	var budgetRows []sowBudgetRow
	for _, d := range personWBSAccum {
		fullyLoaded := d.SalaryAmount + d.FringeAmount + d.IndirectAmount
		ftePct := int(math.Round(d.EffortMonths / 12.0 * 100))
		budgetRows = append(budgetRows, sowBudgetRow{
			WBSCode:     d.WBSCode,
			Description: fmt.Sprintf("%s (%s)", d.Name, d.JobType),
			Budget:      math.Round(fullyLoaded*100) / 100,
			Effort:      fmt.Sprintf("%d%% FTE", ftePct),
		})
	}
	// Non-personnel budget rows (fully loaded with indirect)
	for _, li := range nonPersonnelItems {
		indRate := getIndirectRate(li)
		allocs := lineItemWBS[li.ID]
		desc := li.Description
		if desc == "" {
			desc = formatLineTypeLabel(li.LineType, sub.Institution)
		}
		if len(allocs) == 0 {
			fullyLoaded := li.Amount + li.Amount*indRate
			budgetRows = append(budgetRows, sowBudgetRow{
				Description: desc,
				Budget:      math.Round(fullyLoaded*100) / 100,
			})
		} else {
			for _, alloc := range allocs {
				wbs := wbsMap[alloc.WBSAreaID]
				code := ""
				if wbs != nil {
					code = wbs.Code
				}
				pctAmt := li.Amount * alloc.AllocationPercent / 100.0
				fullyLoaded := pctAmt + pctAmt*indRate
				budgetRows = append(budgetRows, sowBudgetRow{
					WBSCode:     code,
					Description: desc,
					Budget:      math.Round(fullyLoaded*100) / 100,
				})
			}
		}
	}
	sort.Slice(budgetRows, func(i, j int) bool {
		if budgetRows[i].WBSCode != budgetRows[j].WBSCode {
			return budgetRows[i].WBSCode < budgetRows[j].WBSCode
		}
		return budgetRows[i].Description < budgetRows[j].Description
	})

	// Total budget = sum of fully-loaded rows
	var totalBudget float64
	for _, row := range budgetRows {
		totalBudget += row.Budget
	}

	// Build personnel description blocks using accumulated person IDs
	var persDescBlocks []sowPersonnelDescBlock
	seenDescPersonnel := make(map[string]bool)
	for _, row := range personnelRows {
		for pid, p := range personnelMap {
			if p.Name == row.Name && !seenDescPersonnel[pid] {
				seenDescPersonnel[pid] = true
				md := persDescMap[pid]
				if md == "" {
					break
				}
				isPIOrLead := p.Role == "pi" || p.Role == "co_pi" || p.Role == "subaward_pi"
				persDescBlocks = append(persDescBlocks, sowPersonnelDescBlock{
					Name:        p.Name,
					JobType:     p.Title,
					IsPIOrLead:  isPIOrLead,
					Description: markdownToHTML(md),
				})
				break
			}
		}
	}

	// Build atypical line item description blocks
	var liDescBlocks []sowLineItemDescBlock
	for _, li := range lineItems {
		if isTypicalLineType(li.LineType) {
			continue
		}
		md, ok := liDescMap[li.ID]
		if !ok || md == "" {
			continue
		}
		desc := li.Description
		if desc == "" {
			desc = formatLineTypeLabel(li.LineType, sub.Institution)
		}
		liDescBlocks = append(liDescBlocks, sowLineItemDescBlock{
			Description:   desc,
			DescriptionMD: markdownToHTML(md),
		})
	}

	// Parse concurrence signers
	var signers []sowSignerBlock
	if sowCfg.ConcurrenceSigners != "" && sowCfg.ConcurrenceSigners != "[]" {
		_ = json.Unmarshal([]byte(sowCfg.ConcurrenceSigners), &signers)
	}
	for i := range signers {
		signers[i].Name = renderSOWTemplate(signers[i].Name, sow, sub, grant, totalBudget)
		signers[i].Title = renderSOWTemplate(signers[i].Title, sow, sub, grant, totalBudget)
		signers[i].Affiliation = renderSOWTemplate(signers[i].Affiliation, sow, sub, grant, totalBudget)
	}

	// Render all template strings
	introHTML := renderSOWTemplate(sowCfg.IntroTemplate, sow, sub, grant, totalBudget)
	costsHTML := renderSOWTemplate(sowCfg.CostsTemplate, sow, sub, grant, totalBudget)
	headerTitle := renderSOWTemplate(sowCfg.HeaderTitle, sow, sub, grant, totalBudget)
	headerSubtitle := renderSOWTemplate(sowCfg.HeaderSubtitle, sow, sub, grant, totalBudget)
	projectName := renderSOWTemplate(sowCfg.ProjectName, sow, sub, grant, totalBudget)

	yearLabel := fmt.Sprintf("Year %d", sow.FiscalYear)

	data := sowRenderData{
		HeaderTitle:           headerTitle,
		HeaderSubtitle:        headerSubtitle,
		ProjectName:           projectName,
		YearLabel:             yearLabel,
		IntroHTML:             template.HTML(introHTML),
		Institution:           sub.Institution,
		PIName:                sub.PIName,
		PersonnelRows:         personnelRows,
		PersonnelDescriptions: persDescBlocks,
		BudgetRows:            budgetRows,
		TotalBudget:           totalBudget,
		LineItemDescriptions:  liDescBlocks,
		CostsHTML:             template.HTML(costsHTML),
		PeriodStart:           sow.PeriodStart,
		PeriodEnd:             sow.PeriodEnd,
		Signers:               signers,
		DateString:            sow.PeriodStart,
		DocTitle:              fmt.Sprintf("%s %s SOW", grant.Title, yearLabel),
	}

	funcMap := template.FuncMap{
		"dollar":     sowFormatDollar,
		"fmtPctInt":  func(v int) string { return fmt.Sprintf("%d%%", v) },
	}

	tmpl, err := template.New("sow").Funcs(funcMap).Parse(sowHTMLTemplate)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse SOW template")
		respondError(w, http.StatusInternalServerError, "Template error")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Error().Err(err).Msg("Failed to render SOW")
	}
}

// formatLineTypeLabel produces a human-readable label for a budget line type.
func formatLineTypeLabel(lineType, institution string) string {
	switch lineType {
	case "personnel":
		return "Labor " + institution
	case "fringe":
		return "Fringe " + institution
	case "travel":
		return "Travel " + institution
	case "equipment":
		return "Equipment " + institution
	case "supplies":
		return "Materials & Supplies " + institution
	case "contractual":
		return "Contractual " + institution
	case "participant_support":
		return "Participant Support " + institution
	case "tuition":
		return "Tuition " + institution
	case "other":
		return "Other " + institution
	default:
		return lineType + " " + institution
	}
}

// markdownToHTML does a simple conversion of markdown bullet lists to HTML.
// For a full implementation, use a proper markdown library; this handles the
// common case of bullet-point descriptions in a SOW.
func markdownToHTML(md string) template.HTML {
	lines := strings.Split(md, "\n")
	var out strings.Builder
	inList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inList {
				out.WriteString("</ul>\n")
				inList = false
			}
			continue
		}
		// Check for bullet: - or * at start
		isBullet := false
		bulletContent := ""
		if strings.HasPrefix(trimmed, "- ") {
			isBullet = true
			bulletContent = strings.TrimSpace(trimmed[2:])
		} else if strings.HasPrefix(trimmed, "* ") {
			isBullet = true
			bulletContent = strings.TrimSpace(trimmed[2:])
		} else if strings.HasPrefix(trimmed, "  - ") || strings.HasPrefix(trimmed, "  * ") ||
			strings.HasPrefix(trimmed, "\t- ") || strings.HasPrefix(trimmed, "\t* ") {
			// Nested bullet - treat as sub-item
			isBullet = true
			bulletContent = strings.TrimSpace(trimmed[2:])
			if strings.HasPrefix(bulletContent, "- ") || strings.HasPrefix(bulletContent, "* ") {
				bulletContent = strings.TrimSpace(bulletContent[2:])
			}
		}

		if isBullet {
			if !inList {
				out.WriteString("<ul>\n")
				inList = true
			}
			out.WriteString("<li>")
			out.WriteString(template.HTMLEscapeString(bulletContent))
			out.WriteString("</li>\n")
		} else {
			if inList {
				out.WriteString("</ul>\n")
				inList = false
			}
			out.WriteString("<p>")
			out.WriteString(template.HTMLEscapeString(trimmed))
			out.WriteString("</p>\n")
		}
	}
	if inList {
		out.WriteString("</ul>\n")
	}
	return template.HTML(out.String())
}

// renderSOWTemplate executes a Go text template string with SOW context variables.
func renderSOWTemplate(tmplText string, sow *models.StatementOfWork, sub *models.Subaward, grant *models.Grant, totalBudget float64) string {
	if tmplText == "" {
		return ""
	}
	data := map[string]interface{}{
		"Institution":  sub.Institution,
		"PIName":       sub.PIName,
		"GrantTitle":   grant.Title,
		"AwardNumber":  grant.AwardNumber,
		"FiscalYear":   sow.FiscalYear,
		"PeriodStart":  sow.PeriodStart,
		"PeriodEnd":    sow.PeriodEnd,
		"TotalBudget":  sowFormatDollar(totalBudget),
	}
	funcMap := template.FuncMap{
		"dollar": sowFormatDollar,
	}

	t, err := template.New("sow_section").Funcs(funcMap).Parse(tmplText)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse SOW section template")
		return template.HTMLEscapeString(tmplText)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		log.Error().Err(err).Msg("Failed to execute SOW section template")
		return template.HTMLEscapeString(tmplText)
	}
	return buf.String()
}

func sowFormatDollar(v float64) string {
	negative := v < 0
	v = math.Abs(v)
	whole := int64(math.Round(v))
	s := fmt.Sprintf("%d", whole)
	if len(s) > 3 {
		parts := make([]string, 0)
		for len(s) > 3 {
			parts = append([]string{s[len(s)-3:]}, parts...)
			s = s[:len(s)-3]
		}
		parts = append([]string{s}, parts...)
		s = strings.Join(parts, ",")
	}
	if negative {
		return "$(" + s + ")"
	}
	if whole == 0 {
		return "$0"
	}
	return "$" + s
}

// The HTML template for rendering the SOW document,
// modeled after the IRIS-HEP Y6 SOW format.
const sowHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.DocTitle}}</title>
<style>
  @page { size: letter; margin: 1in; }
  body {
    font-family: "Times New Roman", Times, serif;
    font-size: 12pt;
    line-height: 1.4;
    max-width: 7.5in;
    margin: 0 auto;
    padding: 1in;
    color: #000;
  }
  h1 { text-align: center; font-size: 14pt; margin-bottom: 0.2em; }
  h2 { font-size: 13pt; margin-top: 1.5em; }
  .header-block { text-align: center; margin-bottom: 2em; }
  .header-block .subtitle { font-style: italic; margin: 0.2em 0; }
  .header-block .project { font-weight: bold; }
  .header-block .year { margin-top: 0.5em; }

  table.personnel {
    width: 100%;
    border-collapse: collapse;
    margin: 1em 0;
    font-size: 11pt;
  }
  table.personnel th {
    border-bottom: 2px solid #000;
    text-align: left;
    padding: 4px 8px;
    font-weight: bold;
  }
  table.personnel td {
    padding: 3px 8px;
    vertical-align: top;
  }
  table.personnel td.amount { text-align: right; }
  table.personnel td.fte { text-align: right; }
  table.personnel tr.total td { border-top: 2px solid #000; font-weight: bold; }

  table.budget {
    width: 80%;
    margin: 1em auto;
    border-collapse: collapse;
    font-size: 11pt;
  }
  table.budget th {
    border-bottom: 2px solid #000;
    text-align: left;
    padding: 4px 8px;
  }
  table.budget td {
    padding: 3px 8px;
  }
  table.budget td.amount { text-align: right; }
  table.budget tr.total td { border-top: 2px solid #000; font-weight: bold; }

  .description-block { margin: 1em 0 1em 1em; }
  .description-block h3 { font-size: 12pt; margin-bottom: 0.3em; }
  .description-block ul { margin: 0.3em 0; padding-left: 1.5em; }
  .description-block li { margin: 0.2em 0; }

  .signer-block {
    margin-top: 3em;
    page-break-inside: avoid;
  }
  .signer-block .line {
    margin-top: 2em;
    border-top: 1px solid #000;
    width: 50%;
    padding-top: 0.2em;
  }

  .footer {
    text-align: center;
    font-size: 10pt;
    color: #555;
    margin-top: 2em;
  }

  @media print {
    body { padding: 0; }
    .no-print { display: none; }
  }
</style>
</head>
<body>

<div class="no-print" style="text-align:center; margin-bottom:16px;">
<button onclick="window.print()" style="padding:8px 24px; font-size:14px; cursor:pointer;">Print / Save as PDF</button>
</div>

<div class="header-block">
  {{if .HeaderTitle}}<h1>{{.HeaderTitle}}</h1>{{end}}
  {{if .HeaderSubtitle}}<div class="subtitle">{{.HeaderSubtitle}}</div>{{end}}
  {{if .ProjectName}}<div class="project">{{.ProjectName}} - {{.YearLabel}}</div>{{end}}
</div>

{{if .IntroHTML}}
<h2>1&nbsp;&nbsp;&nbsp;Introduction</h2>
{{.IntroHTML}}
{{end}}

<h2>2&nbsp;&nbsp;&nbsp;Personnel</h2>
<p>The table below lists Personnel whose salary support for activities covered by this SOW comes fully
or partially from the project. "Job Type" is Faculty, Research Faculty, Professional,
Staff, Postdoc, Grad Student, etc. "Cost to Project" includes salary, fringe benefits, and
applicable indirect costs.</p>

<table class="personnel">
<thead>
<tr>
  <th>Name</th>
  <th>Job Type</th>
  <th>Cost to Project<br>for this SOW</th>
  <th>Area</th>
  <th>%FTE for<br>this SOW</th>
  <th>WBS#</th>
</tr>
</thead>
<tbody>
{{range .PersonnelRows}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.JobType}}</td>
  <td class="amount">{{dollar .FullyLoadedCost}}</td>
  <td>{{.Area}}</td>
  <td class="fte">{{fmtPctInt .FTEPercent}}</td>
  <td>{{.WBSCode}}</td>
</tr>
{{end}}
</tbody>
</table>

<p>Changes to the names of funded personnel, including initial names for effort labeled TBN in this
SOW, or to the effort levels foreseen above should be reported to the Principal Investigator
and as part of monthly and/or quarterly reports.</p>

<h2>3&nbsp;&nbsp;&nbsp;Description of Work</h2>
<p>During this period of performance, the group at {{.Institution}} agrees to supply the
following deliverables and/or effort at a cost not to exceed the cost listed below. The
following itemized list describes the items (or partial completion of items) provided in this period
(Statement of Work).</p>

{{range .PersonnelDescriptions}}
<div class="description-block">
  <h3>&bull; {{.Name}}{{if .JobType}}, {{.JobType}}{{end}}:</h3>
  {{.Description}}
</div>
{{end}}

{{range .LineItemDescriptions}}
<div class="description-block">
  <h3>&bull; {{.Description}}:</h3>
  {{.DescriptionMD}}
</div>
{{end}}

<p>The following table summarizes the budget by WBS area. All costs are fully loaded, including
salary, fringe benefits, and applicable indirect costs.</p>

<table class="budget">
<thead>
<tr>
  <th>WBS#</th>
  <th>Description</th>
  <th>Effort</th>
  <th>Budget</th>
</tr>
</thead>
<tbody>
{{range .BudgetRows}}
<tr>
  <td>{{.WBSCode}}</td>
  <td>{{.Description}}</td>
  <td>{{.Effort}}</td>
  <td class="amount">{{dollar .Budget}}</td>
</tr>
{{end}}
<tr class="total">
  <td></td>
  <td>Total Budget</td>
  <td></td>
  <td class="amount">{{dollar .TotalBudget}}</td>
</tr>
</tbody>
</table>

<h2>4&nbsp;&nbsp;&nbsp;Costs and Funding</h2>
{{if .CostsHTML}}
{{.CostsHTML}}
{{else}}
<p>The group at {{.Institution}} will report all project-related expenditures together with
associated technical progress for each item of work by Work Breakdown Structure (WBS) at the
lowest WBS Level. The group at {{.Institution}} agrees to furnish on request all software,
related documentation and any quality control and performance checks which are carried out
for the project in the performance of this work. It is the responsibility of the subaward
recipient to ensure that cost reports or invoices with WBS breakdowns are submitted on a monthly
basis and that all invoices are submitted within 60 days after the end of the period of performance.</p>
<p>A total amount of {{dollar .TotalBudget}} is detailed above for the full year from {{.PeriodStart}} through
{{.PeriodEnd}}. Upon execution of this SOW, funds obligated to the subawardee institution will
be increased by this amount.</p>
{{end}}

{{if .Signers}}
<h2>5&nbsp;&nbsp;&nbsp;Makers and Concurrence</h2>
<p>The following persons concur in the terms of this Statement of Work.</p>

{{range .Signers}}
<div class="signer-block">
  <div class="line">
    {{.Name}}&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;(date)<br>
    {{.Title}}<br>
    {{.Affiliation}}
  </div>
</div>
{{end}}
{{end}}

</body>
</html>
`
