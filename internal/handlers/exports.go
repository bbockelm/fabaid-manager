package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// sortedWBSSummary returns the summaries ordered by WBS code then fiscal year
// so exports are deterministic and readable.
func sortedWBSSummary(s []models.WBSEffortSummary) []models.WBSEffortSummary {
	out := make([]models.WBSEffortSummary, len(s))
	copy(out, s)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WBSCode != out[j].WBSCode {
			return out[i].WBSCode < out[j].WBSCode
		}
		return out[i].FiscalYear < out[j].FiscalYear
	})
	return out
}

// writeWBSSummaryCSV writes the WBS effort/cost breakdown as a downloadable CSV.
func writeWBSSummaryCSV(w http.ResponseWriter, summaries []models.WBSEffortSummary) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="wbs-effort-summary.csv"`)
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"WBS Code", "WBS Area", "Fiscal Year", "Effort (person-months)", "Estimated Cost (USD)"})
	var totEffort, totCost float64
	for _, s := range sortedWBSSummary(summaries) {
		_ = cw.Write([]string{
			s.WBSCode, s.WBSName, strconv.Itoa(s.FiscalYear),
			strconv.FormatFloat(s.EffortMonths, 'f', 2, 64),
			strconv.FormatFloat(s.Amount, 'f', 2, 64),
		})
		totEffort += s.EffortMonths
		totCost += s.Amount
	}
	_ = cw.Write([]string{"", "TOTAL", "", strconv.FormatFloat(totEffort, 'f', 2, 64), strconv.FormatFloat(totCost, 'f', 2, 64)})
}

// writeWBSSummaryMarkdown writes the WBS effort/cost breakdown as a Markdown table.
func writeWBSSummaryMarkdown(w http.ResponseWriter, summaries []models.WBSEffortSummary) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="wbs-effort-summary.md"`)
	fmt.Fprintln(w, "# WBS Effort & Estimated Cost Breakdown")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| WBS Code | WBS Area | Fiscal Year | Effort (person-months) | Estimated Cost |")
	fmt.Fprintln(w, "|---|---|---:|---:|---:|")
	var totEffort, totCost float64
	for _, s := range sortedWBSSummary(summaries) {
		fmt.Fprintf(w, "| %s | %s | %d | %.2f | $%s |\n",
			s.WBSCode, s.WBSName, s.FiscalYear, s.EffortMonths, humanUSD(s.Amount))
		totEffort += s.EffortMonths
		totCost += s.Amount
	}
	fmt.Fprintf(w, "| | **Total** | | **%.2f** | **$%s** |\n", totEffort, humanUSD(totCost))
}

// humanUSD formats a dollar amount with thousands separators and no decimals.
func humanUSD(v float64) string {
	n := int64(v + 0.5)
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	// insert commas
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
