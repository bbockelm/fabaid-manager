package docextract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

// ExtractedTable holds structured data extracted from a document.
type ExtractedTable struct {
	SheetName string     `json:"sheet_name,omitempty"`
	Headers   []string   `json:"headers,omitempty"`
	Rows      [][]string `json:"rows"`
}

// ExtractionResult is everything we could pull from a file.
type ExtractionResult struct {
	Filename   string           `json:"filename"`
	Format     string           `json:"format"` // "xlsx", "xls", "csv", "pdf", "unknown"
	Tables     []ExtractedTable `json:"tables,omitempty"`
	RawText    string           `json:"raw_text,omitempty"`    // for PDFs / fallback
	Truncated  bool             `json:"truncated,omitempty"`   // true if content was cut for size
}

// maxCellsPerSheet caps the number of cells we extract to avoid blowing up context.
const maxCellsPerSheet = 20000

// Extract parses the given file content and returns structured data.
func Extract(filename string, data []byte) (*ExtractionResult, error) {
	ext := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(ext, ".xlsx"):
		return extractExcel(filename, data)
	case strings.HasSuffix(ext, ".xls"):
		return extractXLS(filename, data)
	case strings.HasSuffix(ext, ".csv"):
		return extractCSV(filename, data)
	case strings.HasSuffix(ext, ".pdf"):
		return extractPDF(filename, data)
	default:
		// Try Excel first, then fall back to text
		res, err := extractExcel(filename, data)
		if err == nil {
			return res, nil
		}
		return &ExtractionResult{
			Filename: filename,
			Format:   "unknown",
			RawText:  truncateString(string(data), 50000),
		}, nil
	}
}

func extractExcel(filename string, data []byte) (*ExtractionResult, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("opening excel file: %w", err)
	}
	defer f.Close()

	result := &ExtractionResult{
		Filename: filename,
		Format:   "xlsx",
	}

	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			log.Warn().Err(err).Str("sheet", sheet).Msg("Failed to read sheet, skipping")
			continue
		}

		table := ExtractedTable{
			SheetName: sheet,
		}

		cellCount := 0
		for i, row := range rows {
			if i == 0 {
				table.Headers = row
			}
			table.Rows = append(table.Rows, row)
			cellCount += len(row)
			if cellCount > maxCellsPerSheet {
				result.Truncated = true
				break
			}
		}
		result.Tables = append(result.Tables, table)
	}

	return result, nil
}

func extractCSV(filename string, data []byte) (*ExtractionResult, error) {
	lines := strings.Split(string(data), "\n")
	table := ExtractedTable{SheetName: "CSV"}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Simple CSV split (doesn't handle quoted commas, but good enough for most budgets)
		cells := strings.Split(line, ",")
		if i == 0 {
			table.Headers = cells
		}
		table.Rows = append(table.Rows, cells)
	}
	return &ExtractionResult{
		Filename: filename,
		Format:   "csv",
		Tables:   []ExtractedTable{table},
	}, nil
}

func extractPDF(filename string, data []byte) (*ExtractionResult, error) {
	// Try pdftotext first (from poppler-utils)
	text, err := runPDFToText(data)
	if err != nil {
		log.Warn().Err(err).Msg("pdftotext failed, returning raw bytes hint")
		return &ExtractionResult{
			Filename: filename,
			Format:   "pdf",
			RawText:  "[PDF extraction failed. The LLM may need to process this document using vision capabilities.]",
		}, nil
	}

	return &ExtractionResult{
		Filename:  filename,
		Format:    "pdf",
		RawText:   truncateString(text, 80000),
		Truncated: len(text) > 80000,
	}, nil
}

func runPDFToText(data []byte) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", "-", "-")
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// extractXLS handles legacy .xls (BIFF) files by converting via Python/xlrd.
func extractXLS(filename string, data []byte) (*ExtractionResult, error) {
	// Write data to a temp file
	tmpFile, err := os.CreateTemp("", "fabaid-xls-*.xls")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	tmpFile.Close()

	// Use Python + xlrd to dump all sheets as CSV-like output
	script := `
import sys, json, xlrd
wb = xlrd.open_workbook(sys.argv[1])
result = []
for sheet in wb.sheets():
    rows = []
    for rx in range(sheet.nrows):
        row = []
        for cx in range(sheet.ncols):
            cell = sheet.cell(rx, cx)
            if cell.ctype == xlrd.XL_CELL_NUMBER:
                v = cell.value
                row.append(str(int(v)) if v == int(v) else str(v))
            elif cell.ctype == xlrd.XL_CELL_DATE:
                row.append(xlrd.xldate_as_datetime(cell.value, wb.datemode).isoformat())
            else:
                row.append(str(cell.value))
        rows.append(row)
    result.append({"name": sheet.name, "rows": rows})
print(json.dumps(result))
`
	cmd := exec.Command("python3", "-c", script, tmpPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python3 xlrd conversion failed: %w: %s", err, stderr.String())
	}

	var sheets []struct {
		Name string     `json:"name"`
		Rows [][]string `json:"rows"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &sheets); err != nil {
		return nil, fmt.Errorf("parsing xlrd output: %w", err)
	}

	result := &ExtractionResult{
		Filename: filename,
		Format:   "xls",
	}

	for _, sheet := range sheets {
		table := ExtractedTable{
			SheetName: sheet.Name,
		}
		cellCount := 0
		for i, row := range sheet.Rows {
			if i == 0 {
				table.Headers = row
			}
			table.Rows = append(table.Rows, row)
			cellCount += len(row)
			if cellCount > maxCellsPerSheet {
				result.Truncated = true
				break
			}
		}
		result.Tables = append(result.Tables, table)
	}

	return result, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}

// ToMarkdown converts an extraction result to a markdown representation
// suitable for passing to an LLM.
func (r *ExtractionResult) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Extracted Data from %s\n\n", r.Filename))
	sb.WriteString(fmt.Sprintf("Format: %s\n\n", r.Format))

	if r.Truncated {
		sb.WriteString("**Note: Content was truncated due to size limits.**\n\n")
	}

	for _, table := range r.Tables {
		if table.SheetName != "" {
			sb.WriteString(fmt.Sprintf("## Sheet: %s\n\n", table.SheetName))
		}

		// Render as pipe-delimited table with row numbers
		for i, row := range table.Rows {
			sb.WriteString(fmt.Sprintf("Row %d: ", i+1))
			for j, cell := range row {
				if j > 0 {
					sb.WriteString(" | ")
				}
				sb.WriteString(strings.TrimSpace(cell))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if r.RawText != "" {
		sb.WriteString("## Document Text\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.RawText)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}
