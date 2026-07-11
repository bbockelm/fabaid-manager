package models

import (
	"time"
)

// Grant represents an NSF grant/award.
type Grant struct {
	ID                   string    `json:"id"`
	AwardNumber          string    `json:"award_number"`
	Title                string    `json:"title"`
	PIName               string    `json:"pi_name"`
	Institution          string    `json:"institution"`
	Agency               string    `json:"agency"`
	StartDate            string    `json:"start_date"`
	EndDate              string    `json:"end_date"`
	TotalBudget          float64   `json:"total_budget"`
	SalaryEscalationRate float64   `json:"salary_escalation_rate"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// WBSArea represents a Work Breakdown Structure area within a grant.
// WBS areas form an arbitrary-depth hierarchy via ParentID.
type WBSArea struct {
	ID          string    `json:"id"`
	GrantID     string    `json:"grant_id"`
	ParentID    *string   `json:"parent_id,omitempty"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Budget      float64   `json:"budget"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PersonnelDefaultWBS is a default WBS effort allocation for a person.
type PersonnelDefaultWBS struct {
	ID          string    `json:"id"`
	PersonnelID string    `json:"personnel_id"`
	WBSAreaID   string    `json:"wbs_area_id"`
	Percent     float64   `json:"percent"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WBSEffortSummary is effort and cost for a WBS area in a single fiscal year.
type WBSEffortSummary struct {
	WBSAreaID    string  `json:"wbs_area_id"`
	WBSCode      string  `json:"wbs_code"`
	WBSName      string  `json:"wbs_name"`
	FiscalYear   int     `json:"fiscal_year"`
	EffortMonths float64 `json:"effort_months"`
	Amount       float64 `json:"amount"`
}

// Personnel represents a person working on the grant.
type Personnel struct {
	ID           string    `json:"id"`
	GrantID      string    `json:"grant_id"`
	WBSAreaID    *string   `json:"wbs_area_id,omitempty"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`  // NSF 1030 category: pi, co_pi, subaward_pi, senior_personnel, postdoc, other_professional, graduate_student, undergraduate_student, clerical, other
	Title        string    `json:"title"` // Descriptive job title: Investigator, Programmer, etc.
	Institution  string    `json:"institution,omitempty"`
	AnnualSalary float64   `json:"annual_salary"`
	FundedMonths float64   `json:"funded_months"`
	StartDate    *string   `json:"start_date,omitempty"`
	EndDate      *string   `json:"end_date,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PersonnelBudgetEntry is a budget summary for one person in one year at one institution.
type PersonnelBudgetEntry struct {
	Institution          string  `json:"institution"`
	FiscalYear           int     `json:"fiscal_year"`
	EffortMonths         float64 `json:"effort_months"`
	SalaryAmount         float64 `json:"salary_amount"`
	FringeAmount         float64 `json:"fringe_amount"`
	SalaryEscalationRate float64 `json:"salary_escalation_rate"`
}

// BudgetLineItem represents a cost line in an institution budget.
type BudgetLineItem struct {
	ID                  string    `json:"id"`
	InstitutionBudgetID string    `json:"institution_budget_id"`
	LineType            string    `json:"line_type"`
	Description         string    `json:"description,omitempty"`
	PersonnelID         *string   `json:"personnel_id,omitempty"`
	EffortMonths        float64   `json:"effort_months"`
	Amount              float64   `json:"amount"`
	OverheadRateID      *string   `json:"overhead_rate_id,omitempty"`
	Notes               string    `json:"notes,omitempty"`
	SortOrder           int       `json:"sort_order"`
	IsManualOverride    bool      `json:"is_manual_override"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// BudgetLineItemWBS tracks a WBS cost allocation for a line item.
type BudgetLineItemWBS struct {
	ID                string  `json:"id"`
	LineItemID        string  `json:"line_item_id"`
	WBSAreaID         string  `json:"wbs_area_id"`
	AllocationPercent float64 `json:"allocation_percent"`
}

// OverheadRate represents a named overhead (F&A) rate for an institution.
type OverheadRate struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	RateName    string    `json:"rate_name"`
	Rate        float64   `json:"rate"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Subaward represents a subaward under a grant.
type Subaward struct {
	ID                   string    `json:"id"`
	GrantID              string    `json:"grant_id"`
	Institution          string    `json:"institution"`
	PIName               string    `json:"pi_name"`
	TotalAmount          float64   `json:"total_amount"`
	SalaryEscalationRate float64   `json:"salary_escalation_rate"`
	StartDate            string    `json:"start_date"`
	EndDate              string    `json:"end_date"`
	Status               string    `json:"status"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Invoice represents an invoice from a billing institution (lead grant or subaward).
type Invoice struct {
	ID            string    `json:"id"`
	EntityType    string    `json:"entity_type"` // 'grant' or 'subaward'
	EntityID      string    `json:"entity_id"`
	SubawardID    *string   `json:"subaward_id,omitempty"` // legacy; set when entity_type='subaward'
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	InvoiceDate   string    `json:"invoice_date"`
	Amount        float64   `json:"amount"`
	PeriodStart   *string   `json:"period_start,omitempty"`
	PeriodEnd     *string   `json:"period_end,omitempty"`
	Status        string    `json:"status"`        // payment status: pending, approved, rejected, paid
	CodingStatus  string    `json:"coding_status"` // expense coding: uncoded, draft, final
	DocumentID    *string    `json:"document_id,omitempty"`
	FiscalYear    *int       `json:"fiscal_year,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// InvoiceExpense is one billed line within an invoice, coded to a category and
// (via InvoiceExpenseWBS) split across WBS areas. line_type may be 'uncategorized'.
type InvoiceExpense struct {
	ID               string    `json:"id"`
	InvoiceID        string    `json:"invoice_id"`
	LineType         string    `json:"line_type"`
	Description      string    `json:"description,omitempty"`
	Amount           float64   `json:"amount"`
	PersonnelID      *string   `json:"personnel_id,omitempty"`
	BudgetLineItemID *string   `json:"budget_line_item_id,omitempty"`
	Notes            string    `json:"notes,omitempty"`
	SortOrder        int       `json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// InvoiceExpenseWBS tracks a WBS cost allocation for an invoice expense line.
// Allocations are NOT required to sum to 100%; any remainder is WBS-uncategorized.
type InvoiceExpenseWBS struct {
	ID                string  `json:"id"`
	InvoiceExpenseID  string  `json:"invoice_expense_id"`
	WBSAreaID         string  `json:"wbs_area_id"`
	AllocationPercent float64 `json:"allocation_percent"`
}

// BillingEntity is an institution that submits invoices against the grant:
// the lead grant itself, or a subaward. TotalBudget is used as the burn-rate denominator.
type BillingEntity struct {
	EntityType  string  `json:"entity_type"`
	EntityID    string  `json:"entity_id"`
	Institution string  `json:"institution"`
	TotalBudget float64 `json:"total_budget"`
	StartDate   string  `json:"start_date,omitempty"`
	EndDate     string  `json:"end_date,omitempty"`
}

// FinalizedExpense is a flattened finalized invoice expense used for actuals/burn rollups.
type FinalizedExpense struct {
	ExpenseID   string   `json:"expense_id"`
	InvoiceID   string   `json:"invoice_id"`
	EntityType  string   `json:"entity_type"`
	EntityID    string   `json:"entity_id"`
	LineType    string   `json:"line_type"`
	Amount      float64  `json:"amount"`
	InvoiceDate string   `json:"invoice_date"`
	PeriodEnd   *string  `json:"period_end,omitempty"`
	// WBS holds this expense's WBS allocations (percent); remainder is uncategorized.
	WBS []InvoiceExpenseWBS `json:"wbs,omitempty"`
}

// Document represents a file stored in S3.
type Document struct {
	ID          string    `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	S3Key       string    `json:"s3_key"`
	FileSize    int64     `json:"file_size"`
	UploadedBy  string    `json:"uploaded_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// StatementOfWork represents an annual statement of work for a subaward.
type StatementOfWork struct {
	ID          string    `json:"id"`
	SubawardID  string    `json:"subaward_id"`
	FiscalYear  int       `json:"fiscal_year"`
	PeriodStart string    `json:"period_start"`
	PeriodEnd   string    `json:"period_end"`
	BudgetID    *string   `json:"budget_id,omitempty"`
	ScopeText   string    `json:"scope_text,omitempty"`
	Status      string    `json:"status"`
	SignedDocID *string   `json:"signed_doc_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SOWConfig holds per-grant template settings for generating SOW documents.
type SOWConfig struct {
	ID                  string    `json:"id"`
	GrantID             string    `json:"grant_id"`
	HeaderTitle         string    `json:"header_title"`
	HeaderSubtitle      string    `json:"header_subtitle"`
	ProjectName         string    `json:"project_name"`
	IntroTemplate       string    `json:"intro_template"`
	CostsTemplate       string    `json:"costs_template"`
	ConcurrenceSigners  string    `json:"concurrence_signers"` // JSON array
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// SOWPersonnelDescription is free-form markdown for what a person does in a SOW.
type SOWPersonnelDescription struct {
	ID            string    `json:"id"`
	SOWID         string    `json:"sow_id"`
	PersonnelID   string    `json:"personnel_id"`
	DescriptionMD string    `json:"description_md"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SOWLineItemDescription is free-form markdown for an atypical line item in a SOW.
type SOWLineItemDescription struct {
	ID            string    `json:"id"`
	SOWID         string    `json:"sow_id"`
	LineItemID    string    `json:"line_item_id"`
	DescriptionMD string    `json:"description_md"`
	SortOrder     int       `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BudgetDocument represents an encrypted budget or budget justification PDF.
type BudgetDocument struct {
	ID           string     `json:"id"`
	EntityType   string     `json:"entity_type"`
	EntityID     string     `json:"entity_id"`
	BudgetID     *string    `json:"budget_id,omitempty"`
	DocType      string     `json:"doc_type"`
	Filename     string     `json:"filename"`
	ContentType  string     `json:"content_type"`
	S3Key        string     `json:"s3_key"`
	FileSize     int64      `json:"file_size"`
	EncryptedDEK []byte     `json:"-"`
	DEKNonce     []byte     `json:"-"`
	UploadedBy   *string    `json:"uploaded_by,omitempty"`
	UploadedName string     `json:"uploaded_by_name,omitempty"`
	Notes        string     `json:"notes"`
	CreatedAt    time.Time  `json:"created_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	DeletedBy    *string    `json:"deleted_by,omitempty"`
}

// InstitutionFringeRate stores a fringe rate for an institution (grant or subaward) per year.
type InstitutionFringeRate struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	FiscalYear int       `json:"fiscal_year"`
	RateName   string    `json:"rate_name"`
	Rate       float64   `json:"rate"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// InstitutionBudget stores a versioned per-year budget for an institution.
type InstitutionBudget struct {
	ID         string    `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	FiscalYear int       `json:"fiscal_year"`
	Version    int       `json:"version"`
	IsLatest   bool      `json:"is_latest"`
	Status     string    `json:"status"`
	Budget     float64   `json:"budget"`
	Notes      string    `json:"notes,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SpendForecast provides a projection of future spending.
type SpendForecast struct {
	Month           string  `json:"month"`
	ProjectedSpend  float64 `json:"projected_spend"`
	CumulativeSpend float64 `json:"cumulative_spend"`
	BudgetRemaining float64 `json:"budget_remaining"`
}

// --- Auth / RBAC models ---

// User represents an application user.
type User struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"display_name"`
	Status      string     `json:"status"`
	LastLogin   *time.Time `json:"last_login,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UserRole is a role assigned to a user.
type UserRole struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// UserIdentity is a federated identity (OIDC) linked to a user.
type UserIdentity struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	Email       string    `json:"email,omitempty"`
	EPPN        string    `json:"eppn,omitempty"`
	OIDC        string    `json:"oidc,omitempty"`
	CILogonID   string    `json:"cilogon_id,omitempty"`
	IdPName     string    `json:"idp_name,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Session represents an authenticated login session.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	TokenHash []byte    `json:"-"` // SHA-256 hash of raw session token; never serialized
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Invite represents a single-use invite link.
type Invite struct {
	ID        string    `json:"id"`
	Token     string    `json:"token,omitempty"` // only populated transiently at creation; never stored in DB
	TokenHash []byte    `json:"-"`               // SHA-256 hash of the raw invite token
	UserID    string    `json:"user_id"`
	Used      bool      `json:"used"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AppConfig is a key-value configuration entry.
type AppConfig struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionInfo is the response for /auth/me with user + session details.
type SessionInfo struct {
	User         *User    `json:"user"`
	Role         string   `json:"role"`
	Roles        []string `json:"roles"`
	Institutions []string `json:"institutions,omitempty"` // for subaward_admin: permitted institutions
	IsDevLogin   bool     `json:"is_dev_login"`
}

// APIKey represents a long-lived API key for programmatic access.
type APIKey struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	KeyHash       string     `json:"-"`          // bcrypt hash — never exposed
	KeyPrefix     string     `json:"key_prefix"` // "fabaid_Xxxx" — safe to display
	Roles         []string   `json:"roles"`
	CreatedBy     string     `json:"created_by"`
	CreatedByName string     `json:"created_by_name,omitempty"` // joined from users
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	IdleTimeoutS  *int       `json:"idle_timeout_s,omitempty"` // seconds
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

// APIKeyCreateResponse is returned once when a key is first created.
// The RawKey is shown to the user exactly once and never stored.
type APIKeyCreateResponse struct {
	APIKey
	RawKey string `json:"raw_key"`
}

// Backup represents a point-in-time backup stored in S3.
type Backup struct {
	ID           string     `json:"id"`
	Filename     string     `json:"filename"`
	S3Key        string     `json:"s3_key"`
	S3Bucket     string     `json:"s3_bucket"`
	SizeBytes    int64      `json:"size_bytes"`
	Status       string     `json:"status"` // running, completed, failed
	StatusDetail string     `json:"status_detail,omitempty"`
	ErrorMsg     string     `json:"error_msg,omitempty"`
	InitiatedBy  string     `json:"initiated_by"` // scheduler, manual
	Encrypted    bool       `json:"encrypted"`
	Checksum     string     `json:"checksum,omitempty"` // SHA-256 of the archive
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ObjectHash stores a SHA-256 hash for an S3 object.
type ObjectHash struct {
	ID        string    `json:"id"`
	S3Key     string    `json:"s3_key"`
	SHA256    string    `json:"sha256_hash"`
	SizeBytes int64     `json:"size_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserInstitutionAccess maps a subaward_admin user to an institution they manage.
type UserInstitutionAccess struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Institution string    `json:"institution"`
	CreatedAt   time.Time `json:"created_at"`
}

// BackupSettings holds admin-configurable backup parameters.
type BackupSettings struct {
	BackupFrequencyHours int    `json:"backup_frequency_hours"` // 0 = disabled
	BackupBucket         string `json:"backup_bucket"`          // empty = use default bucket
	BackupEndpoint       string `json:"backup_endpoint"`        // empty = use default endpoint
	BackupAccessKey      string `json:"backup_access_key"`      // empty = use default creds
	BackupSecretKey      string `json:"backup_secret_key"`      // empty = use default creds
	BackupUseSSL         bool   `json:"backup_use_ssl"`
}

// --- Budget Overview ---

// BudgetOverviewResponse is the overall project budget overview.
type BudgetOverviewResponse struct {
	Institutions   []*BudgetOverviewInstitution `json:"institutions"`
	WBSAreas       []*BudgetOverviewWBS         `json:"wbs_areas"`
	YearlyTotals   map[int]float64              `json:"yearly_totals"`
	YearlyDirect   map[int]float64              `json:"yearly_direct"`
	YearlyIndirect map[int]float64              `json:"yearly_indirect"`
	GrandTotal     float64                      `json:"grand_total"`
	GrandDirect    float64                      `json:"grand_direct"`
	GrandIndirect  float64                      `json:"grand_indirect"`
	AwardTotal     float64                      `json:"award_total"`
}

// BudgetOverviewInstitution is an institution's budget summary in the overview.
type BudgetOverviewInstitution struct {
	EntityType    string                      `json:"entity_type"`
	EntityID      string                      `json:"entity_id"`
	Name          string                      `json:"name"`
	IsLead        bool                        `json:"is_lead"`
	Years         map[int]*BudgetOverviewYear `json:"years"`
	Total         float64                     `json:"total"`
	DirectTotal   float64                     `json:"direct_total"`
	IndirectTotal float64                     `json:"indirect_total"`
}

// BudgetOverviewYear is budget totals for one institution-year.
type BudgetOverviewYear struct {
	BudgetID      string             `json:"budget_id"`
	Status        string             `json:"status"`
	Total         float64            `json:"total"`
	DirectCosts   float64            `json:"direct_costs"`
	IndirectCosts float64            `json:"indirect_costs"`
	ByCategory    map[string]float64 `json:"by_category"`
}

// BudgetOverviewWBS is a WBS area's budget summary in the overview.
type BudgetOverviewWBS struct {
	WBSAreaID *string         `json:"wbs_area_id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Years     map[int]float64 `json:"years"`
	Total     float64         `json:"total"`
}

// BudgetInstitutionRow is a raw row from the budget-by-institution query.
type BudgetInstitutionRow struct {
	EntityType string
	EntityID   string
	FiscalYear int
	BudgetID   string
	Status     string
	LineType   string
	Amount     float64
}

// BudgetWBSRow is a raw row from the budget-by-WBS query.
type BudgetWBSRow struct {
	FiscalYear int
	WBSAreaID  *string
	Amount     float64
}

// OverheadBaseRow is the F&A base amount for a single entity/year/rate from the overhead bases query.
type OverheadBaseRow struct {
	EntityType     string
	EntityID       string
	FiscalYear     int
	OverheadRateID string
	BaseAmount     float64
}

// DocumentProcessingRun records an AI processing run on an uploaded budget document.
type DocumentProcessingRun struct {
	ID               string     `json:"id"`
	DocumentID       *string    `json:"document_id,omitempty"`
	InvoiceID        *string    `json:"invoice_id,omitempty"`
	RunType          string     `json:"run_type"` // budget_extraction | invoice_coding
	EntityType       string     `json:"entity_type"`
	EntityID         string     `json:"entity_id"`
	Status           string     `json:"status"`        // pending, extracting, processing, applying, completed, failed
	StatusDetail     string     `json:"status_detail"`
	SummaryMD        string     `json:"summary_md"`
	Conversation     string     `json:"conversation"`   // JSON array of messages
	ActionsTaken     string     `json:"actions_taken"`   // JSON array of tool calls + results
	ErrorMsg         string     `json:"error_msg,omitempty"`
	LLMModel         string     `json:"llm_model"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
