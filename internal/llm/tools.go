package llm

import "encoding/json"

// BudgetTools returns the tool definitions for the budget processing LLM.
func BudgetTools() []Tool {
	return []Tool{
		makeTool("create_budget", "Create a new draft institution budget for a specific fiscal year. Returns the new budget ID.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fiscal_year": map[string]any{"type": "integer", "description": "Fiscal year number (1-5)"},
				"budget":      map[string]any{"type": "number", "description": "Total budget amount for this year"},
				"notes":       map[string]any{"type": "string", "description": "Optional notes about this budget"},
			},
			"required": []string{"fiscal_year"},
		})),

		makeTool("create_line_item", "Add a new line item to a budget for a specific fiscal year.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fiscal_year":  map[string]any{"type": "integer", "description": "Fiscal year (1-5) — the line item will be added to the latest draft budget for this year"},
				"line_type":    map[string]any{"type": "string", "enum": []string{"personnel", "fringe", "travel", "equipment", "supplies", "contractual", "participant_support", "tuition", "other"}, "description": "Type of cost"},
				"description":  map[string]any{"type": "string", "description": "Description of the line item (e.g., person's name for personnel, 'Domestic travel' for travel)"},
				"personnel_id": map[string]any{"type": "string", "description": "Personnel UUID if line_type is 'personnel' or 'fringe'"},
				"effort_months": map[string]any{"type": "number", "description": "Person-months of effort (for personnel lines)"},
				"amount":       map[string]any{"type": "number", "description": "Dollar amount for this line item"},
				"notes":        map[string]any{"type": "string", "description": "Optional notes (e.g., fringe rate calculation)"},
			},
			"required": []string{"fiscal_year", "line_type", "amount"},
		})),

		makeTool("update_line_item", "Update an existing budget line item.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"line_item_id":  map[string]any{"type": "string", "description": "UUID of the line item to update"},
				"description":   map[string]any{"type": "string", "description": "New description"},
				"personnel_id":  map[string]any{"type": "string", "description": "New personnel UUID"},
				"effort_months": map[string]any{"type": "number", "description": "New effort months"},
				"amount":        map[string]any{"type": "number", "description": "New dollar amount"},
				"notes":         map[string]any{"type": "string", "description": "New notes"},
			},
			"required": []string{"line_item_id"},
		})),

		makeTool("delete_line_item", "Delete a budget line item.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"line_item_id": map[string]any{"type": "string", "description": "UUID of the line item to delete"},
			},
			"required": []string{"line_item_id"},
		})),

		makeTool("create_personnel", "Create a new personnel record for the grant.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":          map[string]any{"type": "string", "description": "Person's full name"},
				"role":          map[string]any{"type": "string", "enum": []string{"pi", "co_pi", "subaward_pi", "senior_personnel", "postdoc", "other_professional", "graduate_student", "undergraduate_student", "clerical", "other"}, "description": "NSF role category"},
				"title":         map[string]any{"type": "string", "description": "Job title (e.g., 'Research Programmer', 'Graduate Research Assistant')"},
				"institution":   map[string]any{"type": "string", "description": "Institution name"},
				"annual_salary": map[string]any{"type": "number", "description": "Base annual salary"},
				"funded_months": map[string]any{"type": "number", "description": "Total funded months across the grant"},
			},
			"required": []string{"name", "role", "institution", "annual_salary"},
		})),

		makeTool("update_personnel", "Update an existing personnel record.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"personnel_id":  map[string]any{"type": "string", "description": "UUID of the personnel to update"},
				"name":          map[string]any{"type": "string", "description": "Updated name"},
				"role":          map[string]any{"type": "string", "description": "Updated NSF role"},
				"title":         map[string]any{"type": "string", "description": "Updated job title"},
				"annual_salary": map[string]any{"type": "number", "description": "Updated base annual salary"},
				"funded_months": map[string]any{"type": "number", "description": "Updated total funded months"},
			},
			"required": []string{"personnel_id"},
		})),

		makeTool("upsert_fringe_rate", "Set a fringe benefit rate for the institution for a specific fiscal year. Creates or updates.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fiscal_year": map[string]any{"type": "integer", "description": "Fiscal year (1-5)"},
				"rate_name":   map[string]any{"type": "string", "description": "Name of the fringe rate (e.g., 'default', 'FICA', 'Health')"},
				"rate":        map[string]any{"type": "number", "description": "Rate as a decimal (e.g., 0.3138 for 31.38%)"},
			},
			"required": []string{"fiscal_year", "rate_name", "rate"},
		})),

		makeTool("create_overhead_rate", "Create a new overhead (F&A) rate for the institution.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rate_name":   map[string]any{"type": "string", "description": "Name (e.g., 'MTDC On-Campus', 'TDC')"},
				"rate":        map[string]any{"type": "number", "description": "Rate as decimal (e.g., 0.56 for 56%)"},
				"description": map[string]any{"type": "string", "description": "Description of this rate"},
			},
			"required": []string{"rate_name", "rate"},
		})),

		makeTool("update_overhead_rate", "Update an existing overhead rate.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"overhead_rate_id": map[string]any{"type": "string", "description": "UUID of the overhead rate"},
				"rate_name":        map[string]any{"type": "string", "description": "Updated name"},
				"rate":             map[string]any{"type": "number", "description": "Updated rate"},
				"description":      map[string]any{"type": "string", "description": "Updated description"},
			},
			"required": []string{"overhead_rate_id"},
		})),

		makeTool("update_subaward", "Update properties of the subaward (only if entity is a subaward).", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"total_amount":          map[string]any{"type": "number", "description": "Updated total subaward amount"},
				"salary_escalation_rate": map[string]any{"type": "number", "description": "Annual salary escalation rate (e.g., 0.03 for 3%)"},
			},
			"required": []string{},
		})),

		makeTool("report_summary", "Report a markdown summary of all changes made and items needing review. Call this as the final step.", mustJSON(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary_markdown": map[string]any{"type": "string", "description": "Markdown-formatted summary of: (1) what was done, (2) confidence level for key figures, (3) items that need human review"},
			},
			"required": []string{"summary_markdown"},
		})),
	}
}

func makeTool(name, description string, params json.RawMessage) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters:  params,
		},
	}
}

func mustJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
