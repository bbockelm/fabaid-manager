'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  SOWConfig,
  StatementOfWork,
  SOWPersonnelDescription,
  SOWLineItemDescription,
  Personnel,
  BudgetLineItem,
  Subaward,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { useState, useEffect, useCallback } from 'react';

const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};
const TYPICAL_LINE_TYPES = new Set(['personnel', 'fringe', 'travel', 'supplies']);

export default function SOWPage() {
  const { grantId, isLoading: grantLoading } = useGrant();
  const [activeTab, setActiveTab] = useState<'subawards' | 'template'>('subawards');

  if (grantLoading) return <div className="p-4">Loading...</div>;
  if (!grantId)
    return (
      <div className="p-4 text-gray-500">
        No grant selected. Create one in Settings first.
      </div>
    );

  return (
    <div className="max-w-5xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">Statements of Work</h1>
        <p className="text-sm text-gray-500 mt-1">
          Manage SOW documents for each subaward institution, and configure the
          shared document template.
        </p>
      </div>

      {/* Tab bar */}
      <div className="flex border-b">
        {(['subawards', 'template'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${
              activeTab === tab
                ? 'border-nsf-blue text-nsf-blue'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            {tab === 'subawards' ? 'SOW by Institution' : 'Document Template'}
          </button>
        ))}
      </div>

      {activeTab === 'subawards' ? (
        <SubawardSOWList grantId={grantId} />
      ) : (
        <TemplateConfig grantId={grantId} />
      )}
    </div>
  );
}

/* ═══════════════════════════════════════════════════════════════════════════════
   SOW by Subaward — lists each subaward, its SOWs, and inline editors
   ═══════════════════════════════════════════════════════════════════════════════ */

function SubawardSOWList({ grantId }: { grantId: string }) {
  const { isSubawardAdmin, permittedInstitutions } = useAuth();
  const { data: subawards, isLoading } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId),
  });

  const visibleSubawards = isSubawardAdmin && permittedInstitutions.length > 0
    ? (subawards ?? []).filter((s) => permittedInstitutions.includes(s.institution))
    : subawards;

  if (isLoading) return <div className="text-sm text-gray-500">Loading subawards...</div>;
  if (!visibleSubawards?.length)
    return <div className="text-sm text-gray-400">No subawards yet. Add one from the Institutions page.</div>;

  return (
    <div className="space-y-6">
      {visibleSubawards.map((sub) => (
        <SubawardSOWPanel key={sub.id} grantId={grantId} subaward={sub} />
      ))}
    </div>
  );
}

function SubawardSOWPanel({ grantId, subaward }: { grantId: string; subaward: Subaward }) {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [expandedSOW, setExpandedSOW] = useState<string | null>(null);

  const { data: sows, isLoading } = useQuery({
    queryKey: ['sow', grantId, subaward.id],
    queryFn: () => api.sow.list(grantId, subaward.id),
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<StatementOfWork>) =>
      api.sow.create(grantId, subaward.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sow', grantId, subaward.id] });
      setShowForm(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (sowId: string) => api.sow.delete(grantId, subaward.id, sowId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sow', grantId, subaward.id] });
      setExpandedSOW(null);
    },
  });

  return (
    <section className="bg-white rounded-lg border">
      <div className="px-6 py-4 border-b flex items-center justify-between">
        <div>
          <h2 className="font-semibold text-lg">{subaward.institution}</h2>
          <p className="text-xs text-gray-500">
            PI: {subaward.pi_name} &middot; ${subaward.total_amount.toLocaleString()}
          </p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-sm px-3 py-1.5 bg-nsf-light text-white rounded hover:bg-nsf-blue"
        >
          {showForm ? 'Cancel' : '+ New SOW'}
        </button>
      </div>

      {showForm && (
        <div className="p-4 border-b bg-gray-50">
          <SOWCreateForm
            onSubmit={(data) => createMutation.mutate(data)}
            isLoading={createMutation.isPending}
            subawardStart={subaward.start_date}
            subawardEnd={subaward.end_date}
          />
        </div>
      )}

      {isLoading ? (
        <div className="p-4 text-sm text-gray-500">Loading...</div>
      ) : !sows?.length ? (
        <div className="p-6 text-center text-gray-400 text-sm">
          No statements of work yet.
        </div>
      ) : (
        <div className="divide-y">
          {sows.map((sow) => (
            <div key={sow.id}>
              {/* SOW row */}
              <div className="px-6 py-3 flex items-center justify-between hover:bg-gray-50">
                <div className="flex items-center gap-4 flex-1 min-w-0">
                  <button
                    onClick={() =>
                      setExpandedSOW(expandedSOW === sow.id ? null : sow.id)
                    }
                    className="text-gray-400 hover:text-gray-600 flex-shrink-0"
                    title={expandedSOW === sow.id ? 'Collapse' : 'Edit descriptions'}
                  >
                    {expandedSOW === sow.id ? '▼' : '▶'}
                  </button>
                  <div className="min-w-0">
                    <span className="font-medium text-sm">
                      {YEAR_LABELS[sow.fiscal_year] ?? `Year ${sow.fiscal_year}`}
                    </span>
                    <span className="text-xs text-gray-500 ml-2">
                      {sow.period_start} &mdash; {sow.period_end}
                    </span>
                  </div>
                  <span className="text-sm text-gray-700">
                    {sow.budget_id ? 'Budget linked' : 'No budget linked'}
                  </span>
                  <StatusBadge status={sow.status} />
                </div>
                <div className="flex items-center gap-3 flex-shrink-0">
                  <button
                    onClick={() =>
                      setExpandedSOW(expandedSOW === sow.id ? null : sow.id)
                    }
                    className="text-sm text-nsf-light hover:underline font-medium"
                  >
                    Edit Descriptions
                  </button>
                  <a
                    href={api.sow.renderUrl(grantId, subaward.id, sow.id)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-nsf-light hover:underline font-medium"
                  >
                    Preview
                  </a>
                  <button
                    onClick={() => {
                      if (confirm(`Delete SOW for ${YEAR_LABELS[sow.fiscal_year] ?? `Year ${sow.fiscal_year}`}? This cannot be undone.`)) {
                        deleteMutation.mutate(sow.id);
                      }
                    }}
                    className="text-sm text-red-500 hover:text-red-700 font-medium"
                  >
                    Delete
                  </button>
                </div>
              </div>

              {/* Expanded inline editor */}
              {expandedSOW === sow.id && (
                <div className="px-6 pb-6 bg-gray-50 border-t">
                  <SOWDescriptionEditor
                    grantId={grantId}
                    subawardId={subaward.id}
                    sow={sow}
                  />
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

/* ─── SOW Description Editor (inline, per-SOW) ─── */

function SOWDescriptionEditor({
  grantId,
  subawardId,
  sow,
}: {
  grantId: string;
  subawardId: string;
  sow: StatementOfWork;
}) {
  const { data: personnel } = useQuery({
    queryKey: ['personnel', grantId],
    queryFn: () => api.personnel.list(grantId),
    enabled: !!grantId,
  });

  const { data: subawardBudgets } = useQuery({
    queryKey: ['institution-budgets', 'subaward', subawardId],
    queryFn: () => api.institutionBudgets.list('subaward', subawardId),
    enabled: !!subawardId,
  });

  const matchingBudget = subawardBudgets?.find(
    (b) => b.fiscal_year === sow.fiscal_year && b.is_latest
  );

  const { data: lineItems } = useQuery({
    queryKey: ['budget-line-items', 'subaward', subawardId, matchingBudget?.id],
    queryFn: () =>
      api.budgetLineItems.list('subaward', subawardId, matchingBudget!.id),
    enabled: !!matchingBudget,
  });

  const { data: persDescs } = useQuery({
    queryKey: ['sow-pers-descs', grantId, subawardId, sow.id],
    queryFn: () =>
      api.sow.listPersonnelDescriptions(grantId, subawardId, sow.id),
    enabled: !!grantId,
  });

  const { data: liDescs } = useQuery({
    queryKey: ['sow-li-descs', grantId, subawardId, sow.id],
    queryFn: () =>
      api.sow.listLineItemDescriptions(grantId, subawardId, sow.id),
    enabled: !!grantId,
  });

  const personnelInBudget = (personnel ?? []).filter((p) =>
    (lineItems ?? []).some(
      (li) => li.line_type === 'personnel' && li.personnel_id === p.id
    )
  );

  const atypicalLineItems = (lineItems ?? []).filter(
    (li) => !TYPICAL_LINE_TYPES.has(li.line_type)
  );

  return (
    <div className="pt-4 space-y-6">
      {/* Personnel Descriptions */}
      <div className="space-y-3">
        <div>
          <h3 className="font-semibold text-sm">Personnel Descriptions</h3>
          <p className="text-xs text-gray-500">
            Describe what each funded person will do during this period. Use
            markdown bullet points (lines starting with &ldquo;- &rdquo;).
          </p>
        </div>

        {!matchingBudget ? (
          <p className="text-sm text-amber-600 italic">
            No matching budget found for Year {sow.fiscal_year}. Create a budget
            in the Institutions page first.
          </p>
        ) : personnelInBudget.length === 0 ? (
          <p className="text-sm text-gray-400 italic">
            No personnel with salary line items found for this fiscal year.
          </p>
        ) : (
          personnelInBudget.map((p) => (
            <PersonnelDescriptionEditor
              key={p.id}
              person={p}
              grantId={grantId}
              subawardId={subawardId}
              sowId={sow.id}
              existing={persDescs?.find((d) => d.personnel_id === p.id)}
            />
          ))
        )}
      </div>

      {/* Atypical Line Item Descriptions */}
      {atypicalLineItems.length > 0 && (
        <div className="space-y-3">
          <div>
            <h3 className="font-semibold text-sm">Special Line Item Descriptions</h3>
            <p className="text-xs text-gray-500">
              Describe non-standard budget items (equipment, contractual,
              participant support, tuition, other).
            </p>
          </div>

          {atypicalLineItems.map((li) => (
            <LineItemDescriptionEditor
              key={li.id}
              lineItem={li}
              grantId={grantId}
              subawardId={subawardId}
              sowId={sow.id}
              existing={liDescs?.find((d) => d.line_item_id === li.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

/* ─── Personnel Description Editor ─── */

function PersonnelDescriptionEditor({
  person,
  grantId,
  subawardId,
  sowId,
  existing,
}: {
  person: Personnel;
  grantId: string;
  subawardId: string;
  sowId: string;
  existing?: SOWPersonnelDescription;
}) {
  const queryClient = useQueryClient();
  const [md, setMd] = useState(existing?.description_md ?? '');
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setMd(existing?.description_md ?? '');
    setDirty(false);
  }, [existing]);

  const mutation = useMutation({
    mutationFn: (text: string) =>
      api.sow.upsertPersonnelDescription(grantId, subawardId, sowId, {
        personnel_id: person.id,
        description_md: text,
        sort_order: 0,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['sow-pers-descs', grantId, subawardId, sowId],
      });
      setDirty(false);
    },
  });

  const handleSave = useCallback(() => {
    mutation.mutate(md);
  }, [md, mutation]);

  return (
    <div className="bg-white border rounded p-4 space-y-2">
      <div className="flex justify-between items-center">
        <div>
          <span className="font-medium text-sm">{person.name}</span>
          {person.title && (
            <span className="text-gray-500 text-xs ml-2">({person.title})</span>
          )}
          {person.role && (
            <span className="text-gray-400 text-xs ml-1">&middot; {person.role}</span>
          )}
        </div>
        <button
          onClick={handleSave}
          disabled={mutation.isPending || !dirty}
          className="px-3 py-1 bg-nsf-blue text-white rounded text-xs hover:bg-blue-700 disabled:opacity-50"
        >
          {mutation.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
      <textarea
        className="block w-full rounded border-gray-300 shadow-sm text-sm font-mono leading-relaxed"
        rows={8}
        placeholder={`- Contribute to the overall direction and guidance of the project\n  - example sub-bullet with details\n- Serve as area lead for...\n- Participate in team activities and meetings\n- Coordinate with collaborators on...`}
        value={md}
        onChange={(e) => {
          setMd(e.target.value);
          setDirty(true);
        }}
        onKeyDown={(e) => {
          if ((e.ctrlKey || e.metaKey) && e.key === 's') {
            e.preventDefault();
            if (dirty) mutation.mutate(md);
          }
        }}
      />
      <p className="text-xs text-gray-400">
        Use markdown bullet points. Each line starting with &ldquo;-&rdquo; becomes a bullet.
        Indent with spaces for sub-bullets. Ctrl+S to save.
      </p>
      {mutation.isError && (
        <p className="text-red-500 text-xs">
          Error: {(mutation.error as Error).message}
        </p>
      )}
    </div>
  );
}

/* ─── Line Item Description Editor ─── */

function LineItemDescriptionEditor({
  lineItem,
  grantId,
  subawardId,
  sowId,
  existing,
}: {
  lineItem: BudgetLineItem;
  grantId: string;
  subawardId: string;
  sowId: string;
  existing?: SOWLineItemDescription;
}) {
  const queryClient = useQueryClient();
  const [md, setMd] = useState(existing?.description_md ?? '');
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setMd(existing?.description_md ?? '');
    setDirty(false);
  }, [existing]);

  const mutation = useMutation({
    mutationFn: (text: string) =>
      api.sow.upsertLineItemDescription(grantId, subawardId, sowId, {
        line_item_id: lineItem.id,
        description_md: text,
        sort_order: 0,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['sow-li-descs', grantId, subawardId, sowId],
      });
      setDirty(false);
    },
  });

  const handleSave = useCallback(() => {
    mutation.mutate(md);
  }, [md, mutation]);

  const typeLabel = lineItem.line_type.replace(/_/g, ' ');

  return (
    <div className="bg-white border rounded p-4 space-y-2">
      <div className="flex justify-between items-center">
        <div>
          <span className="font-medium text-sm capitalize">{typeLabel}</span>
          {lineItem.description && (
            <span className="text-gray-500 text-xs ml-2">
              ({lineItem.description})
            </span>
          )}
          <span className="text-gray-400 text-xs ml-2">
            ${lineItem.amount?.toLocaleString() ?? '0'}
          </span>
        </div>
        <button
          onClick={handleSave}
          disabled={mutation.isPending || !dirty}
          className="px-3 py-1 bg-nsf-blue text-white rounded text-xs hover:bg-blue-700 disabled:opacity-50"
        >
          {mutation.isPending ? 'Saving...' : 'Save'}
        </button>
      </div>
      <textarea
        className="block w-full rounded border-gray-300 shadow-sm text-sm font-mono leading-relaxed"
        rows={5}
        placeholder="Describe this line item for the SOW..."
        value={md}
        onChange={(e) => {
          setMd(e.target.value);
          setDirty(true);
        }}
        onKeyDown={(e) => {
          if ((e.ctrlKey || e.metaKey) && e.key === 's') {
            e.preventDefault();
            if (dirty) mutation.mutate(md);
          }
        }}
      />
      {mutation.isError && (
        <p className="text-red-500 text-xs">
          Error: {(mutation.error as Error).message}
        </p>
      )}
    </div>
  );
}

/* ─── SOW Create Form ─── */

function SOWCreateForm({
  onSubmit,
  isLoading,
  subawardStart,
  subawardEnd,
}: {
  onSubmit: (data: Partial<StatementOfWork>) => void;
  isLoading: boolean;
  subawardStart: string;
  subawardEnd: string;
}) {
  const [form, setForm] = useState({
    fiscal_year: 1,
    period_start: subawardStart,
    period_end: '',
    scope_text: '',
    status: 'draft',
  });

  const handleYearChange = (year: number) => {
    const start = new Date(subawardStart);
    const periodStart = new Date(start);
    periodStart.setFullYear(start.getFullYear() + (year - 1));
    const periodEnd = new Date(periodStart);
    periodEnd.setFullYear(periodStart.getFullYear() + 1);
    periodEnd.setDate(periodEnd.getDate() - 1);

    const end = new Date(subawardEnd);
    const finalEnd = periodEnd > end ? end : periodEnd;

    setForm({
      ...form,
      fiscal_year: year,
      period_start: periodStart.toISOString().slice(0, 10),
      period_end: finalEnd.toISOString().slice(0, 10),
    });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(form);
      }}
      className="space-y-3"
    >
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Fiscal Year
          </label>
          <select
            value={form.fiscal_year}
            onChange={(e) => handleYearChange(parseInt(e.target.value))}
            className="w-full border rounded px-2 py-1.5 text-sm"
          >
            {PROJECT_YEARS.map((y) => (
              <option key={y} value={y}>
                {YEAR_LABELS[y]}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Period Start
          </label>
          <input
            type="date"
            required
            value={form.period_start}
            onChange={(e) => setForm({ ...form, period_start: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Period End
          </label>
          <input
            type="date"
            required
            value={form.period_end}
            onChange={(e) => setForm({ ...form, period_end: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm"
          />
        </div>
        <div className="col-span-2">
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Scope (optional)
          </label>
          <input
            type="text"
            value={form.scope_text}
            onChange={(e) => setForm({ ...form, scope_text: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm"
            placeholder="Brief description of work scope"
          />
        </div>
      </div>
      <button
        type="submit"
        disabled={isLoading}
        className="px-3 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50"
      >
        {isLoading ? 'Creating...' : 'Create SOW'}
      </button>
    </form>
  );
}

/* ─── Status Badge ─── */

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    draft: 'bg-yellow-100 text-yellow-800',
    submitted: 'bg-blue-100 text-blue-800',
    active: 'bg-green-100 text-green-800',
    signed: 'bg-green-100 text-green-800',
    expired: 'bg-gray-100 text-gray-600',
  };
  return (
    <span
      className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${
        colors[status] ?? 'bg-gray-100 text-gray-600'
      }`}
    >
      {status}
    </span>
  );
}

/* ═══════════════════════════════════════════════════════════════════════════════
   Document Template Configuration
   ═══════════════════════════════════════════════════════════════════════════════ */

function TemplateConfig({ grantId }: { grantId: string }) {
  const queryClient = useQueryClient();

  const { data: config, isLoading } = useQuery({
    queryKey: ['sow-config', grantId],
    queryFn: () => api.sowConfig.get(grantId),
  });

  const [form, setForm] = useState({
    header_title: '',
    header_subtitle: '',
    project_name: '',
    intro_template: '',
    costs_template: '',
  });

  const [signers, setSigners] = useState<
    { name: string; title: string; affiliation: string }[]
  >([]);

  useEffect(() => {
    if (config) {
      setForm({
        header_title: config.header_title || '',
        header_subtitle: config.header_subtitle || '',
        project_name: config.project_name || '',
        intro_template: config.intro_template || '',
        costs_template: config.costs_template || '',
      });
      try {
        setSigners(JSON.parse(config.concurrence_signers || '[]'));
      } catch {
        setSigners([]);
      }
    }
  }, [config]);

  const saveMutation = useMutation({
    mutationFn: (data: Partial<SOWConfig>) =>
      api.sowConfig.upsert(grantId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sow-config', grantId] });
    },
  });

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    saveMutation.mutate({
      ...form,
      concurrence_signers: JSON.stringify(signers),
    });
  };

  const addSigner = () => {
    setSigners([...signers, { name: '', title: '', affiliation: '' }]);
  };

  const removeSigner = (idx: number) => {
    setSigners(signers.filter((_, i) => i !== idx));
  };

  const updateSigner = (idx: number, field: string, value: string) => {
    const updated = [...signers];
    updated[idx] = { ...updated[idx], [field]: value };
    setSigners(updated);
  };

  if (isLoading) return <div className="text-sm text-gray-500">Loading...</div>;

  return (
    <form onSubmit={handleSave} className="bg-white p-6 rounded-lg border space-y-6">
      {/* Header Section */}
      <fieldset className="space-y-3">
        <legend className="font-semibold text-lg">Document Header</legend>
        <p className="text-xs text-gray-500">
          These fields appear at the top of the generated SOW document.
        </p>

        <label className="block">
          <span className="text-sm font-medium">Header Title</span>
          <input
            className="mt-1 block w-full rounded border-gray-300 shadow-sm focus:ring-nsf-blue focus:border-nsf-blue text-sm"
            placeholder='e.g., Statement of Work for the {{.Institution}}'
            value={form.header_title}
            onChange={(e) => setForm({ ...form, header_title: e.target.value })}
          />
        </label>

        <label className="block">
          <span className="text-sm font-medium">Header Subtitle</span>
          <input
            className="mt-1 block w-full rounded border-gray-300 shadow-sm focus:ring-nsf-blue focus:border-nsf-blue text-sm"
            placeholder="e.g., for Activities as part of the"
            value={form.header_subtitle}
            onChange={(e) =>
              setForm({ ...form, header_subtitle: e.target.value })
            }
          />
        </label>

        <label className="block">
          <span className="text-sm font-medium">Project Name</span>
          <input
            className="mt-1 block w-full rounded border-gray-300 shadow-sm focus:ring-nsf-blue focus:border-nsf-blue text-sm"
            placeholder="e.g., Federated AI for Discovery (FabAID)"
            value={form.project_name}
            onChange={(e) =>
              setForm({ ...form, project_name: e.target.value })
            }
          />
        </label>
      </fieldset>

      {/* Intro Template */}
      <fieldset className="space-y-3">
        <legend className="font-semibold text-lg">Introduction Section</legend>
        <p className="text-xs text-gray-500">
          Go template for the intro paragraph. Available variables:{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.Institution}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.PIName}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.GrantTitle}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.AwardNumber}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.PeriodStart}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.PeriodEnd}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.FiscalYear}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{dollar .BudgetAmount}}'}
          </code>
        </p>
        <textarea
          className="mt-1 block w-full rounded border-gray-300 shadow-sm focus:ring-nsf-blue focus:border-nsf-blue text-sm font-mono"
          rows={6}
          placeholder={`<p>This Statement of Work (SOW) provides the yearly details...</p>`}
          value={form.intro_template}
          onChange={(e) =>
            setForm({ ...form, intro_template: e.target.value })
          }
        />
      </fieldset>

      {/* Costs Template */}
      <fieldset className="space-y-3">
        <legend className="font-semibold text-lg">
          Costs and Funding Section
        </legend>
        <p className="text-xs text-gray-500">
          Go template for the costs paragraph. Leave empty for default text.
        </p>
        <textarea
          className="mt-1 block w-full rounded border-gray-300 shadow-sm focus:ring-nsf-blue focus:border-nsf-blue text-sm font-mono"
          rows={6}
          placeholder="Leave empty for default boilerplate text"
          value={form.costs_template}
          onChange={(e) =>
            setForm({ ...form, costs_template: e.target.value })
          }
        />
      </fieldset>

      {/* Concurrence Signers */}
      <fieldset className="space-y-3">
        <legend className="font-semibold text-lg">Signature Blocks</legend>
        <p className="text-xs text-gray-500">
          People who will sign the SOW. Each signer gets a signature line.
          Signer fields support template variables:{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.PIName}}'}
          </code>
          ,{' '}
          <code className="text-xs bg-gray-100 px-1 rounded">
            {'{{.Institution}}'}
          </code>
          , etc.
        </p>

        {signers.map((signer, idx) => (
          <div
            key={idx}
            className="flex gap-2 items-start bg-gray-50 p-3 rounded"
          >
            <div className="flex-1 space-y-2">
              <input
                className="block w-full rounded border-gray-300 shadow-sm text-sm"
                placeholder="Name"
                value={signer.name}
                onChange={(e) => updateSigner(idx, 'name', e.target.value)}
              />
              <input
                className="block w-full rounded border-gray-300 shadow-sm text-sm"
                placeholder="Title (e.g., Principal Investigator)"
                value={signer.title}
                onChange={(e) => updateSigner(idx, 'title', e.target.value)}
              />
              <input
                className="block w-full rounded border-gray-300 shadow-sm text-sm"
                placeholder="Affiliation (e.g., Princeton University)"
                value={signer.affiliation}
                onChange={(e) =>
                  updateSigner(idx, 'affiliation', e.target.value)
                }
              />
            </div>
            <button
              type="button"
              onClick={() => removeSigner(idx)}
              className="text-red-500 hover:text-red-700 text-sm mt-1"
            >
              Remove
            </button>
          </div>
        ))}

        <button
          type="button"
          onClick={addSigner}
          className="text-sm text-nsf-light hover:underline"
        >
          + Add Signer
        </button>
      </fieldset>

      {/* Save */}
      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={saveMutation.isPending}
          className="px-4 py-2 bg-nsf-blue text-white rounded hover:bg-blue-700 disabled:opacity-50 text-sm"
        >
          {saveMutation.isPending ? 'Saving...' : 'Save Template'}
        </button>
        {saveMutation.isSuccess && (
          <span className="text-green-600 text-sm">Saved!</span>
        )}
        {saveMutation.isError && (
          <span className="text-red-500 text-sm">
            Error: {(saveMutation.error as Error).message}
          </span>
        )}
      </div>
    </form>
  );
}
