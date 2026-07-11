package db

import (
	"context"
	"fmt"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// This file holds all invoice, invoice-expense, and expense-analytics queries.
// Invoices are polymorphic over (entity_type, entity_id) where entity_type is
// 'grant' (the lead institution) or 'subaward'. Actuals roll up finalized
// invoice expenses; any WBS remainder (allocations not summing to 100%) or a
// line_type of 'uncategorized' is treated as uncategorized and surfaced.

const invoiceCols = `id, entity_type, entity_id, subaward_id, COALESCE(invoice_number,''),
	invoice_date::text, amount, COALESCE(period_start::text,''), COALESCE(period_end::text,''),
	status, coding_status, document_id, fiscal_year, COALESCE(notes,''), deleted_at, created_at, updated_at`

// scanInvoice scans a row selected with invoiceCols into an Invoice.
func scanInvoice(s interface{ Scan(...any) error }) (models.Invoice, error) {
	var inv models.Invoice
	var ps, pe string
	if err := s.Scan(&inv.ID, &inv.EntityType, &inv.EntityID, &inv.SubawardID, &inv.InvoiceNumber,
		&inv.InvoiceDate, &inv.Amount, &ps, &pe, &inv.Status, &inv.CodingStatus,
		&inv.DocumentID, &inv.FiscalYear, &inv.Notes, &inv.DeletedAt, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
		return inv, err
	}
	if ps != "" {
		inv.PeriodStart = &ps
	}
	if pe != "" {
		inv.PeriodEnd = &pe
	}
	return inv, nil
}

// grantEntityScope is a reusable WHERE fragment matching all invoices belonging to
// a grant: the lead grant entity plus every subaward under it. $1 must be the grant ID.
const grantEntityScope = `(
	(i.entity_type='grant' AND i.entity_id=$1)
	OR (i.entity_type='subaward' AND i.entity_id IN (SELECT id FROM subawards WHERE grant_id=$1))
)`

// ListInvoices lists invoices for a subaward (legacy subaward-scoped view).
func (q *Queries) ListInvoices(ctx context.Context, subawardID string) ([]models.Invoice, error) {
	return q.ListInvoicesByEntity(ctx, "subaward", subawardID)
}

// ListInvoicesByEntity lists invoices for one billing entity.
func (q *Queries) ListInvoicesByEntity(ctx context.Context, entityType, entityID string) ([]models.Invoice, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+invoiceCols+`
		FROM invoices WHERE entity_type=$1 AND entity_id=$2 AND deleted_at IS NULL
		ORDER BY invoice_date DESC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()
	var invoices []models.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// ListInvoicesForGrant lists all invoices across a grant and its subawards.
func (q *Queries) ListInvoicesForGrant(ctx context.Context, grantID string) ([]models.Invoice, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+invoiceCols+`
		FROM invoices i WHERE `+grantEntityScope+` AND i.deleted_at IS NULL
		ORDER BY invoice_date DESC`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing grant invoices: %w", err)
	}
	defer rows.Close()
	var invoices []models.Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

// GetInvoice fetches a single invoice by ID.
func (q *Queries) GetInvoice(ctx context.Context, id string) (*models.Invoice, error) {
	inv, err := scanInvoice(q.pool.QueryRow(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE id=$1`, id))
	if err != nil {
		return nil, fmt.Errorf("getting invoice: %w", err)
	}
	return &inv, nil
}

// CreateInvoice inserts a new invoice. SubawardID is populated automatically when
// entity_type='subaward' so legacy subaward-scoped reads keep working.
func (q *Queries) CreateInvoice(ctx context.Context, inv *models.Invoice) error {
	if inv.Status == "" {
		inv.Status = "pending"
	}
	if inv.CodingStatus == "" {
		inv.CodingStatus = "uncoded"
	}
	if inv.EntityType == "subaward" && inv.SubawardID == nil {
		inv.SubawardID = &inv.EntityID
	}
	return q.pool.QueryRow(ctx, `
		INSERT INTO invoices (entity_type, entity_id, subaward_id, invoice_number, invoice_date,
			amount, period_start, period_end, status, coding_status, document_id, fiscal_year, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at`,
		inv.EntityType, inv.EntityID, inv.SubawardID, inv.InvoiceNumber, inv.InvoiceDate,
		inv.Amount, inv.PeriodStart, inv.PeriodEnd, inv.Status, inv.CodingStatus,
		inv.DocumentID, inv.FiscalYear, inv.Notes,
	).Scan(&inv.ID, &inv.CreatedAt, &inv.UpdatedAt)
}

// UpdateInvoice updates the mutable header fields of an invoice, including its
// billing institution (entity) so a draft can be re-assigned via the dropdown.
func (q *Queries) UpdateInvoice(ctx context.Context, inv *models.Invoice) error {
	// Keep the legacy subaward_id consistent with the entity.
	if inv.EntityType == "subaward" {
		inv.SubawardID = &inv.EntityID
	} else {
		inv.SubawardID = nil
	}
	_, err := q.pool.Exec(ctx, `
		UPDATE invoices SET entity_type=$2, entity_id=$3, subaward_id=$4, invoice_number=$5,
			invoice_date=$6, amount=$7, period_start=$8, period_end=$9, status=$10,
			coding_status=$11, document_id=$12, fiscal_year=$13, notes=$14, updated_at=now()
		WHERE id=$1`,
		inv.ID, inv.EntityType, inv.EntityID, inv.SubawardID, inv.InvoiceNumber,
		inv.InvoiceDate, inv.Amount, inv.PeriodStart, inv.PeriodEnd, inv.Status,
		inv.CodingStatus, inv.DocumentID, inv.FiscalYear, inv.Notes)
	return err
}

// UpdateInvoiceStatus sets the payment status.
func (q *Queries) UpdateInvoiceStatus(ctx context.Context, id, status string) error {
	_, err := q.pool.Exec(ctx, `UPDATE invoices SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

// SetInvoiceCodingStatus sets the coding status (uncoded/draft/final).
func (q *Queries) SetInvoiceCodingStatus(ctx context.Context, id, codingStatus string) error {
	_, err := q.pool.Exec(ctx, `UPDATE invoices SET coding_status=$2, updated_at=now() WHERE id=$1`, id, codingStatus)
	return err
}

// SetInvoiceDocument links an uploaded document to an invoice.
func (q *Queries) SetInvoiceDocument(ctx context.Context, id, documentID string) error {
	_, err := q.pool.Exec(ctx, `UPDATE invoices SET document_id=$2, updated_at=now() WHERE id=$1`, id, documentID)
	return err
}

// DeleteInvoice removes an invoice (expenses cascade).
func (q *Queries) DeleteInvoice(ctx context.Context, id string) error {
	// Soft delete so invoices can be recovered/audited.
	_, err := q.pool.Exec(ctx, `UPDATE invoices SET deleted_at=now(), updated_at=now() WHERE id=$1`, id)
	return err
}

// --- Invoice Expenses ---

const expenseCols = `id, invoice_id, line_type, COALESCE(description,''), amount, personnel_id,
	budget_line_item_id, COALESCE(notes,''), sort_order, created_at, updated_at`

func scanExpense(s interface{ Scan(...any) error }) (models.InvoiceExpense, error) {
	var e models.InvoiceExpense
	err := s.Scan(&e.ID, &e.InvoiceID, &e.LineType, &e.Description, &e.Amount, &e.PersonnelID,
		&e.BudgetLineItemID, &e.Notes, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

// ListInvoiceExpenses returns the billed expense lines of an invoice.
func (q *Queries) ListInvoiceExpenses(ctx context.Context, invoiceID string) ([]models.InvoiceExpense, error) {
	rows, err := q.pool.Query(ctx, `SELECT `+expenseCols+`
		FROM invoice_expenses WHERE invoice_id=$1 ORDER BY sort_order, created_at`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("listing invoice expenses: %w", err)
	}
	defer rows.Close()
	var out []models.InvoiceExpense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning invoice expense: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// GetInvoiceExpense fetches one expense line.
func (q *Queries) GetInvoiceExpense(ctx context.Context, id string) (*models.InvoiceExpense, error) {
	e, err := scanExpense(q.pool.QueryRow(ctx, `SELECT `+expenseCols+` FROM invoice_expenses WHERE id=$1`, id))
	if err != nil {
		return nil, fmt.Errorf("getting invoice expense: %w", err)
	}
	return &e, nil
}

// CreateInvoiceExpense inserts an expense line.
func (q *Queries) CreateInvoiceExpense(ctx context.Context, e *models.InvoiceExpense) error {
	if e.LineType == "" {
		e.LineType = "uncategorized"
	}
	return q.pool.QueryRow(ctx, `
		INSERT INTO invoice_expenses (invoice_id, line_type, description, amount, personnel_id,
			budget_line_item_id, notes, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`,
		e.InvoiceID, e.LineType, e.Description, e.Amount, e.PersonnelID,
		e.BudgetLineItemID, e.Notes, e.SortOrder,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// UpdateInvoiceExpense updates an expense line.
func (q *Queries) UpdateInvoiceExpense(ctx context.Context, e *models.InvoiceExpense) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE invoice_expenses SET line_type=$2, description=$3, amount=$4, personnel_id=$5,
			budget_line_item_id=$6, notes=$7, sort_order=$8, updated_at=now()
		WHERE id=$1`,
		e.ID, e.LineType, e.Description, e.Amount, e.PersonnelID,
		e.BudgetLineItemID, e.Notes, e.SortOrder)
	return err
}

// DeleteInvoiceExpense removes an expense line (its WBS allocations cascade).
func (q *Queries) DeleteInvoiceExpense(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM invoice_expenses WHERE id=$1`, id)
	return err
}

// --- Invoice Expense WBS allocations ---

// ListInvoiceExpenseWBS returns a single expense line's WBS allocations.
func (q *Queries) ListInvoiceExpenseWBS(ctx context.Context, expenseID string) ([]models.InvoiceExpenseWBS, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, invoice_expense_id, wbs_area_id, allocation_percent
		FROM invoice_expense_wbs WHERE invoice_expense_id=$1 ORDER BY allocation_percent DESC`, expenseID)
	if err != nil {
		return nil, fmt.Errorf("listing expense WBS: %w", err)
	}
	defer rows.Close()
	var out []models.InvoiceExpenseWBS
	for rows.Next() {
		var w models.InvoiceExpenseWBS
		if err := rows.Scan(&w.ID, &w.InvoiceExpenseID, &w.WBSAreaID, &w.AllocationPercent); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// SetInvoiceExpenseWBS replaces an expense line's WBS allocations. Allocations
// are not required to sum to 100% — any remainder is treated as uncategorized.
func (q *Queries) SetInvoiceExpenseWBS(ctx context.Context, expenseID string, allocations []models.InvoiceExpenseWBS) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM invoice_expense_wbs WHERE invoice_expense_id=$1`, expenseID); err != nil {
		return fmt.Errorf("deleting old expense WBS: %w", err)
	}
	for i := range allocations {
		a := &allocations[i]
		if a.AllocationPercent == 0 {
			continue
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO invoice_expense_wbs (invoice_expense_id, wbs_area_id, allocation_percent)
			VALUES ($1,$2,$3) RETURNING id`,
			expenseID, a.WBSAreaID, a.AllocationPercent,
		).Scan(&a.ID); err != nil {
			return fmt.Errorf("inserting expense WBS: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// --- Analytics ---

// ListBillingEntities returns every institution that bills against a grant — the
// lead grant plus each subaward — with its total budget (burn denominator).
func (q *Queries) ListBillingEntities(ctx context.Context, grantID string) ([]models.BillingEntity, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT 'grant' AS entity_type, id AS entity_id, institution, total_budget,
		       start_date::text, end_date::text
		FROM grants WHERE id=$1
		UNION ALL
		SELECT 'subaward', id, institution, total_amount, start_date::text, end_date::text
		FROM subawards WHERE grant_id=$1
		ORDER BY entity_type DESC, institution`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing billing entities: %w", err)
	}
	defer rows.Close()
	var out []models.BillingEntity
	for rows.Next() {
		var b models.BillingEntity
		if err := rows.Scan(&b.EntityType, &b.EntityID, &b.Institution, &b.TotalBudget, &b.StartDate, &b.EndDate); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

// ListFinalizedExpensesForGrant returns all finalized invoice expenses across a
// grant and its subawards, each with its WBS allocations attached, for actuals
// and burn-rate rollups (computed in the handler layer).
func (q *Queries) ListFinalizedExpensesForGrant(ctx context.Context, grantID string) ([]models.FinalizedExpense, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT e.id, e.invoice_id, i.entity_type, i.entity_id, e.line_type, e.amount,
		       i.invoice_date::text, COALESCE(i.period_end::text,'')
		FROM invoice_expenses e
		JOIN invoices i ON i.id = e.invoice_id
		WHERE i.coding_status='final' AND i.deleted_at IS NULL AND `+grantEntityScope+`
		ORDER BY i.invoice_date`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing finalized expenses: %w", err)
	}
	defer rows.Close()

	var out []models.FinalizedExpense
	byID := map[string]int{}
	for rows.Next() {
		var fe models.FinalizedExpense
		var pe string
		if err := rows.Scan(&fe.ExpenseID, &fe.InvoiceID, &fe.EntityType, &fe.EntityID,
			&fe.LineType, &fe.Amount, &fe.InvoiceDate, &pe); err != nil {
			return nil, err
		}
		if pe != "" {
			fe.PeriodEnd = &pe
		}
		byID[fe.ExpenseID] = len(out)
		out = append(out, fe)
	}
	rows.Close()
	if len(out) == 0 {
		return out, nil
	}

	// Attach WBS allocations for all finalized expenses in this grant.
	wrows, err := q.pool.Query(ctx, `
		SELECT w.id, w.invoice_expense_id, w.wbs_area_id, w.allocation_percent
		FROM invoice_expense_wbs w
		JOIN invoice_expenses e ON e.id = w.invoice_expense_id
		JOIN invoices i ON i.id = e.invoice_id
		WHERE i.coding_status='final' AND i.deleted_at IS NULL AND `+grantEntityScope, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing finalized expense WBS: %w", err)
	}
	defer wrows.Close()
	for wrows.Next() {
		var w models.InvoiceExpenseWBS
		if err := wrows.Scan(&w.ID, &w.InvoiceExpenseID, &w.WBSAreaID, &w.AllocationPercent); err != nil {
			return nil, err
		}
		if idx, ok := byID[w.InvoiceExpenseID]; ok {
			out[idx].WBS = append(out[idx].WBS, w)
		}
	}
	return out, nil
}
