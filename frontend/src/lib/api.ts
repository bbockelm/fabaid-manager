const API_BASE = '/api/v1';

export class ValidationError extends Error {
  validationErrors: string[];
  constructor(message: string, validationErrors: string[]) {
    super(message);
    this.name = 'ValidationError';
    this.validationErrors = validationErrors;
  }
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${url}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...init,
  });
  if (!res.ok) {
    if (res.status === 401) {
      // Redirect to login on auth failure (unless already on login page)
      if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
        window.location.href = '/login';
        return null as T;
      }
    }
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed: ${res.status}`);
  }
  if (res.status === 204) return null as T;
  return res.json();
}

// --- Auth Types ---

export interface AppUser {
  id: string;
  display_name: string;
  status: string;
  last_login?: string;
  created_at: string;
  updated_at: string;
}

export interface SessionInfo {
  user: AppUser | null;
  role: string;
  roles: string[];
  institutions?: string[];
  is_dev_login?: boolean;
}

export interface AuthMode {
  mode: 'dev' | 'oidc';
  oidc_configured: boolean;
  callback_url: string;
}

export interface UserIdentity {
  id: string;
  user_id: string;
  issuer: string;
  subject: string;
  email?: string;
  eppn?: string;
  oidc?: string;
  cilogon_id?: string;
  idp_name?: string;
  display_name?: string;
  created_at: string;
}

export interface UserInfo extends AppUser {
  roles: string[];
  identities: UserIdentity[];
  institutions: string[];
}

export interface InviteInfo {
  id: string;
  token: string;
  user_id: string;
  used: boolean;
  expires_at: string;
  created_at: string;
}

export interface OIDCConfig {
  oidc_issuer: string;
  oidc_client_id: string;
  secret_set: boolean;
  callback_url: string;
}

export interface APIKeyInfo {
  id: string;
  name: string;
  key_prefix: string;
  roles: string[];
  created_by: string;
  created_by_name?: string;
  created_at: string;
  last_used_at?: string;
  idle_timeout_s?: number;
  expires_at?: string;
  revoked_at?: string;
}

export interface APIKeyCreateResponse extends APIKeyInfo {
  raw_key: string;
}

// --- Backup Types ---

export interface BackupRecord {
  id: string;
  filename: string;
  s3_key: string;
  s3_bucket: string;
  size_bytes: number;
  status: string;  // running, completed, failed
  status_detail?: string;
  error_msg?: string;
  initiated_by: string;
  encrypted: boolean;
  checksum?: string;
  started_at: string;
  completed_at?: string;
  created_at: string;
}

export interface BackupSettings {
  backup_frequency_hours: number;
  backup_bucket: string;
  backup_endpoint: string;
  backup_access_key: string;
  backup_secret_key: string;
  backup_use_ssl: boolean;
}

// --- Types ---

export interface Grant {
  id: string;
  award_number: string;
  title: string;
  pi_name: string;
  institution: string;
  agency: string;
  start_date: string;
  end_date: string;
  total_budget: number;
  salary_escalation_rate: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface WBSArea {
  id: string;
  grant_id: string;
  parent_id?: string;
  code: string;
  name: string;
  description?: string;
  budget: number;
  created_at?: string;
  updated_at?: string;
}

export interface PersonnelDefaultWBS {
  id: string;
  personnel_id: string;
  wbs_area_id: string;
  percent: number;
  created_at?: string;
  updated_at?: string;
}

export interface WBSEffortSummary {
  wbs_area_id: string;
  wbs_code: string;
  wbs_name: string;
  fiscal_year: number;
  effort_months: number;
  amount: number;
}

export interface Personnel {
  id: string;
  grant_id: string;
  wbs_area_id?: string;
  name: string;
  role: string;
  title: string;
  institution?: string;
  annual_salary: number;
  funded_months: number;
  start_date?: string;
  end_date?: string;
}

export interface PersonnelBudgetEntry {
  institution: string;
  fiscal_year: number;
  effort_months: number;
  salary_amount: number;
  fringe_amount: number;
  salary_escalation_rate: number;
}

// New budget model types
export interface OverheadRate {
  id: string;
  entity_type: string;
  entity_id: string;
  rate_name: string;
  rate: number;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface BudgetLineItem {
  id: string;
  institution_budget_id: string;
  line_type: string;
  description?: string;
  personnel_id?: string;
  effort_months: number;
  amount: number;
  overhead_rate_id?: string | null;
  notes?: string;
  sort_order: number;
  is_manual_override?: boolean;
  created_at: string;
  updated_at: string;
}

export interface BudgetLineItemWBS {
  id: string;
  line_item_id: string;
  wbs_area_id: string;
  allocation_percent: number;
}

export interface BudgetDocument {
  id: string;
  entity_type: string;
  entity_id: string;
  budget_id?: string;
  doc_type: string; // 'budget' | 'budget_justification'
  filename: string;
  content_type: string;
  s3_key: string;
  file_size: number;
  uploaded_by?: string;
  uploaded_by_name?: string;
  notes: string;
  created_at: string;
  deleted_at?: string;
  deleted_by?: string;
}

export interface Subaward {
  id: string;
  grant_id: string;
  institution: string;
  pi_name: string;
  total_amount: number;
  salary_escalation_rate: number;
  start_date: string;
  end_date: string;
  status: string;
}

export interface Invoice {
  id: string;
  entity_type: string;
  entity_id: string;
  subaward_id?: string;
  invoice_number?: string;
  invoice_date: string;
  amount: number;
  period_start?: string;
  period_end?: string;
  status: string;         // payment: pending/approved/rejected/paid
  coding_status: string;  // expense coding: uncoded/draft/final
  document_id?: string;
  fiscal_year?: number;
  notes?: string;
}

export interface InvoiceExpenseWBS {
  id?: string;
  invoice_expense_id?: string;
  wbs_area_id: string;
  allocation_percent: number;
}

export interface InvoiceExpense {
  id: string;
  invoice_id: string;
  line_type: string; // includes 'uncategorized' and 'indirect'
  description?: string;
  amount: number;
  personnel_id?: string;
  budget_line_item_id?: string;
  notes?: string;
  sort_order: number;
  wbs?: InvoiceExpenseWBS[];
}

export interface InvoiceDetail extends Invoice {
  expenses: InvoiceExpense[];
}

export interface BurnRow {
  entity_type: string;
  entity_id: string;
  institution: string;
  budget: number;
  actual_total: number;
  actual_non_capital: number;
  last_period_end?: string;
  months_since_last: number;
  behind: boolean;
  estimated_monthly: number;
  projected_since_last: number;
  projected_to_date: number;
  expected_remaining: number;
  expected_year_end_funds: number;
}

export interface InvoiceAnalytics {
  total_actual: number;
  by_wbs: { wbs_area_id: string | null; name: string; amount: number; uncategorized?: boolean }[];
  by_category: { line_type: string; amount: number; uncategorized?: boolean }[];
  by_institution: { entity_type: string; entity_id: string; institution: string; amount: number }[];
  uncategorized: { category: number; wbs: number };
  behind: { entity_type: string; entity_id: string; institution: string; last_period_end?: string | null; months_since_last?: number | null }[];
  burn: BurnRow[];
}

export interface Document {
  id: string;
  entity_type: string;
  entity_id: string;
  filename: string;
  content_type: string;
  s3_key: string;
  file_size: number;
}

export interface InstitutionFringeRate {
  id: string;
  entity_type: string;
  entity_id: string;
  fiscal_year: number;
  rate_name: string;
  rate: number;
  created_at: string;
  updated_at: string;
}

export interface InstitutionBudget {
  id: string;
  entity_type: string;
  entity_id: string;
  fiscal_year: number;
  version: number;
  is_latest: boolean;
  status: string; // 'draft' | 'final'
  budget: number;
  notes?: string;
  created_at: string;
}

export interface BudgetSummary {
  category: string;
  planned_total: number;
  actual_total: number;
  remaining: number;
  percent_spent: number;
}

export interface BudgetSummaryByYear {
  fiscal_year: number;
  category: string;
  planned_total: number;
  actual_total: number;
  remaining: number;
  percent_spent: number;
}

// --- Budget Overview ---

export interface BudgetOverviewYear {
  budget_id: string;
  status: string;
  total: number;
  direct_costs: number;
  indirect_costs: number;
  by_category: Record<string, number>;
}

export interface BudgetOverviewInstitution {
  entity_type: string;
  entity_id: string;
  name: string;
  is_lead: boolean;
  years: Record<string, BudgetOverviewYear>;
  total: number;
  direct_total: number;
  indirect_total: number;
}

export interface BudgetOverviewWBS {
  wbs_area_id: string | null;
  code: string;
  name: string;
  years: Record<string, number>;
  total: number;
}

export interface BudgetOverviewResponse {
  institutions: BudgetOverviewInstitution[];
  wbs_areas: BudgetOverviewWBS[];
  yearly_totals: Record<string, number>;
  yearly_direct: Record<string, number>;
  yearly_indirect: Record<string, number>;
  grand_total: number;
  grand_direct: number;
  grand_indirect: number;
  award_total: number;
}

export interface StatementOfWork {
  id: string;
  subaward_id: string;
  fiscal_year: number;
  period_start: string;
  period_end: string;
  budget_id?: string;
  scope_text?: string;
  status: string;
  signed_doc_id?: string;
}

export interface SOWConfig {
  id: string;
  grant_id: string;
  header_title: string;
  header_subtitle: string;
  project_name: string;
  intro_template: string;
  costs_template: string;
  concurrence_signers: string; // JSON array
  created_at?: string;
  updated_at?: string;
}

export interface SOWPersonnelDescription {
  id: string;
  sow_id: string;
  personnel_id: string;
  description_md: string;
  sort_order: number;
  created_at?: string;
  updated_at?: string;
}

export interface SOWLineItemDescription {
  id: string;
  sow_id: string;
  line_item_id: string;
  description_md: string;
  sort_order: number;
  created_at?: string;
  updated_at?: string;
}

// --- Document Processing ---

export interface DocumentProcessingRun {
  id: string;
  document_id?: string;
  invoice_id?: string;
  run_type?: string; // budget_extraction | invoice_coding
  entity_type: string;
  entity_id: string;
  status: string; // pending, extracting, processing, applying, completed, failed
  status_detail: string;
  summary_md: string;
  conversation: string;   // JSON string
  actions_taken: string;  // JSON string
  error_msg?: string;
  llm_model: string;
  prompt_tokens: number;
  completion_tokens: number;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

// --- Grants ---

export const api = {
  grants: {
    list: () => fetchJSON<Grant[]>('/grants'),
    get: (id: string) => fetchJSON<Grant>(`/grants/${id}`),
    create: (data: Partial<Grant>) =>
      fetchJSON<Grant>('/grants', { method: 'POST', body: JSON.stringify(data) }),
    update: (id: string, data: Partial<Grant>) =>
      fetchJSON<Grant>(`/grants/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (id: string) =>
      fetchJSON<void>(`/grants/${id}`, { method: 'DELETE' }),
  },

  // Budget summary helpers — derived client-side from institution budgets.
  // These return empty arrays until budget data is allocated.
  budget: {
    summary: async (_grantId: string): Promise<BudgetSummary[]> => {
      // TODO: implement backend aggregate endpoint
      return [];
    },
    summaryByYear: async (_grantId: string): Promise<BudgetSummaryByYear[]> => {
      // TODO: implement backend aggregate endpoint
      return [];
    },
    list: async (_grantId: string, _year?: number): Promise<BudgetSummaryByYear[]> => {
      // TODO: implement backend aggregate endpoint
      return [];
    },
    overview: (grantId: string, institutions?: string[]) => {
      const params = institutions?.length
        ? `?institutions=${institutions.map(encodeURIComponent).join(',')}`
        : '';
      return fetchJSON<BudgetOverviewResponse>(`/grants/${grantId}/budget-overview${params}`);
    },
  },

  wbs: {
    list: (grantId: string) => fetchJSON<WBSArea[]>(`/grants/${grantId}/wbs`),
    get: (grantId: string, id: string) => fetchJSON<WBSArea>(`/grants/${grantId}/wbs/${id}`),
    create: (grantId: string, data: Partial<WBSArea>) =>
      fetchJSON<WBSArea>(`/grants/${grantId}/wbs`, { method: 'POST', body: JSON.stringify(data) }),
    update: (grantId: string, id: string, data: Partial<WBSArea>) =>
      fetchJSON<WBSArea>(`/grants/${grantId}/wbs/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (grantId: string, id: string) =>
      fetchJSON<void>(`/grants/${grantId}/wbs/${id}`, { method: 'DELETE' }),
    effortSummary: (grantId: string, institutions?: string[]) => {
      const params = institutions?.length
        ? `?institutions=${institutions.map(encodeURIComponent).join(',')}`
        : '';
      return fetchJSON<WBSEffortSummary[]>(`/grants/${grantId}/wbs/effort-summary${params}`);
    },
    // Download URL for the effort/cost breakdown as 'csv' or 'md' (same-origin, uses the session cookie).
    effortSummaryExportUrl: (grantId: string, format: 'csv' | 'md', institutions?: string[]) => {
      const inst = institutions?.length ? `&institutions=${institutions.map(encodeURIComponent).join(',')}` : '';
      return `${API_BASE}/grants/${grantId}/wbs/effort-summary?format=${format}${inst}`;
    },
  },

  personnel: {
    list: (grantId: string) => fetchJSON<Personnel[]>(`/grants/${grantId}/personnel`),
    create: (grantId: string, data: Partial<Personnel>) =>
      fetchJSON<Personnel>(`/grants/${grantId}/personnel`, { method: 'POST', body: JSON.stringify(data) }),
    update: (grantId: string, id: string, data: Partial<Personnel>) =>
      fetchJSON<Personnel>(`/grants/${grantId}/personnel/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (grantId: string, id: string) =>
      fetchJSON<void>(`/grants/${grantId}/personnel/${id}`, { method: 'DELETE' }),
    titles: (grantId: string) => fetchJSON<string[]>(`/grants/${grantId}/personnel/titles`),
    budgetSummary: (grantId: string, personnelId: string) =>
      fetchJSON<PersonnelBudgetEntry[]>(`/grants/${grantId}/personnel/${personnelId}/budget-summary`),
    defaultWBS: (grantId: string, personnelId: string) =>
      fetchJSON<PersonnelDefaultWBS[]>(`/grants/${grantId}/personnel/${personnelId}/default-wbs`),
    setDefaultWBS: (grantId: string, personnelId: string, items: Partial<PersonnelDefaultWBS>[]) =>
      fetchJSON<PersonnelDefaultWBS[]>(`/grants/${grantId}/personnel/${personnelId}/default-wbs`, {
        method: 'PUT', body: JSON.stringify(items),
      }),
  },

  // Overhead rates per institution
  overheadRates: {
    list: (entityType: string, entityId: string) =>
      fetchJSON<OverheadRate[]>(`/institution-rates/${entityType}/${entityId}/overhead-rates`),
    create: (entityType: string, entityId: string, data: Partial<OverheadRate>) =>
      fetchJSON<OverheadRate>(`/institution-rates/${entityType}/${entityId}/overhead-rates`, {
        method: 'POST', body: JSON.stringify(data),
      }),
    update: (entityType: string, entityId: string, id: string, data: Partial<OverheadRate>) =>
      fetchJSON<OverheadRate>(`/institution-rates/${entityType}/${entityId}/overhead-rates/${id}`, {
        method: 'PUT', body: JSON.stringify(data),
      }),
    delete: (entityType: string, entityId: string, id: string) =>
      fetchJSON<void>(`/institution-rates/${entityType}/${entityId}/overhead-rates/${id}`, { method: 'DELETE' }),
  },

  // Budget line items within a versioned institution budget
  budgetLineItems: {
    list: (entityType: string, entityId: string, budgetId: string) =>
      fetchJSON<BudgetLineItem[]>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items`),
    create: (entityType: string, entityId: string, budgetId: string, data: Partial<BudgetLineItem>) =>
      fetchJSON<BudgetLineItem>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items`, {
        method: 'POST', body: JSON.stringify(data),
      }),
    update: (entityType: string, entityId: string, budgetId: string, id: string, data: Partial<BudgetLineItem>) =>
      fetchJSON<BudgetLineItem>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items/${id}`, {
        method: 'PUT', body: JSON.stringify(data),
      }),
    delete: (entityType: string, entityId: string, budgetId: string, id: string) =>
      fetchJSON<void>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items/${id}`, { method: 'DELETE' }),
    listWBS: (entityType: string, entityId: string, budgetId: string, lineItemId: string) =>
      fetchJSON<BudgetLineItemWBS[]>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items/${lineItemId}/wbs`),
    setWBS: (entityType: string, entityId: string, budgetId: string, lineItemId: string, allocations: Partial<BudgetLineItemWBS>[]) =>
      fetchJSON<BudgetLineItemWBS[]>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/line-items/${lineItemId}/wbs`, {
        method: 'PUT', body: JSON.stringify(allocations),
      }),
  },

  fringeRates: {
    list: (entityType: string, entityId: string) =>
      fetchJSON<InstitutionFringeRate[]>(`/institution-rates/${entityType}/${entityId}/fringe-rates`),
    upsert: (entityType: string, entityId: string, data: Partial<InstitutionFringeRate>) =>
      fetchJSON<InstitutionFringeRate>(`/institution-rates/${entityType}/${entityId}/fringe-rates`, {
        method: 'PUT', body: JSON.stringify(data),
      }),
    delete: (entityType: string, entityId: string, id: string) =>
      fetchJSON<void>(`/institution-rates/${entityType}/${entityId}/fringe-rates/${id}`, { method: 'DELETE' }),
  },

  institutionBudgets: {
    list: (entityType: string, entityId: string, latestOnly = true) =>
      fetchJSON<InstitutionBudget[]>(`/institution-rates/${entityType}/${entityId}/budgets${latestOnly ? '?latest=true' : ''}`),
    create: (entityType: string, entityId: string, data: Partial<InstitutionBudget>) =>
      fetchJSON<InstitutionBudget>(`/institution-rates/${entityType}/${entityId}/budgets`, {
        method: 'POST', body: JSON.stringify(data),
      }),
    finalize: async (entityType: string, entityId: string, budgetId: string) => {
      const res = await fetch(`${API_BASE}/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/finalize`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });
      if (res.status === 422) {
        const body = await res.json();
        throw new ValidationError(body.error, body.validation_errors);
      }
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Request failed: ${res.status}`);
      }
      const body = await res.json().catch(() => ({ warnings: [] }));
      return { warnings: (body.warnings ?? []) as string[] };
    },
    duplicate: (entityType: string, entityId: string, budgetId: string) =>
      fetchJSON<InstitutionBudget>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}/duplicate`, { method: 'POST' }),
    delete: (entityType: string, entityId: string, budgetId: string) =>
      fetchJSON<void>(`/institution-rates/${entityType}/${entityId}/budgets/${budgetId}`, { method: 'DELETE' }),
  },

  budgetDocuments: {
    list: (entityType: string, entityId: string, includeDeleted = false) =>
      fetchJSON<BudgetDocument[]>(`/institution-rates/${entityType}/${entityId}/budget-documents${includeDeleted ? '?include_deleted=true' : ''}`),
    upload: async (entityType: string, entityId: string, file: File, docType: string, budgetId?: string, notes?: string) => {
      const form = new FormData();
      form.append('file', file);
      form.append('doc_type', docType);
      if (budgetId) form.append('budget_id', budgetId);
      if (notes) form.append('notes', notes);
      const res = await fetch(
        `${API_BASE}/institution-rates/${entityType}/${entityId}/budget-documents`,
        { method: 'POST', body: form, credentials: 'include' }
      );
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Upload failed: ${res.status}`);
      }
      return res.json() as Promise<BudgetDocument>;
    },
    get: (entityType: string, entityId: string, docId: string) =>
      fetchJSON<BudgetDocument>(`/institution-rates/${entityType}/${entityId}/budget-documents/${docId}`),
    downloadUrl: (entityType: string, entityId: string, docId: string) =>
      `${API_BASE}/institution-rates/${entityType}/${entityId}/budget-documents/${docId}/download`,
    delete: (entityType: string, entityId: string, docId: string) =>
      fetchJSON<void>(`/institution-rates/${entityType}/${entityId}/budget-documents/${docId}`, { method: 'DELETE' }),
    // AI processing
    processDocument: (entityType: string, entityId: string, docId: string, userPrompt?: string) =>
      fetchJSON<{ run_id: string; status: string }>(
        `/institution-rates/${entityType}/${entityId}/budget-documents/${docId}/process`,
        { method: 'POST', body: JSON.stringify({ user_prompt: userPrompt || '' }) }
      ),
    previewExtract: (entityType: string, entityId: string, docId: string) =>
      fetchJSON<{ filename: string; tables: number; has_text: boolean; markdown: string; text_size: number }>(
        `/institution-rates/${entityType}/${entityId}/budget-documents/${docId}/preview-extract`,
        { method: 'POST' }
      ),
    listProcessingRuns: (entityType: string, entityId: string, docId: string) =>
      fetchJSON<DocumentProcessingRun[]>(`/institution-rates/${entityType}/${entityId}/budget-documents/${docId}/processing-runs`),
    getProcessingRun: (entityType: string, entityId: string, runId: string) =>
      fetchJSON<DocumentProcessingRun>(`/institution-rates/${entityType}/${entityId}/processing-runs/${runId}`),
    listEntityProcessingRuns: (entityType: string, entityId: string) =>
      fetchJSON<DocumentProcessingRun[]>(`/institution-rates/${entityType}/${entityId}/processing-runs`),
  },

  subawards: {
    list: (grantId: string) => fetchJSON<Subaward[]>(`/grants/${grantId}/subawards`),
    create: (grantId: string, data: Partial<Subaward>) =>
      fetchJSON<Subaward>(`/grants/${grantId}/subawards`, { method: 'POST', body: JSON.stringify(data) }),
    update: (grantId: string, id: string, data: Partial<Subaward>) =>
      fetchJSON<Subaward>(`/grants/${grantId}/subawards/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
    delete: (grantId: string, id: string) =>
      fetchJSON<void>(`/grants/${grantId}/subawards/${id}`, { method: 'DELETE' }),
  },

  invoices: {
    list: (grantId: string, subawardId: string) =>
      fetchJSON<Invoice[]>(`/grants/${grantId}/subawards/${subawardId}/invoices`),
    create: (grantId: string, subawardId: string, data: Partial<Invoice>) =>
      fetchJSON<Invoice>(`/grants/${grantId}/subawards/${subawardId}/invoices`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    updateStatus: (grantId: string, subawardId: string, invoiceId: string, status: string) =>
      fetchJSON<void>(`/grants/${grantId}/subawards/${subawardId}/invoices/${invoiceId}/status`, {
        method: 'PATCH',
        body: JSON.stringify({ status }),
      }),
    upload: async (grantId: string, subawardId: string, invoiceId: string, file: File) => {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(
        `${API_BASE}/grants/${grantId}/subawards/${subawardId}/invoices/${invoiceId}/upload`,
        { method: 'POST', body: form }
      );
      if (!res.ok) throw new Error('Upload failed');
      return res.json() as Promise<Document>;
    },
  },

  sow: {
    list: (grantId: string, subawardId: string) =>
      fetchJSON<StatementOfWork[]>(`/grants/${grantId}/subawards/${subawardId}/sow`),
    create: (grantId: string, subawardId: string, data: Partial<StatementOfWork>) =>
      fetchJSON<StatementOfWork>(`/grants/${grantId}/subawards/${subawardId}/sow`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    update: (grantId: string, subawardId: string, id: string, data: Partial<StatementOfWork>) =>
      fetchJSON<StatementOfWork>(`/grants/${grantId}/subawards/${subawardId}/sow/${id}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    delete: (grantId: string, subawardId: string, id: string) =>
      fetchJSON<void>(`/grants/${grantId}/subawards/${subawardId}/sow/${id}`, { method: 'DELETE' }),
    uploadSigned: async (grantId: string, subawardId: string, sowId: string, file: File) => {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(
        `${API_BASE}/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/upload-signed`,
        { method: 'POST', body: form }
      );
      if (!res.ok) throw new Error('Upload failed');
      return res.json() as Promise<Document>;
    },
    renderUrl: (grantId: string, subawardId: string, sowId: string) =>
      `${API_BASE}/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/render`,

    // Personnel descriptions
    listPersonnelDescriptions: (grantId: string, subawardId: string, sowId: string) =>
      fetchJSON<SOWPersonnelDescription[]>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/personnel-descriptions`),
    upsertPersonnelDescription: (grantId: string, subawardId: string, sowId: string, data: Partial<SOWPersonnelDescription>) =>
      fetchJSON<SOWPersonnelDescription>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/personnel-descriptions`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    deletePersonnelDescription: (grantId: string, subawardId: string, sowId: string, descId: string) =>
      fetchJSON<void>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/personnel-descriptions/${descId}`, { method: 'DELETE' }),

    // Line item descriptions
    listLineItemDescriptions: (grantId: string, subawardId: string, sowId: string) =>
      fetchJSON<SOWLineItemDescription[]>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/line-item-descriptions`),
    upsertLineItemDescription: (grantId: string, subawardId: string, sowId: string, data: Partial<SOWLineItemDescription>) =>
      fetchJSON<SOWLineItemDescription>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/line-item-descriptions`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    deleteLineItemDescription: (grantId: string, subawardId: string, sowId: string, descId: string) =>
      fetchJSON<void>(`/grants/${grantId}/subawards/${subawardId}/sow/${sowId}/line-item-descriptions/${descId}`, { method: 'DELETE' }),
  },

  sowConfig: {
    get: (grantId: string) =>
      fetchJSON<SOWConfig>(`/grants/${grantId}/sow-config`),
    upsert: (grantId: string, data: Partial<SOWConfig>) =>
      fetchJSON<SOWConfig>(`/grants/${grantId}/sow-config`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
  },

  documents: {
    get: (id: string) => fetchJSON<Document>(`/documents/${id}`),
    downloadUrl: (id: string) => `${API_BASE}/documents/${id}/download`,
  },

  nsf1030: {
    url: (entityType: string, entityId: string, year?: number) => {
      const base = `${API_BASE}/institution-rates/${entityType}/${entityId}/nsf1030`;
      return year ? `${base}?year=${year}` : base;
    },
  },

  backup: {
    list: () => fetchJSON<BackupRecord[]>('/backups'),
    trigger: () => fetchJSON<{ status: string; id: string }>('/backups/trigger', { method: 'POST' }),
    downloadUrl: (backupId?: string) =>
      backupId ? `${API_BASE}/backups/${backupId}/download` : `${API_BASE}/backup`,
    restore: (backupId: string) =>
      fetchJSON<{ status: string }>(`/backups/${backupId}/restore`, { method: 'POST' }),
    delete: (backupId: string) =>
      fetchJSON<void>(`/backups/${backupId}`, { method: 'DELETE' }),
    deleteFailed: () =>
      fetchJSON<{ deleted: number }>('/backups/failed', { method: 'DELETE' }),
    getPerBackupKey: (backupId: string) =>
      fetchJSON<{ key: string; filename: string }>(`/backups/${backupId}/key`),
    getGeneralBackupKey: () =>
      fetchJSON<{ key: string }>('/backups/general-key'),
    uploadRestore: async (file: File, encrypted = true, decryptKey?: string) => {
      const form = new FormData();
      form.append('file', file);
      form.append('encrypted', String(encrypted));
      if (decryptKey) form.append('decrypt_key', decryptKey);
      const res = await fetch(`${API_BASE}/backups/upload-restore`, {
        method: 'POST',
        body: form,
        credentials: 'include',
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || `Upload failed: ${res.status}`);
      }
      return res.json() as Promise<{ status: string }>;
    },
    getSettings: () => fetchJSON<BackupSettings>('/backups/settings'),
    updateSettings: (data: Partial<BackupSettings>) =>
      fetchJSON<BackupSettings>('/backups/settings', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
  },

  // --- Auth ---
  auth: {
    mode: () => fetchJSON<AuthMode>('/auth/mode'),
    me: () => fetchJSON<SessionInfo>('/auth/me'),
    logout: () => fetchJSON<void>('/auth/logout', { method: 'POST' }),
    devLogin: (displayName: string, role: string) =>
      fetchJSON<{ session_id: string; role: string; user_id: string }>('/auth/dev-login', {
        method: 'POST',
        body: JSON.stringify({ display_name: displayName, role }),
      }),
    updateProfile: (displayName: string) =>
      fetchJSON<AppUser>('/auth/profile', {
        method: 'PUT',
        body: JSON.stringify({ display_name: displayName }),
      }),
    oidcLoginUrl: (invite?: string) => {
      const base = `${API_BASE}/auth/oidc/login`;
      return invite ? `${base}?invite=${invite}` : base;
    },
  },

  // --- Admin ---
  admin: {
    listUsers: () => fetchJSON<UserInfo[]>('/admin/users'),
    createUser: (displayName: string, role: string) =>
      fetchJSON<AppUser>('/admin/users', {
        method: 'POST',
        body: JSON.stringify({ display_name: displayName, role }),
      }),
    updateUser: (userId: string, data: { display_name?: string; status?: string }) =>
      fetchJSON<AppUser>(`/admin/users/${userId}`, {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    deleteUser: (userId: string) =>
      fetchJSON<void>(`/admin/users/${userId}`, { method: 'DELETE' }),
    addRole: (userId: string, role: string) =>
      fetchJSON<void>(`/admin/users/${userId}/roles`, {
        method: 'POST',
        body: JSON.stringify({ role }),
      }),
    removeRole: (userId: string, role: string) =>
      fetchJSON<void>(`/admin/users/${userId}/roles/${role}`, { method: 'DELETE' }),
    removeIdentity: (userId: string, identityId: string) =>
      fetchJSON<void>(`/admin/users/${userId}/identities/${identityId}`, { method: 'DELETE' }),
    createInvite: (userId: string) =>
      fetchJSON<{ invite: InviteInfo; invite_url: string }>(`/admin/users/${userId}/invites`, {
        method: 'POST',
        body: '{}',
      }),
    listInvites: (userId: string) =>
      fetchJSON<InviteInfo[]>(`/admin/users/${userId}/invites`),
    deleteInvite: (userId: string, inviteId: string) =>
      fetchJSON<void>(`/admin/users/${userId}/invites/${inviteId}`, { method: 'DELETE' }),
    listUserInstitutions: (userId: string) =>
      fetchJSON<string[]>(`/admin/users/${userId}/institutions`),
    addUserInstitution: (userId: string, institution: string) =>
      fetchJSON<void>(`/admin/users/${userId}/institutions`, {
        method: 'POST',
        body: JSON.stringify({ institution }),
      }),
    removeUserInstitution: (userId: string, institution: string) =>
      fetchJSON<void>(`/admin/users/${userId}/institutions/${encodeURIComponent(institution)}`, { method: 'DELETE' }),
    getOIDCConfig: () => fetchJSON<OIDCConfig>('/admin/oidc-config'),
    updateOIDCConfig: (data: { oidc_issuer?: string; oidc_client_id?: string; oidc_client_secret?: string }) =>
      fetchJSON<void>('/admin/oidc-config', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),

    // API keys
    listAPIKeys: () => fetchJSON<APIKeyInfo[]>('/admin/api-keys'),
    createAPIKey: (data: { name: string; roles: string[]; idle_timeout_s?: number; expires_at?: string }) =>
      fetchJSON<APIKeyCreateResponse>('/admin/api-keys', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    revokeAPIKey: (keyId: string) =>
      fetchJSON<void>(`/admin/api-keys/${keyId}/revoke`, { method: 'POST' }),
    deleteAPIKey: (keyId: string) =>
      fetchJSON<void>(`/admin/api-keys/${keyId}`, { method: 'DELETE' }),
  },

  // Entity-scoped invoice coding + expense tracking (entityType: 'grant' | 'subaward')
  invoiceCoding: {
    listGrantInvoices: (grantId: string) => fetchJSON<Invoice[]>(`/grants/${grantId}/invoices`),
    analytics: (grantId: string) => fetchJSON<InvoiceAnalytics>(`/grants/${grantId}/invoice-analytics`),

    list: (et: string, eid: string) => fetchJSON<Invoice[]>(`/institution-rates/${et}/${eid}/invoices`),
    get: (et: string, eid: string, invoiceId: string) =>
      fetchJSON<InvoiceDetail>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}`),
    create: (et: string, eid: string, data: Partial<Invoice>) =>
      fetchJSON<Invoice>(`/institution-rates/${et}/${eid}/invoices`, { method: 'POST', body: JSON.stringify(data) }),
    update: (et: string, eid: string, invoiceId: string, data: Partial<Invoice>) =>
      fetchJSON<Invoice>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}`, { method: 'PUT', body: JSON.stringify(data) }),
    remove: (et: string, eid: string, invoiceId: string) =>
      fetchJSON<void>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}`, { method: 'DELETE' }),
    upload: async (et: string, eid: string, invoiceId: string, file: File) => {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch(`${API_BASE}/institution-rates/${et}/${eid}/invoices/${invoiceId}/upload`, {
        method: 'POST', body: form, credentials: 'include',
      });
      if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || `Upload failed: ${res.status}`);
      return res.json() as Promise<Document>;
    },
    code: (et: string, eid: string, invoiceId: string, userPrompt?: string) =>
      fetchJSON<{ run_id: string; status: string }>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/code`, {
        method: 'POST', body: JSON.stringify({ user_prompt: userPrompt || '' }),
      }),
    finalizeCoding: (et: string, eid: string, invoiceId: string) =>
      fetchJSON<{ coding_status: string }>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/finalize-coding`, { method: 'POST' }),
    // Payment approval — server restricts to admin/grant_admin.
    setPaymentStatus: (et: string, eid: string, invoiceId: string, status: string) =>
      fetchJSON<{ status: string }>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/status`, {
        method: 'PATCH', body: JSON.stringify({ status }),
      }),
    setCodingStatus: (et: string, eid: string, invoiceId: string, coding_status: string) =>
      fetchJSON<{ coding_status: string }>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/coding-status`, {
        method: 'PATCH', body: JSON.stringify({ coding_status }),
      }),

    // Expense lines
    createExpense: (et: string, eid: string, invoiceId: string, data: Partial<InvoiceExpense>) =>
      fetchJSON<InvoiceExpense>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/expenses`, { method: 'POST', body: JSON.stringify(data) }),
    updateExpense: (et: string, eid: string, invoiceId: string, expenseId: string, data: Partial<InvoiceExpense>) =>
      fetchJSON<InvoiceExpense>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/expenses/${expenseId}`, { method: 'PUT', body: JSON.stringify(data) }),
    deleteExpense: (et: string, eid: string, invoiceId: string, expenseId: string) =>
      fetchJSON<void>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/expenses/${expenseId}`, { method: 'DELETE' }),
    setExpenseWBS: (et: string, eid: string, invoiceId: string, expenseId: string, allocations: InvoiceExpenseWBS[]) =>
      fetchJSON<InvoiceExpenseWBS[]>(`/institution-rates/${et}/${eid}/invoices/${invoiceId}/expenses/${expenseId}/wbs`, { method: 'PUT', body: JSON.stringify(allocations) }),

    // AI coding run polling (reuses processing-runs infra)
    listRuns: (et: string, eid: string) => fetchJSON<DocumentProcessingRun[]>(`/institution-rates/${et}/${eid}/processing-runs`),
    getRun: (et: string, eid: string, runId: string) => fetchJSON<DocumentProcessingRun>(`/institution-rates/${et}/${eid}/processing-runs/${runId}`),
  },
};
