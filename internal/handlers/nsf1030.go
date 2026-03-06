package handlers

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// --- NSF 1030 data structures ---

type nsf1030Person struct {
	Number int
	Name   string
	CAL    float64
	ACAD   float64
	SUMR   float64
	Amount float64
}

type nsf1030IndirectLine struct {
	Label string
	Rate  string
	Base  float64
}

type nsf1030Page struct {
	Organization   string
	PIName         string
	AwardNumber    string
	DurationMonths int
	YearLabel      string

	SeniorPersonnel   []nsf1030Person
	TotalSeniorCount  int
	TotalSeniorCAL    float64
	TotalSeniorACAD   float64
	TotalSeniorSUMR   float64
	TotalSeniorAmount float64

	PostDocCount         int
	PostDocCAL           float64
	PostDocAmount        float64
	OtherProfCount       int
	OtherProfCAL         float64
	OtherProfAmount      float64
	GradStudentCount     int
	GradStudentAmount    float64
	UndergradCount       int
	UndergradAmount      float64
	ClericalCount        int
	ClericalAmount       float64
	OtherPersonnelCount  int
	OtherPersonnelAmount float64

	TotalSalariesWages float64
	FringeBenefits     float64
	TotalSalWageFringe float64

	EquipmentItems []string
	TotalEquipment float64

	DomesticTravel float64
	ForeignTravel  float64

	TotalParticipant float64

	MaterialsSupplies float64
	Publication       float64
	Consultant        float64
	ComputerServices  float64
	SubawardAmount    float64
	SubawardCount     int
	OtherDirect       float64
	TotalOtherDirect  float64

	TotalDirectCosts float64

	IndirectLines      []nsf1030IndirectLine
	TotalIndirectCosts float64

	TotalDirectIndirect float64
	AmountOfRequest     float64
}

// classifyRole maps a personnel role string to an NSF 1030 category.
func classifyRole(role string) string {
	// Direct match for standardised role codes
	switch role {
	case "pi", "co_pi", "subaward_pi", "senior_personnel":
		return "senior"
	case "postdoc":
		return "postdoc"
	case "other_professional":
		return "other_prof"
	case "graduate_student":
		return "grad"
	case "undergraduate_student":
		return "undergrad"
	case "clerical":
		return "clerical"
	case "other":
		return "other_pers"
	}
	// Fallback: fuzzy match for legacy/free-form data
	r := strings.ToLower(strings.TrimSpace(role))
	switch {
	case strings.Contains(r, "pi") || strings.Contains(r, "faculty") || strings.Contains(r, "senior") || strings.Contains(r, "professor"):
		return "senior"
	case strings.Contains(r, "postdoc") || strings.Contains(r, "post-doc") || strings.Contains(r, "post doc"):
		return "postdoc"
	case strings.Contains(r, "graduate") || strings.Contains(r, "grad student") || strings.Contains(r, "phd"):
		return "grad"
	case strings.Contains(r, "undergraduate") || strings.Contains(r, "undergrad"):
		return "undergrad"
	case strings.Contains(r, "secretary") || strings.Contains(r, "clerical") || strings.Contains(r, "admin"):
		return "clerical"
	default:
		return "other_prof"
	}
}

func formatDollar(v float64) string {
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
		return "$ (" + s + ")"
	}
	if whole == 0 {
		return "$ -"
	}
	return "$ " + s
}

// RenderNSF1030 generates an NSF Form 1030 HTML page for a given institution.
func (h *Handler) RenderNSF1030(w http.ResponseWriter, r *http.Request) {
	entityType := chi.URLParam(r, "entityType")
	entityID := chi.URLParam(r, "entityID")
	yearParam := r.URL.Query().Get("year")
	ctx := r.Context()

	var grantID, organization, piName, awardNumber string
	var grantStartDate, grantEndDate string

	if entityType == "grant" {
		grant, err := h.queries.GetGrant(ctx, entityID)
		if err != nil {
			respondError(w, http.StatusNotFound, "Grant not found")
			return
		}
		grantID = grant.ID
		organization = grant.Institution
		piName = grant.PIName
		awardNumber = grant.AwardNumber
		grantStartDate = grant.StartDate
		grantEndDate = grant.EndDate
	} else if entityType == "subaward" {
		sub, err := h.queries.GetSubaward(ctx, entityID)
		if err != nil {
			respondError(w, http.StatusNotFound, "Subaward not found")
			return
		}
		grant, err := h.queries.GetGrant(ctx, sub.GrantID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get grant")
			return
		}
		grantID = grant.ID
		organization = sub.Institution
		piName = sub.PIName
		awardNumber = grant.AwardNumber
		grantStartDate = grant.StartDate
		grantEndDate = grant.EndDate
	} else {
		respondError(w, http.StatusBadRequest, "Invalid entity type")
		return
	}

	durationMonths := 60
	if len(grantStartDate) >= 10 && len(grantEndDate) >= 10 {
		sy, _ := strconv.Atoi(grantStartDate[:4])
		sm, _ := strconv.Atoi(grantStartDate[5:7])
		ey, _ := strconv.Atoi(grantEndDate[:4])
		em, _ := strconv.Atoi(grantEndDate[5:7])
		durationMonths = (ey-sy)*12 + (em - sm)
	}

	allPersonnel, err := h.queries.ListPersonnel(ctx, grantID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list personnel")
		respondError(w, http.StatusInternalServerError, "Failed to list personnel")
		return
	}
	personnelMap := make(map[string]*models.Personnel)
	for i := range allPersonnel {
		personnelMap[allPersonnel[i].ID] = &allPersonnel[i]
	}

	overheadRates, err := h.queries.ListOverheadRates(ctx, entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list overhead rates")
		respondError(w, http.StatusInternalServerError, "Failed to list overhead rates")
		return
	}
	rateMap := make(map[string]*models.OverheadRate)
	for i := range overheadRates {
		rateMap[overheadRates[i].ID] = &overheadRates[i]
	}

	budgets, err := h.queries.ListLatestInstitutionBudgets(ctx, entityType, entityID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list budgets")
		respondError(w, http.StatusInternalServerError, "Failed to list budgets")
		return
	}

	var years []int
	if yearParam != "" {
		y, err := strconv.Atoi(yearParam)
		if err != nil || y < 1 || y > 10 {
			respondError(w, http.StatusBadRequest, "Invalid year")
			return
		}
		years = []int{y}
	} else {
		yearSet := make(map[int]bool)
		for _, b := range budgets {
			yearSet[b.FiscalYear] = true
		}
		for y := range yearSet {
			years = append(years, y)
		}
		for i := 0; i < len(years); i++ {
			for j := i + 1; j < len(years); j++ {
				if years[i] > years[j] {
					years[i], years[j] = years[j], years[i]
				}
			}
		}
	}

	// For the lead grant, compute subaward totals per year (G.5)
	subawardTotals := make(map[int]float64)
	subawardCountPerYear := make(map[int]int)
	if entityType == "grant" {
		subs, err := h.queries.ListSubawards(ctx, grantID)
		if err == nil {
			for _, sub := range subs {
				subBudgets, err := h.queries.ListLatestInstitutionBudgets(ctx, "subaward", sub.ID)
				if err != nil {
					continue
				}
				for _, sb := range subBudgets {
					subawardTotals[sb.FiscalYear] += sb.Budget
					subawardCountPerYear[sb.FiscalYear]++
				}
			}
		}
	}

	var pages []nsf1030Page
	budgetsByYear := make(map[int]*models.InstitutionBudget)
	for i := range budgets {
		budgetsByYear[budgets[i].FiscalYear] = &budgets[i]
	}

	for _, year := range years {
		page := h.buildNSF1030Page(ctx, year, fmt.Sprintf("Year %d", year),
			organization, piName, awardNumber, durationMonths,
			budgetsByYear[year], personnelMap, rateMap,
			subawardTotals[year], subawardCountPerYear[year])
		pages = append(pages, page)
	}

	if len(years) > 1 || yearParam == "" {
		cumPage := buildCumulativePage(pages, organization, piName, awardNumber, durationMonths)
		pages = append(pages, cumPage)
	}

	funcMap := template.FuncMap{
		"dollar": formatDollar,
		"fmtPM": func(v float64) string {
			if v == 0 {
				return ""
			}
			return fmt.Sprintf("%.2f", v)
		},
		"emptySlots": func(have int, total int) []int {
			n := total - have
			if n <= 0 {
				return nil
			}
			s := make([]int, n)
			for i := range s {
				s[i] = have + i + 1
			}
			return s
		},
	}

	tmpl, err := template.New("nsf1030").Funcs(funcMap).Parse(nsf1030Template)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse NSF 1030 template")
		respondError(w, http.StatusInternalServerError, "Template error")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, pages); err != nil {
		log.Error().Err(err).Msg("Failed to render NSF 1030")
	}
}

func (h *Handler) buildNSF1030Page(
	ctx context.Context,
	year int, yearLabel string,
	organization, piName, awardNumber string,
	durationMonths int,
	budget *models.InstitutionBudget,
	personnelMap map[string]*models.Personnel,
	rateMap map[string]*models.OverheadRate,
	subawardTotal float64, subawardCount int,
) nsf1030Page {
	page := nsf1030Page{
		Organization:   organization,
		PIName:         piName,
		AwardNumber:    awardNumber,
		DurationMonths: durationMonths,
		YearLabel:      yearLabel,
		SubawardAmount: subawardTotal,
		SubawardCount:  subawardCount,
	}

	if budget == nil {
		return page
	}

	items, err := h.queries.ListBudgetLineItems(ctx, budget.ID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list budget line items for NSF 1030")
		return page
	}

	// Track unique personnel per category + overhead bases
	seniorSeen := make(map[string]bool)
	postdocSeen := make(map[string]bool)
	otherProfSeen := make(map[string]bool)
	gradSeen := make(map[string]bool)
	undergradSeen := make(map[string]bool)
	clericalSeen := make(map[string]bool)
	otherPersSeen := make(map[string]bool)
	overheadBases := make(map[string]float64) // rateID -> base amount

	for _, item := range items {
		switch item.LineType {
		case "personnel":
			// Classify by role
			var role, name string
			var pid string
			if item.PersonnelID != nil {
				pid = *item.PersonnelID
				if p, ok := personnelMap[pid]; ok {
					role = p.Role
					name = p.Name
				}
			}
			cat := classifyRole(role)
			switch cat {
			case "senior":
				if !seniorSeen[pid] {
					seniorSeen[pid] = true
					page.SeniorPersonnel = append(page.SeniorPersonnel, nsf1030Person{
						Number: len(page.SeniorPersonnel) + 1,
						Name:   name,
						CAL:    item.EffortMonths,
						Amount: item.Amount,
					})
				} else {
					// Add to existing
					for i := range page.SeniorPersonnel {
						if page.SeniorPersonnel[i].Name == name {
							page.SeniorPersonnel[i].CAL += item.EffortMonths
							page.SeniorPersonnel[i].Amount += item.Amount
							break
						}
					}
				}
				page.TotalSeniorCAL += item.EffortMonths
				page.TotalSeniorAmount += item.Amount
			case "postdoc":
				if !postdocSeen[pid] {
					postdocSeen[pid] = true
					page.PostDocCount++
				}
				page.PostDocCAL += item.EffortMonths
				page.PostDocAmount += item.Amount
			case "grad":
				if !gradSeen[pid] {
					gradSeen[pid] = true
					page.GradStudentCount++
				}
				page.GradStudentAmount += item.Amount
			case "undergrad":
				if !undergradSeen[pid] {
					undergradSeen[pid] = true
					page.UndergradCount++
				}
				page.UndergradAmount += item.Amount
			case "clerical":
				if !clericalSeen[pid] {
					clericalSeen[pid] = true
					page.ClericalCount++
				}
				page.ClericalAmount += item.Amount
			default: // other_prof
				if !otherProfSeen[pid] {
					otherProfSeen[pid] = true
					page.OtherProfCount++
				}
				page.OtherProfCAL += item.EffortMonths
				page.OtherProfAmount += item.Amount
			}
			page.TotalSalariesWages += item.Amount

			// Accumulate overhead base for this item's rate
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		case "fringe":
			page.FringeBenefits += item.Amount
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		case "equipment":
			desc := item.Description
			if desc == "" && item.PersonnelID != nil {
				if p, ok := personnelMap[*item.PersonnelID]; ok {
					desc = p.Name
				}
			}
			if desc == "" {
				desc = fmt.Sprintf("Equipment ($%.0f)", item.Amount)
			}
			page.EquipmentItems = append(page.EquipmentItems, desc)
			page.TotalEquipment += item.Amount

		case "travel":
			desc := strings.ToLower(item.Description)
			if strings.Contains(desc, "foreign") || strings.Contains(desc, "international") {
				page.ForeignTravel += item.Amount
			} else {
				page.DomesticTravel += item.Amount
			}
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		case "participant_support":
			page.TotalParticipant += item.Amount
		// Participant support is excluded from F&A base (do not add to overhead)

		case "supplies":
			page.MaterialsSupplies += item.Amount
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		case "contractual":
			page.Consultant += item.Amount
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		case "tuition":
			page.OtherDirect += item.Amount
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}

		default: // "other"
			// Check description to see if it matches publication or computer
			descL := strings.ToLower(item.Description)
			if strings.Contains(descL, "publication") || strings.Contains(descL, "dissemination") {
				page.Publication += item.Amount
			} else if strings.Contains(descL, "computer") || strings.Contains(descL, "computing") {
				page.ComputerServices += item.Amount
			} else {
				page.OtherDirect += item.Amount
			}
			if item.OverheadRateID != nil {
				overheadBases[*item.OverheadRateID] += item.Amount
			}
		}

		// Track personnel for "other" category
		if item.LineType == "personnel" {
			pid := ""
			if item.PersonnelID != nil {
				pid = *item.PersonnelID
			}
			cat := "other_prof"
			if p, ok := personnelMap[pid]; ok {
				cat = classifyRole(p.Role)
			}
			if cat != "senior" && cat != "postdoc" && cat != "grad" && cat != "undergrad" && cat != "clerical" && cat != "other_prof" {
				if !otherPersSeen[pid] {
					otherPersSeen[pid] = true
					page.OtherPersonnelCount++
				}
				page.OtherPersonnelAmount += item.Amount
			}
		}
	}

	page.TotalSeniorCount = len(page.SeniorPersonnel)
	page.TotalSalWageFringe = page.TotalSalariesWages + page.FringeBenefits

	page.TotalOtherDirect = page.MaterialsSupplies + page.Publication + page.Consultant +
		page.ComputerServices + page.SubawardAmount + page.OtherDirect

	page.TotalDirectCosts = page.TotalSalWageFringe + page.TotalEquipment +
		page.DomesticTravel + page.ForeignTravel + page.TotalParticipant + page.TotalOtherDirect

	// Compute indirect costs from the overhead bases
	for rateID, base := range overheadBases {
		rate, ok := rateMap[rateID]
		if !ok {
			continue
		}
		pct := rate.Rate * 100
		var rateStr string
		if pct == math.Trunc(pct) {
			rateStr = fmt.Sprintf("%.0f%% of MTDC", pct)
		} else {
			rateStr = fmt.Sprintf("%.1f%% of MTDC", pct)
		}
		indirect := base * rate.Rate
		page.IndirectLines = append(page.IndirectLines, nsf1030IndirectLine{
			Label: rate.RateName,
			Rate:  rateStr,
			Base:  base,
		})
		page.TotalIndirectCosts += indirect
	}

	page.TotalIndirectCosts = math.Round(page.TotalIndirectCosts*100) / 100
	page.TotalDirectIndirect = page.TotalDirectCosts + page.TotalIndirectCosts
	page.AmountOfRequest = page.TotalDirectIndirect

	return page
}

func buildCumulativePage(yearPages []nsf1030Page, organization, piName, awardNumber string, durationMonths int) nsf1030Page {
	cum := nsf1030Page{
		Organization:   organization,
		PIName:         piName,
		AwardNumber:    awardNumber,
		DurationMonths: durationMonths,
		YearLabel:      "Cumulative",
	}
	for _, p := range yearPages {
		cum.TotalSeniorAmount += p.TotalSeniorAmount
		cum.TotalSeniorCAL += p.TotalSeniorCAL
		cum.TotalSeniorACAD += p.TotalSeniorACAD
		cum.TotalSeniorSUMR += p.TotalSeniorSUMR
		cum.PostDocAmount += p.PostDocAmount
		cum.PostDocCAL += p.PostDocCAL
		cum.OtherProfAmount += p.OtherProfAmount
		cum.OtherProfCAL += p.OtherProfCAL
		cum.GradStudentAmount += p.GradStudentAmount
		cum.UndergradAmount += p.UndergradAmount
		cum.ClericalAmount += p.ClericalAmount
		cum.OtherPersonnelAmount += p.OtherPersonnelAmount
		cum.TotalSalariesWages += p.TotalSalariesWages
		cum.FringeBenefits += p.FringeBenefits
		cum.TotalSalWageFringe += p.TotalSalWageFringe
		cum.TotalEquipment += p.TotalEquipment
		cum.DomesticTravel += p.DomesticTravel
		cum.ForeignTravel += p.ForeignTravel
		cum.TotalParticipant += p.TotalParticipant
		cum.MaterialsSupplies += p.MaterialsSupplies
		cum.Publication += p.Publication
		cum.Consultant += p.Consultant
		cum.ComputerServices += p.ComputerServices
		cum.SubawardAmount += p.SubawardAmount
		cum.OtherDirect += p.OtherDirect
		cum.TotalOtherDirect += p.TotalOtherDirect
		cum.TotalDirectCosts += p.TotalDirectCosts
		cum.TotalIndirectCosts += p.TotalIndirectCosts
		cum.TotalDirectIndirect += p.TotalDirectIndirect
		cum.AmountOfRequest += p.AmountOfRequest
	}

	// Merge senior personnel across years
	seniorByName := make(map[string]*nsf1030Person)
	var seniorOrder []string
	for _, p := range yearPages {
		for _, sp := range p.SeniorPersonnel {
			if existing, ok := seniorByName[sp.Name]; ok {
				existing.CAL += sp.CAL
				existing.ACAD += sp.ACAD
				existing.SUMR += sp.SUMR
				existing.Amount += sp.Amount
			} else {
				cp := sp
				seniorByName[sp.Name] = &cp
				seniorOrder = append(seniorOrder, sp.Name)
			}
		}
	}
	for i, name := range seniorOrder {
		sp := seniorByName[name]
		sp.Number = i + 1
		cum.SeniorPersonnel = append(cum.SeniorPersonnel, *sp)
	}
	cum.TotalSeniorCount = len(cum.SeniorPersonnel)

	cum.PostDocCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.PostDocCount })
	cum.OtherProfCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.OtherProfCount })
	cum.GradStudentCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.GradStudentCount })
	cum.UndergradCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.UndergradCount })
	cum.ClericalCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.ClericalCount })
	cum.OtherPersonnelCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.OtherPersonnelCount })
	cum.SubawardCount = maxIntField(yearPages, func(p nsf1030Page) int { return p.SubawardCount })

	// Merge indirect cost lines
	indirectByLabel := make(map[string]*nsf1030IndirectLine)
	var indirectOrder []string
	for _, p := range yearPages {
		for _, il := range p.IndirectLines {
			if existing, ok := indirectByLabel[il.Label]; ok {
				existing.Base += il.Base
			} else {
				cp := il
				indirectByLabel[il.Label] = &cp
				indirectOrder = append(indirectOrder, il.Label)
			}
		}
	}
	for _, label := range indirectOrder {
		cum.IndirectLines = append(cum.IndirectLines, *indirectByLabel[label])
	}

	return cum
}

func maxIntField(pages []nsf1030Page, fn func(nsf1030Page) int) int {
	m := 0
	for _, p := range pages {
		if v := fn(p); v > m {
			m = v
		}
	}
	return m
}

const nsf1030Template = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<title>NSF Form 1030 - Budget</title>
<style>
@media print {
  .page-break { page-break-before: always; }
  body { margin: 0; padding: 10px; }
  .no-print { display: none; }
}
body { font-family: Arial, Helvetica, sans-serif; font-size: 8pt; margin: 0; padding: 20px; background: #f5f5f5; }
.page-wrapper { background: white; max-width: 800px; margin: 0 auto 20px auto; padding: 20px 30px; box-shadow: 0 1px 4px rgba(0,0,0,0.15); }
table.nsf { border-collapse: collapse; width: 100%; }
table.nsf td { padding: 1px 3px; vertical-align: bottom; font-size: 8pt; }
.bb1 { border-bottom: 1px solid #000; }
.bb2 { border-bottom: 2px solid #000; }
.br1 { border-right: 1px solid #000; }
.br2 { border-right: 2px solid #000; }
.bl2 { border-left: 2px solid #000; }
.bold { font-weight: bold; }
.center { text-align: center; }
.right { text-align: right; }
.sz10 { font-size: 10pt; }
.sz11 { font-size: 11pt; }
.sz7 { font-size: 7pt; }
.gray { background-color: #d9d9d9; }
td.amt { text-align: right; border-bottom: 1px solid #000; border-right: 1px solid #000; }
td.amt2 { text-align: right; border-bottom: 1px solid #000; border-right: 2px solid #000; }
td.pm { text-align: center; border-bottom: 1px solid #000; border-right: 1px solid #000; }
td.total-amt { text-align: right; border-bottom: 2px solid #000; border-right: 1px solid #000; font-weight: bold; }
td.total-amt2 { text-align: right; border-bottom: 2px solid #000; border-right: 2px solid #000; }
</style>
</head><body>
<div class="no-print" style="text-align:center; margin-bottom:16px;">
<button onclick="window.print()" style="padding:8px 24px; font-size:14px; cursor:pointer;">Print / Save as PDF</button>
</div>
{{range $idx, $page := .}}{{if gt $idx 0}}<div class="page-break"></div>{{end}}
<div class="page-wrapper">
<table class="nsf">
<!-- Header -->
<tr><td colspan="7"></td><td colspan="3"></td><td class="center bold sz10 bb2" colspan="2">{{$page.YearLabel}}</td></tr>
<tr><td colspan="2" class="bb2"></td><td colspan="4" class="bold sz11 center bb2">SUMMARY PROPOSAL BUDGET</td><td class="bb2 br2"></td><td colspan="4" class="bold sz10 center bb2">FOR NSF USE ONLY</td><td class="bb2"></td></tr>
<tr><td colspan="2" class="bold">ORGANIZATION</td><td colspan="5" class="br2"></td><td colspan="2">PROPOSAL NO.</td><td class="br1"></td><td class="bb1 center" colspan="2">DURATION (MONTHS)</td></tr>
<tr><td colspan="7" class="bb1 br2 sz10">{{$page.Organization}}</td><td colspan="3" class="bb1 br1"></td><td class="bb1 center">PROPOSED</td><td class="bb1 br2 center">GRANTED</td></tr>
<tr><td colspan="7" class="br2">PRINCIPAL INVESTIGATOR / PROJECT DIRECTOR</td><td colspan="2">AWARD NO.</td><td class="br1"></td><td class="br1 center">{{$page.DurationMonths}}</td><td class="br2"></td></tr>
<tr><td colspan="7" class="bb2 br2 sz10">{{$page.PIName}}</td><td colspan="5" class="bb2"></td></tr>

<!-- Section A header -->
<tr>
<td colspan="7">A. SENIOR PERSONNEL: PI/PD, Co-PI's, Faculty and Other Senior Associates</td>
<td colspan="2" class="center">NSF-Funded</td><td></td>
<td class="center sz7">Funds</td><td class="center sz7 br2">Funds</td>
</tr>
<tr>
<td colspan="7">&nbsp;&nbsp;&nbsp;&nbsp;(List each separately with title, A.7. show number in brackets)</td>
<td colspan="2" class="bb1 center">Person-months</td><td class="bb1"></td>
<td class="center sz7">Requested By</td><td class="center sz7 br2">Granted by NSF</td>
</tr>
<tr>
<td colspan="7" class="bb1"></td>
<td class="pm">CAL</td><td class="pm">ACAD</td><td class="pm">SUMR</td>
<td class="amt">Proposer</td><td class="amt2">(If Different)</td>
</tr>

<!-- Senior personnel rows (up to 5) -->
{{range $page.SeniorPersonnel}}<tr>
<td colspan="7" class="bb1">&nbsp;&nbsp;{{.Number}}.&nbsp;&nbsp;{{.Name}}</td>
<td class="pm">{{fmtPM .CAL}}</td><td class="pm">{{fmtPM .ACAD}}</td><td class="pm">{{fmtPM .SUMR}}</td>
<td class="amt">{{dollar .Amount}}</td><td class="amt2"></td>
</tr>{{end}}
{{range emptySlots (len $page.SeniorPersonnel) 5}}<tr>
<td colspan="7" class="bb1">&nbsp;&nbsp;{{.}}.&nbsp;&nbsp;</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td>
<td class="amt">$ -</td><td class="amt2"></td>
</tr>{{end}}
<tr>
<td colspan="7" class="bb1">&nbsp;&nbsp;6. (&nbsp;&nbsp;&nbsp;) OTHERS (LIST INDIVIDUALLY ON BUDGET JUSTIFICATION PAGE)</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td>
<td class="amt">$ -</td><td class="amt2"></td>
</tr>
<tr>
<td colspan="7" class="bb1">&nbsp;&nbsp;7. ( {{$page.TotalSeniorCount}} ) TOTAL SENIOR PERSONNEL (1-6)</td>
<td class="pm">{{fmtPM $page.TotalSeniorCAL}}</td><td class="pm">{{fmtPM $page.TotalSeniorACAD}}</td><td class="pm">{{fmtPM $page.TotalSeniorSUMR}}</td>
<td class="amt">{{dollar $page.TotalSeniorAmount}}</td><td class="amt2"></td>
</tr>

<!-- Section B -->
<tr><td colspan="7" class="bb1">B. OTHER PERSONNEL (SHOW NUMBERS IN BRACKETS)</td><td class="gray bb1" colspan="4"></td><td class="gray bb1 br2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;1. ( {{$page.PostDocCount}} ) POST DOCTORAL ASSOCIATES</td>
<td class="pm">{{fmtPM $page.PostDocCAL}}</td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.PostDocAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;2. ( {{$page.OtherProfCount}} ) OTHER PROFESSIONALS (TECHNICIAN, PROGRAMMER, ETC.)</td>
<td class="pm">{{fmtPM $page.OtherProfCAL}}</td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.OtherProfAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;3. ( {{$page.GradStudentCount}} ) GRADUATE STUDENTS</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.GradStudentAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;4. ( {{$page.UndergradCount}} ) UNDERGRADUATE STUDENTS</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.UndergradAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;5. ( {{$page.ClericalCount}} ) SECRETARIAL - CLERICAL (IF CHARGED DIRECTLY)</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.ClericalAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="7" class="bb1">&nbsp;&nbsp;6. ( {{$page.OtherPersonnelCount}} ) OTHER</td>
<td class="pm"></td><td class="pm"></td><td class="pm"></td><td class="amt">{{dollar $page.OtherPersonnelAmount}}</td><td class="amt2"></td></tr>

<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL SALARIES AND WAGES (A+B)</td><td class="amt">{{dollar $page.TotalSalariesWages}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">C. FRINGE BENEFITS (IF CHARGED AS DIRECT COSTS)</td><td class="amt">{{dollar $page.FringeBenefits}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL SALARIES, WAGES AND FRINGE BENEFITS (A+B+C)</td><td class="amt">{{dollar $page.TotalSalWageFringe}}</td><td class="amt2"></td></tr>

<!-- D. Equipment -->
<tr><td colspan="10">D. EQUIPMENT (LIST ITEM AND DOLLAR AMOUNT FOR EACH ITEM EXCEEDING $5,000)</td><td class="gray" colspan="2"></td></tr>
{{range $page.EquipmentItems}}<tr><td colspan="10">&nbsp;&nbsp;{{.}}</td><td class="gray" colspan="2"></td></tr>{{end}}
{{range emptySlots (len $page.EquipmentItems) 6}}<tr><td colspan="10">&nbsp;</td><td class="gray" colspan="2"></td></tr>{{end}}
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL EQUIPMENT</td><td class="amt">{{dollar $page.TotalEquipment}}</td><td class="amt2"></td></tr>

<!-- E. Travel -->
<tr><td colspan="1">E. TRAVEL</td><td colspan="9" class="bb1">1. DOMESTIC (INCL. CANADA, MEXICO AND U.S. POSSESSIONS)</td><td class="amt">{{dollar $page.DomesticTravel}}</td><td class="amt2"></td></tr>
<tr><td></td><td colspan="9" class="bb1">2. FOREIGN</td><td class="amt">{{dollar $page.ForeignTravel}}</td><td class="amt2"></td></tr>

<!-- F. Participant Support -->
<tr><td colspan="10">F. PARTICIPANT SUPPORT COSTS</td><td class="gray" colspan="2"></td></tr>
<tr><td colspan="10">&nbsp;&nbsp;&nbsp;1. STIPENDS</td><td class="gray" colspan="2"></td></tr>
<tr><td colspan="10">&nbsp;&nbsp;&nbsp;2. TRAVEL</td><td class="gray" colspan="2"></td></tr>
<tr><td colspan="10">&nbsp;&nbsp;&nbsp;3. SUBSISTENCE</td><td class="gray" colspan="2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;4. OTHER</td><td class="gray bb1" colspan="2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL PARTICIPANT SUPPORT COSTS</td><td class="amt">{{dollar $page.TotalParticipant}}</td><td class="amt2"></td></tr>

<!-- G. Other Direct Costs -->
<tr><td colspan="10" class="bb1">G. OTHER DIRECT COSTS</td><td class="gray bb1" colspan="2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;1. MATERIALS AND SUPPLIES</td><td class="amt">{{dollar $page.MaterialsSupplies}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;2. PUBLICATION COSTS/DOCUMENTATION/DISSEMINATION</td><td class="amt">{{dollar $page.Publication}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;3. CONSULTANT SERVICES</td><td class="amt">{{dollar $page.Consultant}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;4. COMPUTER SERVICES</td><td class="amt">{{dollar $page.ComputerServices}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;5. SUBAWARDS&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;Number of subawardees: {{$page.SubawardCount}}</td><td class="amt">{{dollar $page.SubawardAmount}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;6. OTHER</td><td class="amt">{{dollar $page.OtherDirect}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL OTHER DIRECT COSTS</td><td class="amt">{{dollar $page.TotalOtherDirect}}</td><td class="amt2"></td></tr>

<!-- H. Total Direct -->
<tr><td colspan="10" class="bb1">H. TOTAL DIRECT COSTS (A THROUGH G)</td><td class="amt">{{dollar $page.TotalDirectCosts}}</td><td class="amt2"></td></tr>

<!-- I. Indirect -->
<tr><td colspan="10">I. INDIRECT COSTS (F&amp;A) (SPECIFY RATE AND BASE)</td><td class="gray" colspan="2"></td></tr>
{{range $page.IndirectLines}}<tr><td colspan="2">{{.Label}}</td><td colspan="2" class="bold">{{.Rate}}</td><td class="right">Base =</td><td class="right">{{dollar .Base}}</td><td colspan="4"></td><td class="gray" colspan="2"></td></tr>{{end}}
{{if not $page.IndirectLines}}<tr><td colspan="10">&nbsp;</td><td class="gray" colspan="2"></td></tr>{{end}}
<tr><td colspan="10" class="bb1">&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;TOTAL INDIRECT COSTS (F&amp;A)</td><td class="amt">{{dollar $page.TotalIndirectCosts}}</td><td class="amt2"></td></tr>

<!-- J-L -->
<tr><td colspan="10" class="bb1">J. TOTAL DIRECT AND INDIRECT COSTS (H+I)</td><td class="amt">{{dollar $page.TotalDirectIndirect}}</td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb1">K. RESIDUAL FUNDS (IF FOR FURTHER SUPPORT OF CURRENT PROJECT SEE GPG II.D.7.j.)</td><td class="bb1"></td><td class="amt2"></td></tr>
<tr><td colspan="10" class="bb2">L. AMOUNT OF THIS REQUEST (J) OR (J MINUS K)</td><td class="total-amt">{{dollar $page.AmountOfRequest}}</td><td class="total-amt2"></td></tr>

<!-- M -->
<tr><td colspan="5" class="bb2">M. COST-SHARING: PROPOSED LEVEL $</td><td colspan="7" class="bb2">&nbsp;&nbsp;&nbsp;&nbsp;AGREED LEVEL IF DIFFERENT $</td></tr>

<!-- Footer -->
<tr><td colspan="5">PI/PD TYPED NAME &amp; SIGNATURE*</td><td>DATE</td><td colspan="6" class="bold">FOR NSF USE ONLY</td></tr>
<tr><td colspan="5" class="bb1">{{$page.PIName}}</td><td class="bb1"></td><td colspan="6" class="bb1">INDIRECT COST RATE VERIFICATION</td></tr>
<tr><td colspan="5">ORG. REP. TYPED NAME &amp; SIGNATURE*</td><td>DATE</td><td colspan="2">Date Checked</td><td colspan="2">Date of Rate Sheet</td><td colspan="2">Initials-ORG</td></tr>
<tr><td colspan="5" class="bb2"></td><td class="bb2"></td><td colspan="2" class="bb2"></td><td colspan="2" class="bb2"></td><td colspan="2" class="bb2"></td></tr>
<tr><td colspan="5" class="bold">NSF FORM 1030 (10/97) Supersedes All Previous Editions</td><td colspan="7">&nbsp;&nbsp;&nbsp;&nbsp;*SIGNATURES REQUIRED ONLY FOR REVISED BUDGET (GPG III. B)</td></tr>
</table>
</div>
{{end}}
</body></html>`
