package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bbockelm/fabaid-manager/internal/models"
)

// Queries provides database query methods.
type Queries struct {
	pool *pgxpool.Pool
}

// NewQueries creates a new Queries instance.
func NewQueries(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// ExecRaw executes a raw SQL string against the database.
func (q *Queries) ExecRaw(ctx context.Context, sql string) error {
	_, err := q.pool.Exec(ctx, sql)
	return err
}

// --- Grants ---

func (q *Queries) ListGrants(ctx context.Context) ([]models.Grant, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, award_number, title, pi_name, institution, agency,
		       start_date::text, end_date::text, total_budget, salary_escalation_rate,
		       status, created_at, updated_at
		FROM grants ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing grants: %w", err)
	}
	defer rows.Close()

	var grants []models.Grant
	for rows.Next() {
		var g models.Grant
		if err := rows.Scan(&g.ID, &g.AwardNumber, &g.Title, &g.PIName, &g.Institution, &g.Agency,
			&g.StartDate, &g.EndDate, &g.TotalBudget, &g.SalaryEscalationRate,
			&g.Status, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		grants = append(grants, g)
	}
	return grants, nil
}

func (q *Queries) GetGrant(ctx context.Context, id string) (*models.Grant, error) {
	var g models.Grant
	err := q.pool.QueryRow(ctx, `
		SELECT id, award_number, title, pi_name, institution, agency,
		       start_date::text, end_date::text, total_budget, salary_escalation_rate,
		       status, created_at, updated_at
		FROM grants WHERE id = $1`, id).Scan(
		&g.ID, &g.AwardNumber, &g.Title, &g.PIName, &g.Institution, &g.Agency,
		&g.StartDate, &g.EndDate, &g.TotalBudget, &g.SalaryEscalationRate,
		&g.Status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting grant: %w", err)
	}
	return &g, nil
}

func (q *Queries) CreateGrant(ctx context.Context, g *models.Grant) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO grants (award_number, title, pi_name, institution, agency, start_date, end_date,
		                    total_budget, salary_escalation_rate, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		g.AwardNumber, g.Title, g.PIName, g.Institution, g.Agency, g.StartDate, g.EndDate,
		g.TotalBudget, g.SalaryEscalationRate, g.Status,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

func (q *Queries) UpdateGrant(ctx context.Context, g *models.Grant) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE grants SET award_number=$2, title=$3, pi_name=$4, institution=$5, agency=$6,
		       start_date=$7, end_date=$8, total_budget=$9, salary_escalation_rate=$10,
		       status=$11, updated_at=now()
		WHERE id=$1`,
		g.ID, g.AwardNumber, g.Title, g.PIName, g.Institution, g.Agency, g.StartDate, g.EndDate,
		g.TotalBudget, g.SalaryEscalationRate, g.Status)
	return err
}

func (q *Queries) DeleteGrant(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM grants WHERE id=$1`, id)
	return err
}

// --- WBS Areas ---

func (q *Queries) ListWBSAreas(ctx context.Context, grantID string) ([]models.WBSArea, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, grant_id, parent_id, code, name, COALESCE(description, ''), budget, created_at, updated_at
		FROM wbs_areas WHERE grant_id=$1 ORDER BY code`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing WBS areas: %w", err)
	}
	defer rows.Close()

	var areas []models.WBSArea
	for rows.Next() {
		var a models.WBSArea
		if err := rows.Scan(&a.ID, &a.GrantID, &a.ParentID, &a.Code, &a.Name, &a.Description, &a.Budget, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning WBS area: %w", err)
		}
		areas = append(areas, a)
	}
	return areas, nil
}

func (q *Queries) GetWBSArea(ctx context.Context, id string) (*models.WBSArea, error) {
	var a models.WBSArea
	err := q.pool.QueryRow(ctx, `
		SELECT id, grant_id, parent_id, code, name, COALESCE(description, ''), budget, created_at, updated_at
		FROM wbs_areas WHERE id=$1`, id).Scan(
		&a.ID, &a.GrantID, &a.ParentID, &a.Code, &a.Name, &a.Description, &a.Budget, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting WBS area: %w", err)
	}
	return &a, nil
}

func (q *Queries) CreateWBSArea(ctx context.Context, a *models.WBSArea) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO wbs_areas (grant_id, parent_id, code, name, description, budget)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		a.GrantID, a.ParentID, a.Code, a.Name, a.Description, a.Budget,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (q *Queries) UpdateWBSArea(ctx context.Context, a *models.WBSArea) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE wbs_areas SET parent_id=$2, code=$3, name=$4, description=$5, budget=$6, updated_at=now()
		WHERE id=$1`,
		a.ID, a.ParentID, a.Code, a.Name, a.Description, a.Budget)
	return err
}

func (q *Queries) DeleteWBSArea(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM wbs_areas WHERE id=$1`, id)
	return err
}

// --- Personnel Default WBS ---

func (q *Queries) ListPersonnelDefaultWBS(ctx context.Context, personnelID string) ([]models.PersonnelDefaultWBS, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, personnel_id, wbs_area_id, percent, created_at, updated_at
		FROM personnel_default_wbs WHERE personnel_id=$1
		ORDER BY created_at`, personnelID)
	if err != nil {
		return nil, fmt.Errorf("listing default WBS: %w", err)
	}
	defer rows.Close()
	var items []models.PersonnelDefaultWBS
	for rows.Next() {
		var d models.PersonnelDefaultWBS
		if err := rows.Scan(&d.ID, &d.PersonnelID, &d.WBSAreaID, &d.Percent, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning default WBS: %w", err)
		}
		items = append(items, d)
	}
	return items, nil
}

func (q *Queries) SetPersonnelDefaultWBS(ctx context.Context, personnelID string, items []models.PersonnelDefaultWBS) ([]models.PersonnelDefaultWBS, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `DELETE FROM personnel_default_wbs WHERE personnel_id=$1`, personnelID)
	if err != nil {
		return nil, fmt.Errorf("clearing default WBS: %w", err)
	}

	var result []models.PersonnelDefaultWBS
	for _, item := range items {
		var d models.PersonnelDefaultWBS
		err := tx.QueryRow(ctx, `
			INSERT INTO personnel_default_wbs (personnel_id, wbs_area_id, percent)
			VALUES ($1, $2, $3)
			RETURNING id, personnel_id, wbs_area_id, percent, created_at, updated_at`,
			personnelID, item.WBSAreaID, item.Percent,
		).Scan(&d.ID, &d.PersonnelID, &d.WBSAreaID, &d.Percent, &d.CreatedAt, &d.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("inserting default WBS: %w", err)
		}
		result = append(result, d)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// --- WBS Effort Summary ---

// WBSEffortSummary returns effort-months and cost per WBS area per fiscal year,
// drawn from the latest institution budgets' line-item WBS allocations.
func (q *Queries) WBSEffortSummary(ctx context.Context, grantID string) ([]models.WBSEffortSummary, error) {
	return q.WBSEffortSummaryFiltered(ctx, grantID, nil)
}

// WBSEffortSummaryFiltered returns effort summaries optionally filtered by institution names.
// When institutions is nil or empty, all institutions are included.
func (q *Queries) WBSEffortSummaryFiltered(ctx context.Context, grantID string, institutions []string) ([]models.WBSEffortSummary, error) {
	var query string
	var args []interface{}

	if len(institutions) > 0 {
		query = `
		SELECT
			w.id AS wbs_area_id,
			w.code AS wbs_code,
			w.name AS wbs_name,
			ib.fiscal_year,
			COALESCE(SUM(bli.effort_months * bliw.allocation_percent / 100.0), 0) AS effort_months,
			COALESCE(SUM(bli.amount * bliw.allocation_percent / 100.0), 0) AS amount
		FROM wbs_areas w
		LEFT JOIN budget_line_item_wbs bliw ON bliw.wbs_area_id = w.id
		LEFT JOIN budget_line_items bli ON bli.id = bliw.line_item_id
		LEFT JOIN institution_budgets ib ON ib.id = bli.institution_budget_id AND ib.is_latest = true
			AND (
				(ib.entity_type = 'grant' AND ib.entity_id IN (SELECT id FROM grants WHERE id = $1 AND institution = ANY($2)))
				OR
				(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1 AND institution = ANY($2)))
			)
		WHERE w.grant_id = $1
		GROUP BY w.id, w.code, w.name, ib.fiscal_year
		ORDER BY w.code, ib.fiscal_year`
		args = []interface{}{grantID, institutions}
	} else {
		query = `
		SELECT
			w.id AS wbs_area_id,
			w.code AS wbs_code,
			w.name AS wbs_name,
			ib.fiscal_year,
			COALESCE(SUM(bli.effort_months * bliw.allocation_percent / 100.0), 0) AS effort_months,
			COALESCE(SUM(bli.amount * bliw.allocation_percent / 100.0), 0) AS amount
		FROM wbs_areas w
		LEFT JOIN budget_line_item_wbs bliw ON bliw.wbs_area_id = w.id
		LEFT JOIN budget_line_items bli ON bli.id = bliw.line_item_id
		LEFT JOIN institution_budgets ib ON ib.id = bli.institution_budget_id AND ib.is_latest = true
		WHERE w.grant_id = $1
		GROUP BY w.id, w.code, w.name, ib.fiscal_year
		ORDER BY w.code, ib.fiscal_year`
		args = []interface{}{grantID}
	}

	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("WBS effort summary: %w", err)
	}
	defer rows.Close()

	var summaries []models.WBSEffortSummary
	for rows.Next() {
		var s models.WBSEffortSummary
		var fy *int
		if err := rows.Scan(&s.WBSAreaID, &s.WBSCode, &s.WBSName, &fy, &s.EffortMonths, &s.Amount); err != nil {
			return nil, fmt.Errorf("scanning WBS effort: %w", err)
		}
		if fy != nil {
			s.FiscalYear = *fy
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// --- Subawards ---

func (q *Queries) ListSubawards(ctx context.Context, grantID string) ([]models.Subaward, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, grant_id, institution, pi_name, total_amount, salary_escalation_rate,
		       start_date::text, end_date::text, status, created_at, updated_at
		FROM subawards WHERE grant_id=$1 ORDER BY institution`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing subawards: %w", err)
	}
	defer rows.Close()

	var subs []models.Subaward
	for rows.Next() {
		var s models.Subaward
		if err := rows.Scan(&s.ID, &s.GrantID, &s.Institution, &s.PIName, &s.TotalAmount, &s.SalaryEscalationRate,
			&s.StartDate, &s.EndDate, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning subaward: %w", err)
		}
		subs = append(subs, s)
	}
	return subs, nil
}

func (q *Queries) GetSubaward(ctx context.Context, id string) (*models.Subaward, error) {
	var s models.Subaward
	err := q.pool.QueryRow(ctx, `
		SELECT id, grant_id, institution, pi_name, total_amount, salary_escalation_rate,
		       start_date::text, end_date::text, status, created_at, updated_at
		FROM subawards WHERE id=$1`, id).Scan(
		&s.ID, &s.GrantID, &s.Institution, &s.PIName, &s.TotalAmount, &s.SalaryEscalationRate,
		&s.StartDate, &s.EndDate, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting subaward: %w", err)
	}
	return &s, nil
}

func (q *Queries) CreateSubaward(ctx context.Context, s *models.Subaward) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO subawards (grant_id, institution, pi_name, total_amount, salary_escalation_rate,
		                       start_date, end_date, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		s.GrantID, s.Institution, s.PIName, s.TotalAmount, s.SalaryEscalationRate,
		s.StartDate, s.EndDate, s.Status,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (q *Queries) UpdateSubaward(ctx context.Context, s *models.Subaward) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE subawards SET institution=$2, pi_name=$3, total_amount=$4, salary_escalation_rate=$5,
		       start_date=$6, end_date=$7, status=$8, updated_at=now()
		WHERE id=$1`,
		s.ID, s.Institution, s.PIName, s.TotalAmount, s.SalaryEscalationRate,
		s.StartDate, s.EndDate, s.Status)
	return err
}

func (q *Queries) DeleteSubaward(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM subawards WHERE id=$1`, id)
	return err
}

// --- Invoices ---

func (q *Queries) ListInvoices(ctx context.Context, subawardID string) ([]models.Invoice, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, subaward_id, COALESCE(invoice_number, ''), invoice_date::text, amount,
		       period_start::text, period_end::text, status, COALESCE(notes, ''), created_at, updated_at
		FROM invoices WHERE subaward_id=$1 ORDER BY invoice_date DESC`, subawardID)
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()

	var invoices []models.Invoice
	for rows.Next() {
		var inv models.Invoice
		var ps, pe string
		if err := rows.Scan(&inv.ID, &inv.SubawardID, &inv.InvoiceNumber, &inv.InvoiceDate,
			&inv.Amount, &ps, &pe, &inv.Status, &inv.Notes, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning invoice: %w", err)
		}
		if ps != "" {
			inv.PeriodStart = &ps
		}
		if pe != "" {
			inv.PeriodEnd = &pe
		}
		invoices = append(invoices, inv)
	}
	return invoices, nil
}

func (q *Queries) CreateInvoice(ctx context.Context, inv *models.Invoice) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO invoices (subaward_id, invoice_number, invoice_date, amount, period_start, period_end, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		inv.SubawardID, inv.InvoiceNumber, inv.InvoiceDate, inv.Amount,
		inv.PeriodStart, inv.PeriodEnd, inv.Status, inv.Notes,
	).Scan(&inv.ID, &inv.CreatedAt, &inv.UpdatedAt)
}

func (q *Queries) UpdateInvoiceStatus(ctx context.Context, id, status string) error {
	_, err := q.pool.Exec(ctx, `UPDATE invoices SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

// --- Personnel ---

func (q *Queries) ListPersonnel(ctx context.Context, grantID string) ([]models.Personnel, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, grant_id, wbs_area_id, name, role, title, COALESCE(institution, ''),
		       annual_salary, funded_months,
		       start_date::text, end_date::text, created_at, updated_at
		FROM personnel WHERE grant_id=$1 ORDER BY name`, grantID)
	if err != nil {
		return nil, fmt.Errorf("listing personnel: %w", err)
	}
	defer rows.Close()

	var people []models.Personnel
	for rows.Next() {
		var p models.Personnel
		var sd, ed *string
		if err := rows.Scan(&p.ID, &p.GrantID, &p.WBSAreaID, &p.Name, &p.Role, &p.Title, &p.Institution,
			&p.AnnualSalary, &p.FundedMonths,
			&sd, &ed, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning personnel: %w", err)
		}
		p.StartDate = sd
		p.EndDate = ed
		people = append(people, p)
	}
	return people, nil
}

func (q *Queries) CreatePersonnel(ctx context.Context, p *models.Personnel) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO personnel (grant_id, wbs_area_id, name, role, title, institution, annual_salary, funded_months, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		p.GrantID, p.WBSAreaID, p.Name, p.Role, p.Title, p.Institution,
		p.AnnualSalary, p.FundedMonths, p.StartDate, p.EndDate,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (q *Queries) UpdatePersonnel(ctx context.Context, p *models.Personnel) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE personnel SET wbs_area_id=$2, name=$3, role=$4, title=$5, institution=$6,
		       annual_salary=$7, funded_months=$8,
		       start_date=$9, end_date=$10, updated_at=now()
		WHERE id=$1`,
		p.ID, p.WBSAreaID, p.Name, p.Role, p.Title, p.Institution,
		p.AnnualSalary, p.FundedMonths, p.StartDate, p.EndDate)
	return err
}

func (q *Queries) DeletePersonnel(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM personnel WHERE id=$1`, id)
	return err
}

func (q *Queries) GetPersonnel(ctx context.Context, id string) (*models.Personnel, error) {
	var p models.Personnel
	var sd, ed *string
	err := q.pool.QueryRow(ctx, `
		SELECT id, grant_id, wbs_area_id, name, role, title, COALESCE(institution, ''),
		       annual_salary, funded_months,
		       start_date::text, end_date::text, created_at, updated_at
		FROM personnel WHERE id=$1`, id).Scan(
		&p.ID, &p.GrantID, &p.WBSAreaID, &p.Name, &p.Role, &p.Title, &p.Institution,
		&p.AnnualSalary, &p.FundedMonths,
		&sd, &ed, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting personnel: %w", err)
	}
	p.StartDate = sd
	p.EndDate = ed
	return &p, nil
}

// ListPersonnelTitles returns distinct non-empty titles used by personnel in a grant.
func (q *Queries) ListPersonnelTitles(ctx context.Context, grantID string) ([]string, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT DISTINCT title FROM personnel WHERE grant_id=$1 AND title != '' ORDER BY title`, grantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		titles = append(titles, t)
	}
	return titles, nil
}

// PersonnelBudgetSummary returns the budgeted effort and salary for a person across
// all latest institution budgets, grouped by institution and fiscal year.
func (q *Queries) PersonnelBudgetSummary(ctx context.Context, personnelID string) ([]models.PersonnelBudgetEntry, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT
			COALESCE(
				CASE ib.entity_type
					WHEN 'grant' THEN (SELECT institution FROM grants WHERE id = ib.entity_id::uuid)
					WHEN 'subaward' THEN (SELECT institution FROM subawards WHERE id = ib.entity_id::uuid)
				END, ''
			) AS institution,
			ib.fiscal_year,
			COALESCE(SUM(bli.effort_months) FILTER (WHERE bli.line_type = 'personnel'), 0) AS effort_months,
			COALESCE(SUM(bli.amount) FILTER (WHERE bli.line_type = 'personnel'), 0) AS salary_amount,
			COALESCE(SUM(bli.amount) FILTER (WHERE bli.line_type = 'fringe'), 0) AS fringe_amount,
			COALESCE(
				CASE ib.entity_type
					WHEN 'grant' THEN (SELECT salary_escalation_rate FROM grants WHERE id = ib.entity_id::uuid)
					WHEN 'subaward' THEN (SELECT salary_escalation_rate FROM subawards WHERE id = ib.entity_id::uuid)
				END, 0
			) AS salary_escalation_rate
		FROM budget_line_items bli
		JOIN institution_budgets ib ON bli.institution_budget_id = ib.id
		WHERE bli.personnel_id = $1 AND ib.is_latest = true
		GROUP BY institution, ib.fiscal_year, salary_escalation_rate
		ORDER BY institution, ib.fiscal_year`, personnelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []models.PersonnelBudgetEntry
	for rows.Next() {
		var e models.PersonnelBudgetEntry
		if err := rows.Scan(&e.Institution, &e.FiscalYear, &e.EffortMonths, &e.SalaryAmount, &e.FringeAmount, &e.SalaryEscalationRate); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- Budget Line Items ---
// (Old grant-level budget_items table replaced by institution-scoped budget_line_items)

// --- Documents ---

func (q *Queries) CreateDocument(ctx context.Context, d *models.Document) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO documents (entity_type, entity_id, filename, content_type, s3_key, file_size, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		d.EntityType, d.EntityID, d.Filename, d.ContentType, d.S3Key, d.FileSize, d.UploadedBy,
	).Scan(&d.ID, &d.CreatedAt)
}

func (q *Queries) ListDocuments(ctx context.Context, entityType, entityID string) ([]models.Document, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, s3_key, file_size,
		       COALESCE(uploaded_by, ''), created_at
		FROM documents WHERE entity_type=$1 AND entity_id=$2 ORDER BY created_at DESC`,
		entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.EntityType, &d.EntityID, &d.Filename, &d.ContentType,
			&d.S3Key, &d.FileSize, &d.UploadedBy, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (q *Queries) GetDocument(ctx context.Context, id string) (*models.Document, error) {
	var d models.Document
	err := q.pool.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, s3_key, file_size,
		       COALESCE(uploaded_by, ''), created_at
		FROM documents WHERE id=$1`, id).Scan(
		&d.ID, &d.EntityType, &d.EntityID, &d.Filename, &d.ContentType,
		&d.S3Key, &d.FileSize, &d.UploadedBy, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting document: %w", err)
	}
	return &d, nil
}

// ListAllDocuments returns all documents in the database (used for backup).
func (q *Queries) ListAllDocuments(ctx context.Context) ([]models.Document, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, s3_key, file_size,
		       COALESCE(uploaded_by, ''), created_at
		FROM documents ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing all documents: %w", err)
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var d models.Document
		if err := rows.Scan(&d.ID, &d.EntityType, &d.EntityID, &d.Filename, &d.ContentType,
			&d.S3Key, &d.FileSize, &d.UploadedBy, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

// --- Statements of Work ---

func (q *Queries) ListStatementsOfWork(ctx context.Context, subawardID string) ([]models.StatementOfWork, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, subaward_id, fiscal_year, period_start::text, period_end::text,
		       budget_id, COALESCE(scope_text, ''), status, signed_doc_id, created_at, updated_at
		FROM statements_of_work WHERE subaward_id=$1 ORDER BY fiscal_year`, subawardID)
	if err != nil {
		return nil, fmt.Errorf("listing SOWs: %w", err)
	}
	defer rows.Close()

	var sows []models.StatementOfWork
	for rows.Next() {
		var s models.StatementOfWork
		if err := rows.Scan(&s.ID, &s.SubawardID, &s.FiscalYear, &s.PeriodStart, &s.PeriodEnd,
			&s.BudgetID, &s.ScopeText, &s.Status, &s.SignedDocID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning SOW: %w", err)
		}
		sows = append(sows, s)
	}
	return sows, nil
}

func (q *Queries) CreateStatementOfWork(ctx context.Context, s *models.StatementOfWork) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO statements_of_work (subaward_id, fiscal_year, period_start, period_end, budget_id, scope_text, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		s.SubawardID, s.FiscalYear, s.PeriodStart, s.PeriodEnd, s.BudgetID, s.ScopeText, s.Status,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (q *Queries) UpdateStatementOfWork(ctx context.Context, s *models.StatementOfWork) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE statements_of_work SET fiscal_year=$2, period_start=$3, period_end=$4,
		       budget_id=$5, scope_text=$6, status=$7, signed_doc_id=$8, updated_at=now()
		WHERE id=$1`,
		s.ID, s.FiscalYear, s.PeriodStart, s.PeriodEnd, s.BudgetID, s.ScopeText, s.Status, s.SignedDocID)
	return err
}

func (q *Queries) DeleteStatementOfWork(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM statements_of_work WHERE id = $1`, id)
	return err
}

// --- Overhead Rates ---

func (q *Queries) ListOverheadRates(ctx context.Context, entityType, entityID string) ([]models.OverheadRate, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, rate_name, rate,
		       COALESCE(description,''), created_at, updated_at
		FROM institution_overhead_rates
		WHERE entity_type=$1 AND entity_id=$2
		ORDER BY rate_name`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing overhead rates: %w", err)
	}
	defer rows.Close()

	var items []models.OverheadRate
	for rows.Next() {
		var r models.OverheadRate
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.RateName, &r.Rate,
			&r.Description, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning overhead rate: %w", err)
		}
		items = append(items, r)
	}
	return items, nil
}

func (q *Queries) CreateOverheadRate(ctx context.Context, r *models.OverheadRate) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO institution_overhead_rates (entity_type, entity_id, rate_name, rate, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		r.EntityType, r.EntityID, r.RateName, r.Rate, r.Description,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

func (q *Queries) UpdateOverheadRate(ctx context.Context, r *models.OverheadRate) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE institution_overhead_rates SET rate_name=$2, rate=$3, description=$4, updated_at=now()
		WHERE id=$1`,
		r.ID, r.RateName, r.Rate, r.Description)
	return err
}

func (q *Queries) DeleteOverheadRate(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM institution_overhead_rates WHERE id=$1`, id)
	return err
}

func (q *Queries) GetOverheadRate(ctx context.Context, id string) (*models.OverheadRate, error) {
	var r models.OverheadRate
	err := q.pool.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, rate_name, rate,
		       COALESCE(description,''), created_at, updated_at
		FROM institution_overhead_rates WHERE id=$1`, id).Scan(
		&r.ID, &r.EntityType, &r.EntityID, &r.RateName, &r.Rate,
		&r.Description, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting overhead rate: %w", err)
	}
	return &r, nil
}

// --- Budget Line Items ---

func (q *Queries) GetBudgetLineItem(ctx context.Context, id string) (*models.BudgetLineItem, error) {
	var b models.BudgetLineItem
	err := q.pool.QueryRow(ctx, `
		SELECT id, institution_budget_id, line_type, COALESCE(description,''),
		       personnel_id, effort_months, amount, overhead_rate_id,
		       COALESCE(notes,''), sort_order, is_manual_override, created_at, updated_at
		FROM budget_line_items WHERE id=$1`, id).Scan(
		&b.ID, &b.InstitutionBudgetID, &b.LineType, &b.Description,
		&b.PersonnelID, &b.EffortMonths, &b.Amount, &b.OverheadRateID,
		&b.Notes, &b.SortOrder, &b.IsManualOverride, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting budget line item: %w", err)
	}
	return &b, nil
}

func (q *Queries) ListBudgetLineItems(ctx context.Context, budgetID string) ([]models.BudgetLineItem, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, institution_budget_id, line_type, COALESCE(description,''),
		       personnel_id, effort_months, amount, overhead_rate_id,
		       COALESCE(notes,''), sort_order, is_manual_override, created_at, updated_at
		FROM budget_line_items
		WHERE institution_budget_id=$1
		ORDER BY sort_order, created_at`, budgetID)
	if err != nil {
		return nil, fmt.Errorf("listing budget line items: %w", err)
	}
	defer rows.Close()

	var items []models.BudgetLineItem
	for rows.Next() {
		var b models.BudgetLineItem
		if err := rows.Scan(&b.ID, &b.InstitutionBudgetID, &b.LineType, &b.Description,
			&b.PersonnelID, &b.EffortMonths, &b.Amount, &b.OverheadRateID,
			&b.Notes, &b.SortOrder, &b.IsManualOverride, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning budget line item: %w", err)
		}
		items = append(items, b)
	}
	return items, nil
}

func (q *Queries) CreateBudgetLineItem(ctx context.Context, b *models.BudgetLineItem) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO budget_line_items (institution_budget_id, line_type, description,
		       personnel_id, effort_months, amount, overhead_rate_id, notes, sort_order, is_manual_override)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at`,
		b.InstitutionBudgetID, b.LineType, b.Description,
		b.PersonnelID, b.EffortMonths, b.Amount, b.OverheadRateID, b.Notes, b.SortOrder, b.IsManualOverride,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

func (q *Queries) UpdateBudgetLineItem(ctx context.Context, b *models.BudgetLineItem) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE budget_line_items SET line_type=$2, description=$3,
		       personnel_id=$4, effort_months=$5, amount=$6, overhead_rate_id=$7,
		       notes=$8, sort_order=$9, is_manual_override=$10, updated_at=now()
		WHERE id=$1`,
		b.ID, b.LineType, b.Description,
		b.PersonnelID, b.EffortMonths, b.Amount, b.OverheadRateID, b.Notes, b.SortOrder, b.IsManualOverride)
	return err
}

func (q *Queries) DeleteBudgetLineItem(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM budget_line_items WHERE id=$1`, id)
	return err
}

// --- Budget Line Item WBS Allocations ---

func (q *Queries) ListLineItemWBS(ctx context.Context, lineItemID string) ([]models.BudgetLineItemWBS, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, line_item_id, wbs_area_id, allocation_percent
		FROM budget_line_item_wbs
		WHERE line_item_id=$1
		ORDER BY allocation_percent DESC`, lineItemID)
	if err != nil {
		return nil, fmt.Errorf("listing line item WBS: %w", err)
	}
	defer rows.Close()

	var items []models.BudgetLineItemWBS
	for rows.Next() {
		var w models.BudgetLineItemWBS
		if err := rows.Scan(&w.ID, &w.LineItemID, &w.WBSAreaID, &w.AllocationPercent); err != nil {
			return nil, fmt.Errorf("scanning line item WBS: %w", err)
		}
		items = append(items, w)
	}
	return items, nil
}

func (q *Queries) SetLineItemWBS(ctx context.Context, lineItemID string, allocations []models.BudgetLineItemWBS) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete existing allocations
	_, err = tx.Exec(ctx, `DELETE FROM budget_line_item_wbs WHERE line_item_id=$1`, lineItemID)
	if err != nil {
		return fmt.Errorf("deleting old WBS allocations: %w", err)
	}

	// Insert new ones
	for i := range allocations {
		a := &allocations[i]
		err = tx.QueryRow(ctx, `
			INSERT INTO budget_line_item_wbs (line_item_id, wbs_area_id, allocation_percent)
			VALUES ($1, $2, $3) RETURNING id`,
			lineItemID, a.WBSAreaID, a.AllocationPercent,
		).Scan(&a.ID)
		if err != nil {
			return fmt.Errorf("inserting WBS allocation: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// --- Institution Fringe Rates ---

func (q *Queries) ListFringeRates(ctx context.Context, entityType, entityID string) ([]models.InstitutionFringeRate, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, fiscal_year, rate_name, rate, created_at, updated_at
		FROM institution_fringe_rates
		WHERE entity_type=$1 AND entity_id=$2
		ORDER BY fiscal_year, rate_name`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing fringe rates: %w", err)
	}
	defer rows.Close()

	var items []models.InstitutionFringeRate
	for rows.Next() {
		var r models.InstitutionFringeRate
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.FiscalYear, &r.RateName, &r.Rate, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning fringe rate: %w", err)
		}
		items = append(items, r)
	}
	return items, nil
}

func (q *Queries) UpsertFringeRate(ctx context.Context, r *models.InstitutionFringeRate) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO institution_fringe_rates (entity_type, entity_id, fiscal_year, rate_name, rate)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (entity_type, entity_id, fiscal_year, rate_name) DO UPDATE SET
			rate=EXCLUDED.rate, updated_at=now()
		RETURNING id, created_at, updated_at`,
		r.EntityType, r.EntityID, r.FiscalYear, r.RateName, r.Rate,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

func (q *Queries) DeleteFringeRate(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM institution_fringe_rates WHERE id=$1`, id)
	return err
}

// --- Institution Budgets (versioned) ---

func (q *Queries) ListInstitutionBudgets(ctx context.Context, entityType, entityID string) ([]models.InstitutionBudget, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, fiscal_year, version, is_latest, status, budget, COALESCE(notes,''), created_at
		FROM institution_budgets
		WHERE entity_type=$1 AND entity_id=$2
		ORDER BY fiscal_year, version DESC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing institution budgets: %w", err)
	}
	defer rows.Close()

	var items []models.InstitutionBudget
	for rows.Next() {
		var b models.InstitutionBudget
		if err := rows.Scan(&b.ID, &b.EntityType, &b.EntityID, &b.FiscalYear, &b.Version, &b.IsLatest, &b.Status, &b.Budget, &b.Notes, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning institution budget: %w", err)
		}
		items = append(items, b)
	}
	return items, nil
}

// ListLatestInstitutionBudgets returns only the latest version of each year's budget.
func (q *Queries) ListLatestInstitutionBudgets(ctx context.Context, entityType, entityID string) ([]models.InstitutionBudget, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, fiscal_year, version, is_latest, status, budget, COALESCE(notes,''), created_at
		FROM institution_budgets
		WHERE entity_type=$1 AND entity_id=$2 AND is_latest=true
		ORDER BY fiscal_year`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing latest institution budgets: %w", err)
	}
	defer rows.Close()

	var items []models.InstitutionBudget
	for rows.Next() {
		var b models.InstitutionBudget
		if err := rows.Scan(&b.ID, &b.EntityType, &b.EntityID, &b.FiscalYear, &b.Version, &b.IsLatest, &b.Status, &b.Budget, &b.Notes, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning institution budget: %w", err)
		}
		items = append(items, b)
	}
	return items, nil
}

// CreateInstitutionBudget adds a new version. It marks previous versions for that year as not-latest.
func (q *Queries) CreateInstitutionBudget(ctx context.Context, b *models.InstitutionBudget) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get next version number
	var maxVersion int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0)
		FROM institution_budgets
		WHERE entity_type=$1 AND entity_id=$2 AND fiscal_year=$3`,
		b.EntityType, b.EntityID, b.FiscalYear).Scan(&maxVersion)
	if err != nil {
		return fmt.Errorf("getting max version: %w", err)
	}

	// Mark old versions as not-latest
	_, err = tx.Exec(ctx, `
		UPDATE institution_budgets SET is_latest=false
		WHERE entity_type=$1 AND entity_id=$2 AND fiscal_year=$3`,
		b.EntityType, b.EntityID, b.FiscalYear)
	if err != nil {
		return fmt.Errorf("marking old versions: %w", err)
	}

	b.Version = maxVersion + 1
	b.IsLatest = true
	if b.Status == "" {
		b.Status = "draft"
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO institution_budgets (entity_type, entity_id, fiscal_year, version, is_latest, status, budget, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
		b.EntityType, b.EntityID, b.FiscalYear, b.Version, b.IsLatest, b.Status, b.Budget, b.Notes,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("inserting institution budget: %w", err)
	}

	return tx.Commit(ctx)
}

// ValidateBudgetForFinalize checks that a budget is ready to finalize.
// Returns (blocking errors, non-blocking warnings). Errors prevent finalization;
// warnings are advisory and do not block (e.g. a salaried person with no fringe line,
// which is legitimate now that fringe is opt-in).
func (q *Queries) ValidateBudgetForFinalize(ctx context.Context, budgetID string) ([]string, []string, error) {
	var errors []string
	var warnings []string

	// Build a personnel_id -> display name map so validation messages are readable.
	nameByID := make(map[string]string)
	nameRows, err := q.pool.Query(ctx, `
		SELECT DISTINCT p.id, p.name
		FROM budget_line_items bli
		JOIN personnel p ON p.id = bli.personnel_id
		WHERE bli.institution_budget_id=$1 AND bli.personnel_id IS NOT NULL`, budgetID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading personnel names: %w", err)
	}
	for nameRows.Next() {
		var id, name string
		if err := nameRows.Scan(&id, &name); err != nil {
			nameRows.Close()
			return nil, nil, err
		}
		nameByID[id] = name
	}
	nameRows.Close()
	nameOf := func(pid string) string {
		if n := nameByID[pid]; n != "" {
			return n
		}
		return pid
	}

	// 1. Check for personnel lines without matching fringe lines (and vice versa)
	// Get distinct personnel_ids with salary lines
	salaryRows, err := q.pool.Query(ctx, `
		SELECT DISTINCT personnel_id FROM budget_line_items
		WHERE institution_budget_id=$1 AND line_type='personnel' AND personnel_id IS NOT NULL`, budgetID)
	if err != nil {
		return nil, nil, fmt.Errorf("checking salary lines: %w", err)
	}
	var salaryPersonIDs []string
	for salaryRows.Next() {
		var pid string
		if err := salaryRows.Scan(&pid); err != nil {
			salaryRows.Close()
			return nil, nil, err
		}
		salaryPersonIDs = append(salaryPersonIDs, pid)
	}
	salaryRows.Close()

	fringeRows, err := q.pool.Query(ctx, `
		SELECT DISTINCT personnel_id FROM budget_line_items
		WHERE institution_budget_id=$1 AND line_type='fringe' AND personnel_id IS NOT NULL`, budgetID)
	if err != nil {
		return nil, nil, fmt.Errorf("checking fringe lines: %w", err)
	}
	fringeSet := make(map[string]bool)
	for fringeRows.Next() {
		var pid string
		if err := fringeRows.Scan(&pid); err != nil {
			fringeRows.Close()
			return nil, nil, err
		}
		fringeSet[pid] = true
	}
	fringeRows.Close()

	salarySet := make(map[string]bool)
	for _, pid := range salaryPersonIDs {
		salarySet[pid] = true
		if !fringeSet[pid] {
			// Non-blocking: fringe is opt-in, so a salaried person may legitimately have none.
			warnings = append(warnings, fmt.Sprintf("%s has salary line(s) but no fringe line(s)", nameOf(pid)))
		}
	}
	for pid := range fringeSet {
		if !salarySet[pid] {
			errors = append(errors, fmt.Sprintf("%s has fringe line(s) but no salary line(s)", nameOf(pid)))
		}
	}

	// 2. Check all line items have complete WBS allocations (sum to 100%)
	wbsRows, err := q.pool.Query(ctx, `
		SELECT li.id, li.description,
		       COALESCE(SUM(w.allocation_percent), 0) as total_pct
		FROM budget_line_items li
		LEFT JOIN budget_line_item_wbs w ON w.line_item_id = li.id
		WHERE li.institution_budget_id=$1
		GROUP BY li.id, li.description`, budgetID)
	if err != nil {
		return nil, nil, fmt.Errorf("checking WBS allocations: %w", err)
	}
	for wbsRows.Next() {
		var id, desc string
		var totalPct float64
		if err := wbsRows.Scan(&id, &desc, &totalPct); err != nil {
			wbsRows.Close()
			return nil, nil, err
		}
		if totalPct < 99.99 || totalPct > 100.01 {
			label := desc
			if label == "" {
				label = id[:8]
			}
			errors = append(errors, fmt.Sprintf("Line item \"%s\" WBS allocation is %.1f%% (must be 100%%)", label, totalPct))
		}
	}
	wbsRows.Close()

	return errors, warnings, nil
}

// FinalizeBudget marks a budget as final (no longer draft).
func (q *Queries) FinalizeBudget(ctx context.Context, budgetID string) error {
	_, err := q.pool.Exec(ctx, `UPDATE institution_budgets SET status='final' WHERE id=$1`, budgetID)
	return err
}

// DeleteBudget deletes a budget and its line items (CASCADE). Only draft budgets should be deleted.
func (q *Queries) DeleteBudget(ctx context.Context, budgetID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM institution_budgets WHERE id=$1`, budgetID)
	return err
}

// DuplicateBudget copies a finalized budget to create a new draft version with all its line items and WBS allocations.
func (q *Queries) DuplicateBudget(ctx context.Context, sourceBudgetID string) (*models.InstitutionBudget, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Get the source budget
	var src models.InstitutionBudget
	err = tx.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, fiscal_year, version, is_latest, status, budget, COALESCE(notes,''), created_at
		FROM institution_budgets WHERE id=$1`, sourceBudgetID,
	).Scan(&src.ID, &src.EntityType, &src.EntityID, &src.FiscalYear, &src.Version, &src.IsLatest, &src.Status, &src.Budget, &src.Notes, &src.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting source budget: %w", err)
	}

	// Get next version
	var maxVersion int
	err = tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0)
		FROM institution_budgets
		WHERE entity_type=$1 AND entity_id=$2 AND fiscal_year=$3`,
		src.EntityType, src.EntityID, src.FiscalYear).Scan(&maxVersion)
	if err != nil {
		return nil, fmt.Errorf("getting max version: %w", err)
	}

	// Mark old versions as not-latest
	_, err = tx.Exec(ctx, `
		UPDATE institution_budgets SET is_latest=false
		WHERE entity_type=$1 AND entity_id=$2 AND fiscal_year=$3`,
		src.EntityType, src.EntityID, src.FiscalYear)
	if err != nil {
		return nil, fmt.Errorf("marking old versions: %w", err)
	}

	// Create new budget version
	newBudget := models.InstitutionBudget{
		EntityType: src.EntityType,
		EntityID:   src.EntityID,
		FiscalYear: src.FiscalYear,
		Version:    maxVersion + 1,
		IsLatest:   true,
		Status:     "draft",
		Budget:     src.Budget,
		Notes:      fmt.Sprintf("Duplicated from v%d", src.Version),
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO institution_budgets (entity_type, entity_id, fiscal_year, version, is_latest, status, budget, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`,
		newBudget.EntityType, newBudget.EntityID, newBudget.FiscalYear,
		newBudget.Version, newBudget.IsLatest, newBudget.Status, newBudget.Budget, newBudget.Notes,
	).Scan(&newBudget.ID, &newBudget.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inserting new budget: %w", err)
	}

	// Copy line items
	rows, err := tx.Query(ctx, `
		SELECT id, line_type, description, personnel_id, effort_months, amount, overhead_rate_id, notes, sort_order
		FROM budget_line_items WHERE institution_budget_id=$1 ORDER BY sort_order`, sourceBudgetID)
	if err != nil {
		return nil, fmt.Errorf("reading source line items: %w", err)
	}
	defer rows.Close()

	type lineMapping struct{ oldID, newID string }
	var mappings []lineMapping

	for rows.Next() {
		var oldID, lineType, desc, notes string
		var personnelID, overheadRateID *string
		var effortMonths, amount float64
		var sortOrder int
		if err := rows.Scan(&oldID, &lineType, &desc, &personnelID, &effortMonths, &amount, &overheadRateID, &notes, &sortOrder); err != nil {
			return nil, fmt.Errorf("scanning source line item: %w", err)
		}
		var newID string
		err = tx.QueryRow(ctx, `
			INSERT INTO budget_line_items (institution_budget_id, line_type, description, personnel_id, effort_months, amount, overhead_rate_id, notes, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id`,
			newBudget.ID, lineType, desc, personnelID, effortMonths, amount, overheadRateID, notes, sortOrder,
		).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("copying line item: %w", err)
		}
		mappings = append(mappings, lineMapping{oldID: oldID, newID: newID})
	}
	rows.Close()

	// Copy WBS allocations for each line item
	for _, m := range mappings {
		_, err = tx.Exec(ctx, `
			INSERT INTO budget_line_item_wbs (line_item_id, wbs_area_id, allocation_percent)
			SELECT $1, wbs_area_id, allocation_percent
			FROM budget_line_item_wbs WHERE line_item_id=$2`,
			m.newID, m.oldID)
		if err != nil {
			return nil, fmt.Errorf("copying WBS allocations: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}
	return &newBudget, nil
}

// --- App Config ---

func (q *Queries) GetAppConfig(ctx context.Context, key string) (string, error) {
	var val string
	err := q.pool.QueryRow(ctx, `SELECT value FROM app_config WHERE key=$1`, key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (q *Queries) SetAppConfig(ctx context.Context, key, value string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO app_config (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value=$2, updated_at=NOW()`, key, value)
	return err
}

func (q *Queries) ListAppConfig(ctx context.Context) ([]models.AppConfig, error) {
	rows, err := q.pool.Query(ctx, `SELECT key, value, updated_at FROM app_config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []models.AppConfig
	for rows.Next() {
		var c models.AppConfig
		if err := rows.Scan(&c.Key, &c.Value, &c.UpdatedAt); err != nil {
			return nil, err
		}
		configs = append(configs, c)
	}
	return configs, nil
}

// --- Users ---

func (q *Queries) CreateUser(ctx context.Context, u *models.User) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO users (display_name, status)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`,
		u.DisplayName, u.Status).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (q *Queries) GetUser(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := q.pool.QueryRow(ctx, `
		SELECT id, display_name, status, last_login, created_at, updated_at
		FROM users WHERE id=$1`, id).Scan(
		&u.ID, &u.DisplayName, &u.Status, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (q *Queries) UpdateUser(ctx context.Context, u *models.User) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE users SET display_name=$2, status=$3, updated_at=NOW()
		WHERE id=$1`, u.ID, u.DisplayName, u.Status)
	return err
}

func (q *Queries) UpdateUserLastLogin(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `UPDATE users SET last_login=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (q *Queries) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, display_name, status, last_login, created_at, updated_at
		FROM users ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.DisplayName, &u.Status, &u.LastLogin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (q *Queries) DeleteUser(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	return err
}

// --- User Roles ---

func (q *Queries) ListUserRoles(ctx context.Context, userID string) ([]models.UserRole, error) {
	rows, err := q.pool.Query(ctx, `SELECT id, user_id, role FROM user_roles WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []models.UserRole
	for rows.Next() {
		var r models.UserRole
		if err := rows.Scan(&r.ID, &r.UserID, &r.Role); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, nil
}

func (q *Queries) AddUserRole(ctx context.Context, userID, role string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role) VALUES ($1, $2)
		ON CONFLICT (user_id, role) DO NOTHING`, userID, role)
	return err
}

func (q *Queries) RemoveUserRole(ctx context.Context, userID, role string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1 AND role=$2`, userID, role)
	return err
}

// --- User Identities ---

func (q *Queries) ListUserIdentities(ctx context.Context, userID string) ([]models.UserIdentity, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, user_id, issuer, subject,
		       COALESCE(email,''), COALESCE(eppn,''), COALESCE(oidc,''),
		       COALESCE(cilogon_id,''), COALESCE(idp_name,''), COALESCE(display_name,''), created_at
		FROM user_identities WHERE user_id=$1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []models.UserIdentity
	for rows.Next() {
		var i models.UserIdentity
		if err := rows.Scan(&i.ID, &i.UserID, &i.Issuer, &i.Subject, &i.Email, &i.EPPN, &i.OIDC, &i.CILogonID, &i.IdPName, &i.DisplayName, &i.CreatedAt); err != nil {
			return nil, err
		}
		ids = append(ids, i)
	}
	return ids, nil
}

func (q *Queries) FindIdentity(ctx context.Context, issuer, subject string) (*models.UserIdentity, error) {
	var i models.UserIdentity
	err := q.pool.QueryRow(ctx, `
		SELECT id, user_id, issuer, subject,
		       COALESCE(email,''), COALESCE(eppn,''), COALESCE(oidc,''),
		       COALESCE(cilogon_id,''), COALESCE(idp_name,''), COALESCE(display_name,''), created_at
		FROM user_identities WHERE issuer=$1 AND subject=$2`, issuer, subject).Scan(
		&i.ID, &i.UserID, &i.Issuer, &i.Subject, &i.Email, &i.EPPN, &i.OIDC, &i.CILogonID, &i.IdPName, &i.DisplayName, &i.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (q *Queries) CreateIdentity(ctx context.Context, id *models.UserIdentity) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO user_identities (user_id, issuer, subject, email, eppn, oidc, cilogon_id, idp_name, display_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, created_at`,
		id.UserID, id.Issuer, id.Subject, id.Email, id.EPPN, id.OIDC, id.CILogonID, id.IdPName, id.DisplayName).Scan(&id.ID, &id.CreatedAt)
}

func (q *Queries) DeleteIdentity(ctx context.Context, identityID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM user_identities WHERE id=$1`, identityID)
	return err
}

// --- Sessions ---

func (q *Queries) CreateSession(ctx context.Context, s *models.Session) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO sessions (user_id, role, expires_at, token_hash)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		s.UserID, s.Role, s.ExpiresAt, s.TokenHash).Scan(&s.ID, &s.CreatedAt)
}

func (q *Queries) GetSession(ctx context.Context, tokenHash []byte) (*models.Session, error) {
	var s models.Session
	err := q.pool.QueryRow(ctx, `
		SELECT id, user_id, role, expires_at, created_at
		FROM sessions WHERE token_hash=$1 AND expires_at > NOW()`, tokenHash).Scan(
		&s.ID, &s.UserID, &s.Role, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (q *Queries) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash)
	return err
}

func (q *Queries) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}

// --- Invites ---

func (q *Queries) CreateInvite(ctx context.Context, inv *models.Invite) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO invites (token, token_hash, user_id, expires_at)
		VALUES ('', $1, $2, $3) RETURNING id, created_at`,
		inv.TokenHash, inv.UserID, inv.ExpiresAt).Scan(&inv.ID, &inv.CreatedAt)
}

func (q *Queries) GetInviteByToken(ctx context.Context, tokenHash []byte) (*models.Invite, error) {
	var inv models.Invite
	err := q.pool.QueryRow(ctx, `
		SELECT id, user_id, used, expires_at, created_at
		FROM invites WHERE token_hash=$1`, tokenHash).Scan(
		&inv.ID, &inv.UserID, &inv.Used, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (q *Queries) MarkInviteUsed(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `UPDATE invites SET used=TRUE WHERE id=$1`, id)
	return err
}

func (q *Queries) ListInvites(ctx context.Context, userID string) ([]models.Invite, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, user_id, used, expires_at, created_at
		FROM invites WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var invites []models.Invite
	for rows.Next() {
		var inv models.Invite
		if err := rows.Scan(&inv.ID, &inv.UserID, &inv.Used, &inv.ExpiresAt, &inv.CreatedAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, nil
}

func (q *Queries) DeleteInvite(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM invites WHERE id=$1`, id)
	return err
}

// --- Budget Documents ---

func (q *Queries) CreateBudgetDocument(ctx context.Context, d *models.BudgetDocument) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO budget_documents
			(entity_type, entity_id, budget_id, doc_type, filename, content_type,
			 s3_key, file_size, encrypted_dek, dek_nonce, uploaded_by, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at`,
		d.EntityType, d.EntityID, d.BudgetID, d.DocType, d.Filename, d.ContentType,
		d.S3Key, d.FileSize, d.EncryptedDEK, d.DEKNonce, d.UploadedBy, d.Notes,
	).Scan(&d.ID, &d.CreatedAt)
}

func (q *Queries) ListBudgetDocuments(ctx context.Context, entityType, entityID string, includeDeleted bool) ([]models.BudgetDocument, error) {
	query := `
		SELECT bd.id, bd.entity_type, bd.entity_id, bd.budget_id, bd.doc_type,
		       bd.filename, bd.content_type, bd.s3_key, bd.file_size,
		       bd.encrypted_dek, bd.dek_nonce,
		       bd.uploaded_by, COALESCE(u.display_name, ''), bd.notes,
		       bd.created_at, bd.deleted_at, bd.deleted_by
		FROM budget_documents bd
		LEFT JOIN users u ON u.id = bd.uploaded_by
		WHERE bd.entity_type=$1 AND bd.entity_id=$2`
	if !includeDeleted {
		query += ` AND bd.deleted_at IS NULL`
	}
	query += ` ORDER BY bd.created_at DESC`
	rows, err := q.pool.Query(ctx, query, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("listing budget documents: %w", err)
	}
	defer rows.Close()

	var docs []models.BudgetDocument
	for rows.Next() {
		var d models.BudgetDocument
		if err := rows.Scan(
			&d.ID, &d.EntityType, &d.EntityID, &d.BudgetID, &d.DocType,
			&d.Filename, &d.ContentType, &d.S3Key, &d.FileSize,
			&d.EncryptedDEK, &d.DEKNonce,
			&d.UploadedBy, &d.UploadedName, &d.Notes,
			&d.CreatedAt, &d.DeletedAt, &d.DeletedBy,
		); err != nil {
			return nil, fmt.Errorf("scanning budget document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (q *Queries) ListAllBudgetDocuments(ctx context.Context) ([]models.BudgetDocument, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT bd.id, bd.entity_type, bd.entity_id, bd.budget_id, bd.doc_type,
		       bd.filename, bd.content_type, bd.s3_key, bd.file_size,
		       bd.encrypted_dek, bd.dek_nonce,
		       bd.uploaded_by, COALESCE(u.display_name, ''), bd.notes,
		       bd.created_at, bd.deleted_at, bd.deleted_by
		FROM budget_documents bd
		LEFT JOIN users u ON u.id = bd.uploaded_by
		WHERE bd.deleted_at IS NULL
		ORDER BY bd.created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing all budget documents: %w", err)
	}
	defer rows.Close()

	var docs []models.BudgetDocument
	for rows.Next() {
		var d models.BudgetDocument
		if err := rows.Scan(
			&d.ID, &d.EntityType, &d.EntityID, &d.BudgetID, &d.DocType,
			&d.Filename, &d.ContentType, &d.S3Key, &d.FileSize,
			&d.EncryptedDEK, &d.DEKNonce,
			&d.UploadedBy, &d.UploadedName, &d.Notes,
			&d.CreatedAt, &d.DeletedAt, &d.DeletedBy,
		); err != nil {
			return nil, fmt.Errorf("scanning budget document: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (q *Queries) GetBudgetDocument(ctx context.Context, id string) (*models.BudgetDocument, error) {
	var d models.BudgetDocument
	err := q.pool.QueryRow(ctx, `
		SELECT bd.id, bd.entity_type, bd.entity_id, bd.budget_id, bd.doc_type,
		       bd.filename, bd.content_type, bd.s3_key, bd.file_size,
		       bd.encrypted_dek, bd.dek_nonce,
		       bd.uploaded_by, COALESCE(u.display_name, ''), bd.notes,
		       bd.created_at, bd.deleted_at, bd.deleted_by
		FROM budget_documents bd
		LEFT JOIN users u ON u.id = bd.uploaded_by
		WHERE bd.id=$1`, id).Scan(
		&d.ID, &d.EntityType, &d.EntityID, &d.BudgetID, &d.DocType,
		&d.Filename, &d.ContentType, &d.S3Key, &d.FileSize,
		&d.EncryptedDEK, &d.DEKNonce,
		&d.UploadedBy, &d.UploadedName, &d.Notes,
		&d.CreatedAt, &d.DeletedAt, &d.DeletedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("getting budget document: %w", err)
	}
	return &d, nil
}

func (q *Queries) SoftDeleteBudgetDocument(ctx context.Context, id string, deletedByUserID string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE budget_documents SET deleted_at=now(), deleted_by=$2 WHERE id=$1`,
		id, deletedByUserID)
	return err
}

// ---- API Keys ----

func (q *Queries) CreateAPIKey(ctx context.Context, k *models.APIKey) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO api_keys (name, key_hash, key_prefix, roles, created_by, idle_timeout_s, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		k.Name, k.KeyHash, k.KeyPrefix, k.Roles,
		k.CreatedBy, k.IdleTimeoutS, k.ExpiresAt,
	).Scan(&k.ID, &k.CreatedAt)
}

func (q *Queries) ListAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT k.id, k.name, k.key_prefix, k.roles,
		       k.created_by, u.display_name, k.created_at,
		       k.last_used_at, k.idle_timeout_s, k.expires_at, k.revoked_at
		FROM api_keys k
		LEFT JOIN users u ON u.id = k.created_by
		ORDER BY k.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing api keys: %w", err)
	}
	defer rows.Close()
	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Roles,
			&k.CreatedBy, &k.CreatedByName, &k.CreatedAt,
			&k.LastUsedAt, &k.IdleTimeoutS, &k.ExpiresAt, &k.RevokedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (q *Queries) GetAPIKeyByID(ctx context.Context, id string) (*models.APIKey, error) {
	var k models.APIKey
	err := q.pool.QueryRow(ctx, `
		SELECT k.id, k.name, k.key_hash, k.key_prefix, k.roles,
		       k.created_by, COALESCE(u.display_name, ''), k.created_at,
		       k.last_used_at, k.idle_timeout_s, k.expires_at, k.revoked_at
		FROM api_keys k
		LEFT JOIN users u ON u.id = k.created_by
		WHERE k.id = $1`, id,
	).Scan(&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Roles,
		&k.CreatedBy, &k.CreatedByName, &k.CreatedAt,
		&k.LastUsedAt, &k.IdleTimeoutS, &k.ExpiresAt, &k.RevokedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting api key: %w", err)
	}
	return &k, nil
}

// ListActiveAPIKeyHashes returns all non-revoked, non-expired keys with their hashes
// for authentication lookups.  Caller is responsible for bcrypt comparison.
func (q *Queries) ListActiveAPIKeyHashes(ctx context.Context) ([]models.APIKey, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, name, key_hash, key_prefix, roles,
		       created_by, created_at, last_used_at, idle_timeout_s, expires_at
		FROM api_keys
		WHERE revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())`)
	if err != nil {
		return nil, fmt.Errorf("listing active api key hashes: %w", err)
	}
	defer rows.Close()
	var keys []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.Roles,
			&k.CreatedBy, &k.CreatedAt, &k.LastUsedAt, &k.IdleTimeoutS, &k.ExpiresAt,
		); err != nil {
			return nil, fmt.Errorf("scanning active api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// TouchAPIKeyLastUsed updates the last_used_at timestamp.  The caller should
// debounce so this isn't called on every single request.
func (q *Queries) TouchAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `UPDATE api_keys SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (q *Queries) RevokeAPIKey(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `UPDATE api_keys SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (q *Queries) DeleteAPIKey(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM api_keys WHERE id = $1`, id)
	return err
}

// --- Backups ---

func (q *Queries) CreateBackup(ctx context.Context, b *models.Backup) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO backups (filename, s3_key, s3_bucket, size_bytes, status, status_detail, initiated_by, encrypted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, started_at`,
		b.Filename, b.S3Key, b.S3Bucket, b.SizeBytes, b.Status, b.StatusDetail, b.InitiatedBy, b.Encrypted,
	).Scan(&b.ID, &b.CreatedAt, &b.StartedAt)
}

func (q *Queries) CompleteBackup(ctx context.Context, id string, sizeBytes int64, checksum string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE backups SET status='completed', status_detail='', size_bytes=$2, checksum=$3, completed_at=NOW()
		WHERE id=$1`, id, sizeBytes, checksum)
	return err
}

func (q *Queries) UpdateBackupProgress(ctx context.Context, id string, detail string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE backups SET status_detail=$2 WHERE id=$1`, id, detail)
	return err
}

func (q *Queries) FailBackup(ctx context.Context, id string, errMsg string) error {
	_, err := q.pool.Exec(ctx, `
		UPDATE backups SET status='failed', error_msg=$2, completed_at=NOW()
		WHERE id=$1`, id, errMsg)
	return err
}

// FailStaleBackups marks any backups still in 'running' state as failed.
// Call this at startup to clean up after a crash or restart.
func (q *Queries) FailStaleBackups(ctx context.Context) (int64, error) {
	tag, err := q.pool.Exec(ctx, `
		UPDATE backups SET status='failed', error_msg='server restarted during backup', completed_at=NOW()
		WHERE status='running'`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (q *Queries) ListBackups(ctx context.Context) ([]models.Backup, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, filename, s3_key, s3_bucket, size_bytes, status,
		       COALESCE(status_detail,''), COALESCE(error_msg,''), initiated_by, encrypted,
		       COALESCE(checksum,''), started_at, completed_at, created_at
		FROM backups ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing backups: %w", err)
	}
	defer rows.Close()

	var backups []models.Backup
	for rows.Next() {
		var b models.Backup
		if err := rows.Scan(&b.ID, &b.Filename, &b.S3Key, &b.S3Bucket, &b.SizeBytes,
			&b.Status, &b.StatusDetail, &b.ErrorMsg, &b.InitiatedBy, &b.Encrypted, &b.Checksum,
			&b.StartedAt, &b.CompletedAt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, nil
}

func (q *Queries) GetBackup(ctx context.Context, id string) (*models.Backup, error) {
	var b models.Backup
	err := q.pool.QueryRow(ctx, `
		SELECT id, filename, s3_key, s3_bucket, size_bytes, status,
		       COALESCE(status_detail,''), COALESCE(error_msg,''), initiated_by, encrypted,
		       COALESCE(checksum,''), started_at, completed_at, created_at
		FROM backups WHERE id=$1`, id).Scan(
		&b.ID, &b.Filename, &b.S3Key, &b.S3Bucket, &b.SizeBytes,
		&b.Status, &b.StatusDetail, &b.ErrorMsg, &b.InitiatedBy, &b.Encrypted, &b.Checksum,
		&b.StartedAt, &b.CompletedAt, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting backup: %w", err)
	}
	return &b, nil
}

func (q *Queries) DeleteBackupRecord(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM backups WHERE id = $1`, id)
	return err
}

// ListFailedBackups returns all backups with status 'failed'.
func (q *Queries) ListFailedBackups(ctx context.Context) ([]models.Backup, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, filename, s3_key, s3_bucket, size_bytes, status,
		       COALESCE(status_detail,''), COALESCE(error_msg,''), initiated_by, encrypted,
		       COALESCE(checksum,''), started_at, completed_at, created_at
		FROM backups WHERE status='failed'
		ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var backups []models.Backup
	for rows.Next() {
		var b models.Backup
		if err := rows.Scan(&b.ID, &b.Filename, &b.S3Key, &b.S3Bucket, &b.SizeBytes,
			&b.Status, &b.StatusDetail, &b.ErrorMsg, &b.InitiatedBy, &b.Encrypted, &b.Checksum,
			&b.StartedAt, &b.CompletedAt, &b.CreatedAt); err != nil {
			return nil, err
		}
		backups = append(backups, b)
	}
	return backups, nil
}

// --- Object Hashes ---

func (q *Queries) UpsertObjectHash(ctx context.Context, s3Key, sha256Hash string, sizeBytes int64) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO object_hashes (s3_key, sha256_hash, size_bytes, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (s3_key) DO UPDATE SET sha256_hash=$2, size_bytes=$3, updated_at=NOW()`,
		s3Key, sha256Hash, sizeBytes)
	return err
}

func (q *Queries) GetObjectHash(ctx context.Context, s3Key string) (*models.ObjectHash, error) {
	var h models.ObjectHash
	err := q.pool.QueryRow(ctx, `
		SELECT id, s3_key, sha256_hash, size_bytes, updated_at
		FROM object_hashes WHERE s3_key=$1`, s3Key).Scan(
		&h.ID, &h.S3Key, &h.SHA256, &h.SizeBytes, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (q *Queries) DeleteObjectHash(ctx context.Context, s3Key string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM object_hashes WHERE s3_key = $1`, s3Key)
	return err
}

// --- User Institution Access (subaward_admin) ---

func (q *Queries) ListUserInstitutions(ctx context.Context, userID string) ([]models.UserInstitutionAccess, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, user_id, institution, created_at
		FROM user_institution_access WHERE user_id=$1 ORDER BY institution`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.UserInstitutionAccess
	for rows.Next() {
		var a models.UserInstitutionAccess
		if err := rows.Scan(&a.ID, &a.UserID, &a.Institution, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, nil
}

func (q *Queries) AddUserInstitution(ctx context.Context, userID, institution string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO user_institution_access (user_id, institution)
		VALUES ($1, $2) ON CONFLICT (user_id, institution) DO NOTHING`, userID, institution)
	return err
}

func (q *Queries) RemoveUserInstitution(ctx context.Context, userID, institution string) error {
	_, err := q.pool.Exec(ctx, `
		DELETE FROM user_institution_access WHERE user_id=$1 AND institution=$2`, userID, institution)
	return err
}

func (q *Queries) ListUserInstitutionNames(ctx context.Context, userID string) ([]string, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT institution FROM user_institution_access WHERE user_id=$1 ORDER BY institution`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// --- SOW Config ---

func (q *Queries) GetSOWConfig(ctx context.Context, grantID string) (*models.SOWConfig, error) {
	var c models.SOWConfig
	err := q.pool.QueryRow(ctx, `
		SELECT id, grant_id, header_title, header_subtitle, project_name,
		       intro_template, costs_template, concurrence_signers::text,
		       created_at, updated_at
		FROM sow_configs WHERE grant_id = $1`, grantID).Scan(
		&c.ID, &c.GrantID, &c.HeaderTitle, &c.HeaderSubtitle, &c.ProjectName,
		&c.IntroTemplate, &c.CostsTemplate, &c.ConcurrenceSigners,
		&c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting SOW config: %w", err)
	}
	return &c, nil
}

func (q *Queries) UpsertSOWConfig(ctx context.Context, c *models.SOWConfig) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO sow_configs (grant_id, header_title, header_subtitle, project_name,
		                         intro_template, costs_template, concurrence_signers)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
		ON CONFLICT (grant_id) DO UPDATE SET
		    header_title = EXCLUDED.header_title,
		    header_subtitle = EXCLUDED.header_subtitle,
		    project_name = EXCLUDED.project_name,
		    intro_template = EXCLUDED.intro_template,
		    costs_template = EXCLUDED.costs_template,
		    concurrence_signers = EXCLUDED.concurrence_signers,
		    updated_at = now()
		RETURNING id, created_at, updated_at`,
		c.GrantID, c.HeaderTitle, c.HeaderSubtitle, c.ProjectName,
		c.IntroTemplate, c.CostsTemplate, c.ConcurrenceSigners,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

// --- SOW Personnel Descriptions ---

func (q *Queries) ListSOWPersonnelDescriptions(ctx context.Context, sowID string) ([]models.SOWPersonnelDescription, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, sow_id, personnel_id, description_md, sort_order, created_at, updated_at
		FROM sow_personnel_descriptions WHERE sow_id = $1 ORDER BY sort_order, created_at`, sowID)
	if err != nil {
		return nil, fmt.Errorf("listing SOW personnel descriptions: %w", err)
	}
	defer rows.Close()

	var items []models.SOWPersonnelDescription
	for rows.Next() {
		var d models.SOWPersonnelDescription
		if err := rows.Scan(&d.ID, &d.SOWID, &d.PersonnelID, &d.DescriptionMD, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning SOW personnel desc: %w", err)
		}
		items = append(items, d)
	}
	return items, nil
}

func (q *Queries) UpsertSOWPersonnelDescription(ctx context.Context, d *models.SOWPersonnelDescription) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO sow_personnel_descriptions (sow_id, personnel_id, description_md, sort_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sow_id, personnel_id) DO UPDATE SET
		    description_md = EXCLUDED.description_md,
		    sort_order = EXCLUDED.sort_order,
		    updated_at = now()
		RETURNING id, created_at, updated_at`,
		d.SOWID, d.PersonnelID, d.DescriptionMD, d.SortOrder,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (q *Queries) DeleteSOWPersonnelDescription(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sow_personnel_descriptions WHERE id=$1`, id)
	return err
}

// --- SOW Line Item Descriptions ---

func (q *Queries) ListSOWLineItemDescriptions(ctx context.Context, sowID string) ([]models.SOWLineItemDescription, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, sow_id, line_item_id, description_md, sort_order, created_at, updated_at
		FROM sow_line_item_descriptions WHERE sow_id = $1 ORDER BY sort_order, created_at`, sowID)
	if err != nil {
		return nil, fmt.Errorf("listing SOW line item descriptions: %w", err)
	}
	defer rows.Close()

	var items []models.SOWLineItemDescription
	for rows.Next() {
		var d models.SOWLineItemDescription
		if err := rows.Scan(&d.ID, &d.SOWID, &d.LineItemID, &d.DescriptionMD, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning SOW line item desc: %w", err)
		}
		items = append(items, d)
	}
	return items, nil
}

func (q *Queries) UpsertSOWLineItemDescription(ctx context.Context, d *models.SOWLineItemDescription) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO sow_line_item_descriptions (sow_id, line_item_id, description_md, sort_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sow_id, line_item_id) DO UPDATE SET
		    description_md = EXCLUDED.description_md,
		    sort_order = EXCLUDED.sort_order,
		    updated_at = now()
		RETURNING id, created_at, updated_at`,
		d.SOWID, d.LineItemID, d.DescriptionMD, d.SortOrder,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
}

func (q *Queries) DeleteSOWLineItemDescription(ctx context.Context, id string) error {
	_, err := q.pool.Exec(ctx, `DELETE FROM sow_line_item_descriptions WHERE id=$1`, id)
	return err
}

// GetStatementOfWork returns a single SOW by ID.
func (q *Queries) GetStatementOfWork(ctx context.Context, id string) (*models.StatementOfWork, error) {
	var s models.StatementOfWork
	err := q.pool.QueryRow(ctx, `
		SELECT id, subaward_id, fiscal_year, period_start::text, period_end::text,
		       budget_id, COALESCE(scope_text, ''), status, signed_doc_id,
		       created_at, updated_at
		FROM statements_of_work WHERE id = $1`, id).Scan(
		&s.ID, &s.SubawardID, &s.FiscalYear, &s.PeriodStart, &s.PeriodEnd,
		&s.BudgetID, &s.ScopeText, &s.Status, &s.SignedDocID,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting SOW: %w", err)
	}
	return &s, nil
}

// --- Budget Overview ---

// BudgetOverviewByInstitution returns line-item totals grouped by institution, year, and category.
func (q *Queries) BudgetOverviewByInstitution(ctx context.Context, grantID string) ([]models.BudgetInstitutionRow, error) {
	return q.BudgetOverviewByInstitutionFiltered(ctx, grantID, nil)
}

// BudgetOverviewByInstitutionFiltered returns line-item totals grouped by institution, year, and category.
// When institutions is non-nil, only budgets for those institution names are included.
func (q *Queries) BudgetOverviewByInstitutionFiltered(ctx context.Context, grantID string, institutions []string) ([]models.BudgetInstitutionRow, error) {
	var query string
	var args []interface{}

	if len(institutions) > 0 {
		query = `
		SELECT ib.entity_type, ib.entity_id, ib.fiscal_year, ib.id, ib.status,
		       li.line_type, SUM(li.amount)
		FROM institution_budgets ib
		JOIN budget_line_items li ON li.institution_budget_id = ib.id
		WHERE ib.is_latest = true
		AND (
			(ib.entity_type = 'grant' AND ib.entity_id IN (SELECT id FROM grants WHERE id = $1 AND institution = ANY($2)))
			OR
			(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1 AND institution = ANY($2)))
		)
		GROUP BY ib.entity_type, ib.entity_id, ib.fiscal_year, ib.id, ib.status, li.line_type
		ORDER BY ib.entity_type DESC, ib.entity_id, ib.fiscal_year`
		args = []interface{}{grantID, institutions}
	} else {
		query = `
		SELECT ib.entity_type, ib.entity_id, ib.fiscal_year, ib.id, ib.status,
		       li.line_type, SUM(li.amount)
		FROM institution_budgets ib
		JOIN budget_line_items li ON li.institution_budget_id = ib.id
		WHERE ib.is_latest = true
		AND (
			(ib.entity_type = 'grant' AND ib.entity_id = $1)
			OR
			(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1))
		)
		GROUP BY ib.entity_type, ib.entity_id, ib.fiscal_year, ib.id, ib.status, li.line_type
		ORDER BY ib.entity_type DESC, ib.entity_id, ib.fiscal_year`
		args = []interface{}{grantID}
	}

	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("budget overview by institution: %w", err)
	}
	defer rows.Close()

	var result []models.BudgetInstitutionRow
	for rows.Next() {
		var r models.BudgetInstitutionRow
		if err := rows.Scan(&r.EntityType, &r.EntityID, &r.FiscalYear, &r.BudgetID, &r.Status, &r.LineType, &r.Amount); err != nil {
			return nil, fmt.Errorf("scanning budget institution row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// BudgetOverviewByWBS returns budget amounts allocated to each WBS area (and unassigned) by year.
func (q *Queries) BudgetOverviewByWBS(ctx context.Context, grantID string) ([]models.BudgetWBSRow, error) {
	return q.BudgetOverviewByWBSFiltered(ctx, grantID, nil)
}

// BudgetOverviewByWBSFiltered returns WBS area budget amounts, optionally filtered by institution names.
func (q *Queries) BudgetOverviewByWBSFiltered(ctx context.Context, grantID string, institutions []string) ([]models.BudgetWBSRow, error) {
	var instFilter string
	var args []interface{}

	if len(institutions) > 0 {
		instFilter = `
			AND (
				(ib.entity_type = 'grant' AND ib.entity_id IN (SELECT id FROM grants WHERE id = $1 AND institution = ANY($2)))
				OR
				(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1 AND institution = ANY($2)))
			)`
		args = []interface{}{grantID, institutions}
	} else {
		instFilter = `
			AND (
				(ib.entity_type = 'grant' AND ib.entity_id = $1)
				OR
				(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1))
			)`
		args = []interface{}{grantID}
	}

	query := `
		WITH budget_scope AS (
			SELECT ib.id, ib.fiscal_year
			FROM institution_budgets ib
			WHERE ib.is_latest = true` + instFilter + `
		),
		wbs_allocated AS (
			SELECT bs.fiscal_year, w.wbs_area_id::text, SUM(li.amount * w.allocation_percent / 100.0) as amount
			FROM budget_scope bs
			JOIN budget_line_items li ON li.institution_budget_id = bs.id
			JOIN budget_line_item_wbs w ON w.line_item_id = li.id
			GROUP BY bs.fiscal_year, w.wbs_area_id
		),
		unassigned AS (
			SELECT bs.fiscal_year, SUM(li.amount * (100.0 - COALESCE(alloc.total_pct, 0)) / 100.0) as amount
			FROM budget_scope bs
			JOIN budget_line_items li ON li.institution_budget_id = bs.id
			LEFT JOIN (
				SELECT line_item_id, SUM(allocation_percent) as total_pct
				FROM budget_line_item_wbs GROUP BY line_item_id
			) alloc ON alloc.line_item_id = li.id
			WHERE COALESCE(alloc.total_pct, 0) < 100.0
			GROUP BY bs.fiscal_year
		)
		SELECT fiscal_year, wbs_area_id, amount FROM wbs_allocated
		UNION ALL
		SELECT fiscal_year, NULL::text, amount FROM unassigned
		ORDER BY 2 NULLS LAST, 1`

	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("budget overview by WBS: %w", err)
	}
	defer rows.Close()

	var result []models.BudgetWBSRow
	for rows.Next() {
		var r models.BudgetWBSRow
		if err := rows.Scan(&r.FiscalYear, &r.WBSAreaID, &r.Amount); err != nil {
			return nil, fmt.Errorf("scanning budget WBS row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// BudgetOverheadBases returns the F&A base amounts grouped by entity, year, and overhead rate.
// It excludes equipment and participant_support from the base (NSF MTDC rules).
func (q *Queries) BudgetOverheadBases(ctx context.Context, grantID string) ([]models.OverheadBaseRow, error) {
	return q.BudgetOverheadBasesFiltered(ctx, grantID, nil)
}

// BudgetOverheadBasesFiltered returns overhead bases, optionally filtered by institution names.
func (q *Queries) BudgetOverheadBasesFiltered(ctx context.Context, grantID string, institutions []string) ([]models.OverheadBaseRow, error) {
	var query string
	var args []interface{}

	if len(institutions) > 0 {
		query = `
		SELECT ib.entity_type, ib.entity_id, ib.fiscal_year,
		       li.overhead_rate_id, SUM(li.amount)
		FROM institution_budgets ib
		JOIN budget_line_items li ON li.institution_budget_id = ib.id
		WHERE ib.is_latest = true
		AND (
			(ib.entity_type = 'grant' AND ib.entity_id IN (SELECT id FROM grants WHERE id = $1 AND institution = ANY($2)))
			OR
			(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1 AND institution = ANY($2)))
		)
		AND li.overhead_rate_id IS NOT NULL
		AND li.line_type NOT IN ('equipment', 'participant_support')
		GROUP BY ib.entity_type, ib.entity_id, ib.fiscal_year, li.overhead_rate_id
		ORDER BY ib.entity_type DESC, ib.entity_id, ib.fiscal_year`
		args = []interface{}{grantID, institutions}
	} else {
		query = `
		SELECT ib.entity_type, ib.entity_id, ib.fiscal_year,
		       li.overhead_rate_id, SUM(li.amount)
		FROM institution_budgets ib
		JOIN budget_line_items li ON li.institution_budget_id = ib.id
		WHERE ib.is_latest = true
		AND (
			(ib.entity_type = 'grant' AND ib.entity_id = $1)
			OR
			(ib.entity_type = 'subaward' AND ib.entity_id IN (SELECT id FROM subawards WHERE grant_id = $1))
		)
		AND li.overhead_rate_id IS NOT NULL
		AND li.line_type NOT IN ('equipment', 'participant_support')
		GROUP BY ib.entity_type, ib.entity_id, ib.fiscal_year, li.overhead_rate_id
		ORDER BY ib.entity_type DESC, ib.entity_id, ib.fiscal_year`
		args = []interface{}{grantID}
	}

	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("budget overhead bases: %w", err)
	}
	defer rows.Close()

	var result []models.OverheadBaseRow
	for rows.Next() {
		var r models.OverheadBaseRow
		if err := rows.Scan(&r.EntityType, &r.EntityID, &r.FiscalYear, &r.OverheadRateID, &r.BaseAmount); err != nil {
			return nil, fmt.Errorf("scanning overhead base row: %w", err)
		}
		result = append(result, r)
	}
	return result, nil
}

// --- Document Processing Runs ---

// CreateDocumentProcessingRun inserts a new processing-run record and fills in the generated ID.
func (q *Queries) CreateDocumentProcessingRun(ctx context.Context, r *models.DocumentProcessingRun) error {
	return q.pool.QueryRow(ctx, `
		INSERT INTO document_processing_runs
			(document_id, entity_type, entity_id, status, status_detail, llm_model)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		r.DocumentID, r.EntityType, r.EntityID, r.Status, r.StatusDetail, r.LLMModel,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

// UpdateDocumentProcessingRun updates a processing run in-place.
func (q *Queries) UpdateDocumentProcessingRun(ctx context.Context, r *models.DocumentProcessingRun) error {
	// JSONB columns must contain valid JSON; default to "[]" if empty.
	convo := r.Conversation
	if convo == "" {
		convo = "[]"
	}
	actions := r.ActionsTaken
	if actions == "" {
		actions = "[]"
	}
	_, err := q.pool.Exec(ctx, `
		UPDATE document_processing_runs SET
			status=$2, status_detail=$3, summary_md=$4, conversation=$5,
			actions_taken=$6, error_msg=$7, prompt_tokens=$8, completion_tokens=$9,
			started_at=$10, completed_at=$11, updated_at=now()
		WHERE id=$1`,
		r.ID, r.Status, r.StatusDetail, r.SummaryMD, convo,
		actions, r.ErrorMsg, r.PromptTokens, r.CompletionTokens,
		r.StartedAt, r.CompletedAt,
	)
	return err
}

// FailStaleProcessingRuns marks any processing runs stuck in non-terminal states as failed.
// Call at startup to clean up after a crash or restart.
func (q *Queries) FailStaleProcessingRuns(ctx context.Context) (int64, error) {
	tag, err := q.pool.Exec(ctx, `
		UPDATE document_processing_runs
		SET status='failed',
		    status_detail='Server restarted during processing',
		    error_msg='Processing interrupted by server restart',
		    completed_at=NOW(),
		    updated_at=NOW()
		WHERE status NOT IN ('completed', 'failed')`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetDocumentProcessingRun fetches a single run by ID.
func (q *Queries) GetDocumentProcessingRun(ctx context.Context, id string) (*models.DocumentProcessingRun, error) {
	var r models.DocumentProcessingRun
	err := q.pool.QueryRow(ctx, `
		SELECT id, document_id, entity_type, entity_id, status, status_detail,
			summary_md, conversation, actions_taken, error_msg, llm_model,
			prompt_tokens, completion_tokens, started_at, completed_at, created_at, updated_at
		FROM document_processing_runs WHERE id=$1`, id,
	).Scan(&r.ID, &r.DocumentID, &r.EntityType, &r.EntityID, &r.Status, &r.StatusDetail,
		&r.SummaryMD, &r.Conversation, &r.ActionsTaken, &r.ErrorMsg, &r.LLMModel,
		&r.PromptTokens, &r.CompletionTokens, &r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListDocumentProcessingRuns returns all runs for a given document.
func (q *Queries) ListDocumentProcessingRuns(ctx context.Context, documentID string) ([]models.DocumentProcessingRun, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, document_id, entity_type, entity_id, status, status_detail,
			summary_md, conversation, actions_taken, error_msg, llm_model,
			prompt_tokens, completion_tokens, started_at, completed_at, created_at, updated_at
		FROM document_processing_runs WHERE document_id=$1
		ORDER BY created_at DESC`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.DocumentProcessingRun
	for rows.Next() {
		var r models.DocumentProcessingRun
		if err := rows.Scan(&r.ID, &r.DocumentID, &r.EntityType, &r.EntityID, &r.Status, &r.StatusDetail,
			&r.SummaryMD, &r.Conversation, &r.ActionsTaken, &r.ErrorMsg, &r.LLMModel,
			&r.PromptTokens, &r.CompletionTokens, &r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// ListDocumentProcessingRunsByEntity returns all runs for an institution entity.
func (q *Queries) ListDocumentProcessingRunsByEntity(ctx context.Context, entityType, entityID string) ([]models.DocumentProcessingRun, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT id, document_id, entity_type, entity_id, status, status_detail,
			summary_md, conversation, actions_taken, error_msg, llm_model,
			prompt_tokens, completion_tokens, started_at, completed_at, created_at, updated_at
		FROM document_processing_runs WHERE entity_type=$1 AND entity_id=$2
		ORDER BY created_at DESC`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []models.DocumentProcessingRun
	for rows.Next() {
		var r models.DocumentProcessingRun
		if err := rows.Scan(&r.ID, &r.DocumentID, &r.EntityType, &r.EntityID, &r.Status, &r.StatusDetail,
			&r.SummaryMD, &r.Conversation, &r.ActionsTaken, &r.ErrorMsg, &r.LLMModel,
			&r.PromptTokens, &r.CompletionTokens, &r.StartedAt, &r.CompletedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, nil
}
