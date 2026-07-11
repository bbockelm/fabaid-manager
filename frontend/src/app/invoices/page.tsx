'use client';

import React, { useMemo, useState, useEffect, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api, Invoice, InvoiceExpense, InvoiceExpenseWBS, InvoiceAnalytics,
  WBSArea, Subaward, Grant, Personnel,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';

function fmt$(n: number | undefined | null) {
  if (n === undefined || n === null) return '—';
  return `$${Math.round(n).toLocaleString()}`;
}
function today() { return new Date().toISOString().slice(0, 10); }

// Expense categories (mirrors backend expenseCategories). 'equipment' == capital.
const CATEGORIES = [
  'personnel', 'fringe', 'travel', 'equipment', 'supplies', 'contractual',
  'participant_support', 'tuition', 'indirect', 'other', 'uncategorized',
];
const PERSONNEL_CATEGORIES = ['personnel', 'fringe'];

const CODING_BADGE: Record<string, string> = {
  uncoded: 'bg-gray-100 text-gray-600', draft: 'bg-amber-100 text-amber-800', final: 'bg-green-100 text-green-800',
};
const PAY_BADGE: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-600', approved: 'bg-blue-100 text-blue-700',
  rejected: 'bg-red-100 text-red-700', paid: 'bg-green-100 text-green-800',
};

function wbsPct(e: InvoiceExpense): number {
  return (e.wbs ?? []).reduce((s, w) => s + w.allocation_percent, 0);
}
function uncategorizedForExpenses(expenses: InvoiceExpense[]) {
  let wbs = 0, category = 0;
  for (const e of expenses) {
    const rem = e.amount * Math.max(0, 100 - wbsPct(e)) / 100;
    if (rem > 0.005) wbs += rem;
    if (e.line_type === 'uncategorized' || !e.line_type) category += e.amount;
  }
  return { wbs, category };
}

type EditorState = { mode: 'new' } | { mode: 'edit'; invoiceId: string } | null;

export default function InvoicesPage() {
  const { grantId } = useGrant();

  const invoicesQ = useQuery({ queryKey: ['grant-invoices', grantId], queryFn: () => api.invoiceCoding.listGrantInvoices(grantId!), enabled: !!grantId });
  const analyticsQ = useQuery({ queryKey: ['invoice-analytics', grantId], queryFn: () => api.invoiceCoding.analytics(grantId!), enabled: !!grantId });
  const subawardsQ = useQuery({ queryKey: ['subawards', grantId], queryFn: () => api.subawards.list(grantId!), enabled: !!grantId });
  const grantQ = useQuery({ queryKey: ['grant', grantId], queryFn: () => api.grants.get(grantId!), enabled: !!grantId });

  const [editor, setEditor] = useState<EditorState>(null);

  const invoices = invoicesQ.data ?? [];
  const analytics = analyticsQ.data;

  const instName = useMemo(() => {
    const m = new Map<string, string>();
    if (grantQ.data) m.set(`grant:${grantQ.data.id}`, grantQ.data.institution);
    for (const s of subawardsQ.data ?? []) m.set(`subaward:${s.id}`, s.institution);
    return m;
  }, [grantQ.data, subawardsQ.data]);
  const nameOf = (inv: Invoice) => instName.get(`${inv.entity_type}:${inv.entity_id}`) ?? inv.entity_id.slice(0, 8);

  if (!grantId) return <div className="p-8 text-gray-500">Select a grant to view invoices.</div>;

  const uncatTotal = (analytics?.uncategorized.category ?? 0) + (analytics?.uncategorized.wbs ?? 0);
  const nBehind = analytics?.behind.length ?? 0;
  // Dollars on invoices not yet finalized-coded (still need coding work).
  const uncodedTotal = invoices.filter(i => i.coding_status !== 'final').reduce((s, i) => s + i.amount, 0);

  return (
    <div className="p-6 space-y-6 max-w-6xl">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-nsf-blue">Invoices &amp; Expense Tracking</h1>
        <button onClick={() => setEditor({ mode: 'new' })} className="px-3 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue">+ New Invoice</button>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-3">
        <Card label="Invoices" value={String(invoices.length)} />
        <Card label="Finalized actuals" value={fmt$(analytics?.total_actual)} />
        <Card label="Uncoded $ remaining" value={fmt$(uncodedTotal)} tone={uncodedTotal > 0.5 ? 'warn' : undefined} />
        <Card label="Behind on invoicing" value={String(nBehind)} tone={nBehind > 0 ? 'warn' : undefined} />
        <Card label="Uncategorized" value={fmt$(uncatTotal)} tone={uncatTotal > 0.5 ? 'danger' : undefined} />
      </div>

      {nBehind > 0 && (
        <section className="border border-amber-300 bg-amber-50 rounded-lg p-4">
          <h2 className="text-sm font-semibold text-amber-800 mb-2">Institutions behind on invoicing</h2>
          <ul className="text-sm text-amber-800 space-y-1">
            {analytics!.behind.map((b, i) => (
              <li key={i}><span className="font-medium">{b.institution}</span>{' '}
                {b.last_period_end ? `— last billed through ${b.last_period_end} (${b.months_since_last?.toFixed(1)} mo ago)` : '— no invoices submitted yet'}</li>
            ))}
          </ul>
        </section>
      )}

      {analytics && analytics.burn.length > 0 && <BurnTable analytics={analytics} />}

      {analytics && analytics.total_actual > 0 && (
        <div className="grid md:grid-cols-2 gap-4">
          <Breakdown title="Actuals by WBS area" rows={analytics.by_wbs.map(r => ({ label: r.name, amount: r.amount, flag: r.uncategorized }))} />
          <Breakdown title="Actuals by category" rows={analytics.by_category.map(r => ({ label: r.line_type, amount: r.amount, flag: r.uncategorized }))} />
        </div>
      )}

      <section>
        <h2 className="text-sm font-semibold text-gray-700 mb-2">All invoices</h2>
        <div className="overflow-x-auto border rounded-lg">
          <table className="min-w-full text-sm">
            <thead className="bg-gray-50 text-gray-500 text-xs">
              <tr>
                <th className="px-3 py-2 text-left">Institution</th><th className="px-3 py-2 text-left">Number</th>
                <th className="px-3 py-2 text-left">Period</th><th className="px-3 py-2 text-right">Amount</th>
                <th className="px-3 py-2 text-center">Payment</th><th className="px-3 py-2 text-center">Coding</th><th></th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {invoices.map(inv => (
                <tr key={inv.id} className="hover:bg-gray-50">
                  <td className="px-3 py-2">{nameOf(inv)}</td>
                  <td className="px-3 py-2">{inv.invoice_number || '—'}</td>
                  <td className="px-3 py-2 text-gray-500">{inv.period_start && inv.period_end ? `${inv.period_start} → ${inv.period_end}` : inv.invoice_date}</td>
                  <td className="px-3 py-2 text-right font-medium">{fmt$(inv.amount)}</td>
                  <td className="px-3 py-2 text-center"><span className={`px-2 py-0.5 rounded text-xs ${PAY_BADGE[inv.status] ?? ''}`}>{inv.status}</span></td>
                  <td className="px-3 py-2 text-center"><span className={`px-2 py-0.5 rounded text-xs ${CODING_BADGE[inv.coding_status] ?? ''}`}>{inv.coding_status}</span></td>
                  <td className="px-3 py-2 text-right"><button onClick={() => setEditor({ mode: 'edit', invoiceId: inv.id })} className="text-nsf-light hover:underline text-xs">Open</button></td>
                </tr>
              ))}
              {invoices.length === 0 && <tr><td colSpan={7} className="px-3 py-8 text-center text-gray-400">No invoices yet.</td></tr>}
            </tbody>
          </table>
        </div>
      </section>

      {editor && grantQ.data && (
        <InvoiceEditor
          mode={editor.mode} invoiceId={editor.mode === 'edit' ? editor.invoiceId : undefined}
          grant={grantQ.data} subawards={subawardsQ.data ?? []} grantId={grantId}
          onClose={() => setEditor(null)}
        />
      )}
    </div>
  );
}

function Card({ label, value, tone }: { label: string; value: string; tone?: 'warn' | 'danger' }) {
  const cls = tone === 'danger' ? 'border-red-300 bg-red-50' : tone === 'warn' ? 'border-amber-300 bg-amber-50' : 'border-gray-200 bg-white';
  const val = tone === 'danger' ? 'text-red-700' : tone === 'warn' ? 'text-amber-700' : 'text-nsf-blue';
  return <div className={`rounded-lg border p-3 ${cls}`}><div className="text-xs text-gray-500">{label}</div><div className={`text-xl font-bold ${val}`}>{value}</div></div>;
}

function Breakdown({ title, rows }: { title: string; rows: { label: string; amount: number; flag?: boolean }[] }) {
  const total = rows.reduce((s, r) => s + r.amount, 0) || 1;
  return (
    <div className="border rounded-lg p-4">
      <h3 className="text-sm font-semibold text-gray-700 mb-2">{title}</h3>
      <div className="space-y-1.5">
        {rows.map((r, i) => (
          <div key={i} className={`text-sm ${r.flag ? 'text-red-700 font-medium' : ''}`}>
            <div className="flex justify-between"><span>{r.flag ? '⚠ ' : ''}{r.label}</span><span>{fmt$(r.amount)}</span></div>
            <div className="h-1.5 bg-gray-100 rounded"><div className={`h-1.5 rounded ${r.flag ? 'bg-red-400' : 'bg-nsf-light'}`} style={{ width: `${(r.amount / total) * 100}%` }} /></div>
          </div>
        ))}
      </div>
    </div>
  );
}

function BurnTable({ analytics }: { analytics: InvoiceAnalytics }) {
  return (
    <section>
      <h2 className="text-sm font-semibold text-gray-700 mb-2">Burn rate &amp; projected year-end funds</h2>
      <div className="overflow-x-auto border rounded-lg">
        <table className="min-w-full text-sm">
          <thead className="bg-gray-50 text-gray-500 text-xs"><tr>
            <th className="px-3 py-2 text-left">Institution</th><th className="px-3 py-2 text-right">Budget</th>
            <th className="px-3 py-2 text-right">Actual (to last invoice)</th><th className="px-3 py-2 text-right">Est. $/mo</th>
            <th className="px-3 py-2 text-right">Projected to date</th><th className="px-3 py-2 text-right">Expected year-end funds</th>
          </tr></thead>
          <tbody className="divide-y">
            {analytics.burn.map((b, i) => (
              <tr key={i} className={b.behind ? 'bg-amber-50' : ''}>
                <td className="px-3 py-2">{b.institution}{b.behind && <span className="ml-1 text-[10px] text-amber-700">(behind)</span>}</td>
                <td className="px-3 py-2 text-right">{fmt$(b.budget)}</td>
                <td className="px-3 py-2 text-right">{fmt$(b.actual_non_capital)}</td>
                <td className="px-3 py-2 text-right text-gray-500">{fmt$(b.estimated_monthly)}</td>
                <td className="px-3 py-2 text-right">{fmt$(b.projected_to_date)}</td>
                <td className={`px-3 py-2 text-right font-medium ${b.expected_year_end_funds < 0 ? 'text-red-700' : 'text-green-700'}`}>{fmt$(b.expected_year_end_funds)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="text-[11px] text-gray-400 mt-1">Estimated burn uses recurring (non-capital) actuals. Projected-to-date extrapolates unbilled time since the last invoice using the trailing 3-invoice average. Expected year-end funds = budget − projected spend to period end.</p>
    </section>
  );
}

// ---------------------------------------------------------------------------
// Unified invoice editor: upload/AI, details, coding — in one screen.
// ---------------------------------------------------------------------------
function InvoiceEditor({ mode, invoiceId: initialId, grant, subawards, grantId, onClose }: {
  mode: 'new' | 'edit'; invoiceId?: string; grant: Grant; subawards: Subaward[]; grantId: string; onClose: () => void;
}) {
  const qc = useQueryClient();
  const { isAdmin, isGrantAdmin, isSubawardAdmin, permittedInstitutions } = useAuth();
  const canApprove = isAdmin || isGrantAdmin;
  const [invoiceId, setInvoiceId] = useState<string | undefined>(initialId);
  const [runId, setRunId] = useState<string | null>(null);
  const createdRef = useRef(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const entityOptions = useMemo(() => {
    const all = [
      { key: `grant:${grant.id}`, entity_type: 'grant', entity_id: grant.id, label: `${grant.institution} (lead)`, institution: grant.institution },
      ...subawards.map(s => ({ key: `subaward:${s.id}`, entity_type: 'subaward', entity_id: s.id, label: s.institution, institution: s.institution })),
    ];
    // Subaward admins may only bill for their permitted institution(s).
    return isSubawardAdmin ? all.filter(o => permittedInstitutions.includes(o.institution)) : all;
  }, [grant, subawards, isSubawardAdmin, permittedInstitutions]);

  // In "new" mode, create a draft invoice under the first institution the user may bill.
  useEffect(() => {
    if (mode === 'new' && !invoiceId && !createdRef.current && entityOptions.length > 0) {
      createdRef.current = true;
      const def = entityOptions[0];
      api.invoiceCoding.create(def.entity_type, def.entity_id, { invoice_date: today(), amount: 0 })
        .then(inv => setInvoiceId(inv.id));
    }
  }, [mode, invoiceId, entityOptions]);

  const detailQ = useQuery({ queryKey: ['invoice-detail', invoiceId], queryFn: () => api.invoiceCoding.get(cur.entity_type, cur.entity_id, invoiceId!), enabled: false });
  // We must know the entity to fetch detail; track it locally, seeded once known.
  const [cur, setCur] = useState<{ entity_type: string; entity_id: string }>({ entity_type: 'grant', entity_id: grant.id });

  const detail = detailQ.data;

  // (Re)fetch detail whenever invoiceId or the current entity changes.
  useEffect(() => { if (invoiceId) detailQ.refetch(); /* eslint-disable-next-line */ }, [invoiceId, cur.entity_type, cur.entity_id]);
  useEffect(() => { if (detail) setCur({ entity_type: detail.entity_type, entity_id: detail.entity_id }); /* eslint-disable-next-line */ }, [detail?.entity_type, detail?.entity_id]);

  const personnelQ = useQuery({ queryKey: ['personnel', grantId], queryFn: () => api.personnel.list(grantId) });
  const wbsQ = useQuery({ queryKey: ['wbs', grantId], queryFn: () => api.wbs.list(grantId) });

  const et = cur.entity_type, eid = cur.entity_id;
  const invalidate = () => {
    if (invoiceId) qc.invalidateQueries({ queryKey: ['invoice-detail', invoiceId] });
    detailQ.refetch();
    qc.invalidateQueries({ queryKey: ['grant-invoices', grantId] });
    qc.invalidateQueries({ queryKey: ['invoice-analytics', grantId] });
  };

  // AI run polling.
  const runQ = useQuery({
    queryKey: ['coding-run', runId], queryFn: () => api.invoiceCoding.getRun(et, eid, runId!), enabled: !!runId,
    refetchInterval: (q) => { const s = q.state.data?.status; return s === 'completed' || s === 'failed' ? false : 1500; },
  });
  useEffect(() => { if (runQ.data?.status === 'completed' || runQ.data?.status === 'failed') invalidate(); /* eslint-disable-next-line */ }, [runQ.data?.status]);

  const instName = (etype: string, eidv: string) => entityOptions.find(o => o.entity_type === etype && o.entity_id === eidv)?.institution ?? '';
  const currentInstitution = detail ? instName(detail.entity_type, detail.entity_id) : '';
  const institutionStaff = (personnelQ.data ?? []).filter(p => !currentInstitution || p.institution === currentInstitution);

  const expenses = detail?.expenses ?? [];
  const wbsAreas = wbsQ.data ?? [];
  const codedTotal = expenses.reduce((s, e) => s + e.amount, 0);
  const uncat = uncategorizedForExpenses(expenses);
  const uncatTotal = uncat.wbs + uncat.category;
  const isFinal = detail?.coding_status === 'final';

  // Details form (live-save).
  const saveDetails = useMutation({
    mutationFn: (patch: Partial<Invoice>) => api.invoiceCoding.update(et, eid, invoiceId!, { ...detail!, ...patch }),
    onSuccess: (updated) => { setCur({ entity_type: updated.entity_type, entity_id: updated.entity_id }); invalidate(); },
  });
  const upload = useMutation({ mutationFn: (file: File) => api.invoiceCoding.upload(et, eid, invoiceId!, file), onSuccess: invalidate });
  const codeAI = useMutation({ mutationFn: () => api.invoiceCoding.code(et, eid, invoiceId!), onSuccess: (r) => setRunId(r.run_id) });
  const addExpense = useMutation({
    mutationFn: async () => {
      const created = await api.invoiceCoding.createExpense(et, eid, invoiceId!, { line_type: 'uncategorized', amount: 0, description: '' });
      // Default WBS split from the most recent prior line that has one.
      const prev = [...expenses].reverse().find(e => (e.wbs?.length ?? 0) > 0);
      if (prev?.wbs?.length) {
        await api.invoiceCoding.setExpenseWBS(et, eid, invoiceId!, created.id, prev.wbs.map(w => ({ wbs_area_id: w.wbs_area_id, allocation_percent: w.allocation_percent })));
      }
      return created;
    },
    onSuccess: invalidate,
  });
  const finalize = useMutation({ mutationFn: () => api.invoiceCoding.finalizeCoding(et, eid, invoiceId!), onSuccess: invalidate });
  const setPayStatus = useMutation({ mutationFn: (status: string) => api.invoiceCoding.setPaymentStatus(et, eid, invoiceId!, status), onSuccess: invalidate });
  const reopen = useMutation({ mutationFn: () => api.invoiceCoding.setCodingStatus(et, eid, invoiceId!, 'draft'), onSuccess: invalidate });
  const discard = useMutation({ mutationFn: () => api.invoiceCoding.remove(et, eid, invoiceId!), onSuccess: () => { invalidate(); onClose(); } });

  const doFinalize = () => {
    if (uncatTotal > 0.5 && !confirm(`This coding still has ${fmt$(uncatTotal)} uncategorized. Finalize anyway?`)) return;
    finalize.mutate();
  };

  const loading = !invoiceId || !detail;

  return (
    <Modal title={mode === 'new' ? 'New invoice' : `Invoice ${detail?.invoice_number || ''}`} wide onClose={onClose}>
      {loading ? <div className="p-6 text-gray-400 text-sm">Preparing invoice…</div> : (
      <div className="space-y-5">
        {/* Section 1: document + AI */}
        <Section n={1} title="Document">
          <div className="flex flex-wrap items-center gap-2">
            <input ref={fileRef} type="file" className="hidden" onChange={e => { const f = e.target.files?.[0]; if (f) upload.mutate(f); }} />
            <button onClick={() => fileRef.current?.click()} disabled={isFinal || upload.isPending} className="px-2.5 py-1 text-xs border rounded hover:bg-gray-50 disabled:opacity-50">
              {detail!.document_id ? 'Replace PDF' : 'Upload PDF'}
            </button>
            <button onClick={() => codeAI.mutate()} disabled={isFinal || !detail!.document_id || codeAI.isPending || (!!runQ.data && runQ.data.status !== 'completed' && runQ.data.status !== 'failed')}
              className="px-2.5 py-1 text-xs bg-nsf-blue text-white rounded disabled:opacity-50" title={!detail!.document_id ? 'Upload a PDF first' : 'Let AI propose details + a draft coding'}>
              ✨ Process with AI
            </button>
            {runQ.data && runQ.data.status !== 'completed' && runQ.data.status !== 'failed' && <span className="text-xs text-gray-500">{runQ.data.status_detail || runQ.data.status}…</span>}
            {runQ.data?.status === 'failed' && <span className="text-xs text-red-600">AI failed: {runQ.data.error_msg}</span>}
            {runQ.data?.status === 'completed' && <span className="text-xs text-green-600">AI proposed a draft — review below.</span>}
          </div>
          {runQ.data?.summary_md && runQ.data.status === 'completed' && (
            <details className="text-xs text-gray-600 bg-gray-50 rounded p-2 mt-2"><summary className="cursor-pointer">AI summary</summary><pre className="whitespace-pre-wrap mt-1">{runQ.data.summary_md}</pre></details>
          )}
        </Section>

        {/* Section 2: invoice details */}
        <Section n={2} title="Invoice details">
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
            <Field label="Institution">
              <select disabled={isFinal} value={`${detail!.entity_type}:${detail!.entity_id}`}
                onChange={e => { const o = entityOptions.find(x => x.key === e.target.value)!; saveDetails.mutate({ entity_type: o.entity_type, entity_id: o.entity_id }); }}
                className="w-full border rounded px-2 py-1.5 text-sm">
                {entityOptions.map(o => <option key={o.key} value={o.key}>{o.label}</option>)}
              </select>
            </Field>
            <DetailField label="Invoice #" value={detail!.invoice_number ?? ''} disabled={isFinal} onSave={v => saveDetails.mutate({ invoice_number: v })} />
            <DetailField label="Received date" type="date" value={detail!.invoice_date ?? ''} disabled={isFinal} onSave={v => saveDetails.mutate({ invoice_date: v })} />
            <DetailField label="Period start" type="date" value={detail!.period_start ?? ''} disabled={isFinal} onSave={v => saveDetails.mutate({ period_start: v || undefined })} />
            <DetailField label="Period end" type="date" value={detail!.period_end ?? ''} disabled={isFinal} onSave={v => saveDetails.mutate({ period_end: v || undefined })} />
            <DetailField label="Amount ($)" type="number" value={String(detail!.amount ?? 0)} disabled={isFinal} onSave={v => saveDetails.mutate({ amount: parseFloat(v) || 0 })} />
          </div>
        </Section>

        {/* Section 3: coding */}
        <Section n={3} title="Coding">
          <div className="flex items-center justify-between text-sm mb-2">
            <div className="text-gray-500">Invoice total <span className="font-medium text-gray-800">{fmt$(detail!.amount)}</span> · Coded <span className={`font-medium ${Math.abs(codedTotal - detail!.amount) > 0.5 ? 'text-amber-700' : 'text-gray-800'}`}>{fmt$(codedTotal)}</span></div>
            <span className={`px-2 py-0.5 rounded text-xs ${CODING_BADGE[detail!.coding_status]}`}>{detail!.coding_status}</span>
          </div>
          {uncatTotal > 0.5 && (
            <div className="border border-red-300 bg-red-50 rounded p-3 text-sm text-red-800 mb-2">
              ⚠ <span className="font-semibold">{fmt$(uncatTotal)}</span> of {fmt$(codedTotal)} uncategorized
              {uncat.category > 0.5 && <> · {fmt$(uncat.category)} with no category</>}
              {uncat.wbs > 0.5 && <> · {fmt$(uncat.wbs)} with no WBS area</>}. Clear before finalizing.
            </div>
          )}
          <div className="overflow-x-auto border rounded">
            <table className="min-w-full text-sm">
              <thead className="bg-gray-50 text-gray-500 text-xs"><tr>
                <th className="px-2 py-1.5 text-left">Category</th><th className="px-2 py-1.5 text-left">Description</th>
                <th className="px-2 py-1.5 text-left">Personnel</th><th className="px-2 py-1.5 text-right">Amount</th>
                <th className="px-2 py-1.5 text-left">WBS split</th><th></th>
              </tr></thead>
              <tbody className="divide-y">
                {expenses.map(e => (
                  <ExpenseRow key={e.id} expense={e} et={et} eid={eid} invoiceId={invoiceId!} grantId={grantId}
                    wbsAreas={wbsAreas} staff={institutionStaff} readOnly={isFinal} onChanged={invalidate} />
                ))}
                {expenses.length === 0 && <tr><td colSpan={6} className="px-2 py-6 text-center text-gray-400">No expense lines. Upload a PDF and “Process with AI”, or add lines manually.</td></tr>}
              </tbody>
            </table>
          </div>
          <div className="mt-2"><button onClick={() => addExpense.mutate()} disabled={isFinal} className="px-2.5 py-1 text-xs border rounded hover:bg-gray-50 disabled:opacity-50">+ Add expense line</button></div>
        </Section>

        {/* Footer actions */}
        <div className="flex items-center justify-between border-t pt-3">
          <div className="flex items-center gap-3">
            <button onClick={() => { if (confirm('Delete this invoice?')) discard.mutate(); }} className="px-3 py-1.5 text-sm text-red-500 hover:text-red-700">Delete invoice</button>
            <div className="flex items-center gap-1.5 text-sm">
              <span className="text-gray-500">Payment:</span>
              {canApprove ? (
                <select value={detail!.status} onChange={e => setPayStatus.mutate(e.target.value)} className="border rounded px-1.5 py-1 text-xs">
                  {['pending', 'approved', 'rejected', 'paid'].map(s => <option key={s} value={s}>{s}</option>)}
                </select>
              ) : (
                <span className={`px-2 py-0.5 rounded text-xs ${PAY_BADGE[detail!.status] ?? ''}`}>{detail!.status}</span>
              )}
            </div>
          </div>
          <div className="flex gap-2">
            {isFinal
              ? <button onClick={() => reopen.mutate()} className="px-3 py-1.5 text-sm border rounded">Reopen coding</button>
              : <button onClick={doFinalize} disabled={finalize.isPending} className="px-3 py-1.5 text-sm bg-green-600 text-white rounded disabled:opacity-50">Finalize coding</button>}
            <button onClick={onClose} className="px-3 py-1.5 text-sm bg-nsf-light text-white rounded">Done</button>
          </div>
        </div>
      </div>
      )}
    </Modal>
  );
}

function ExpenseRow({ expense, et, eid, invoiceId, grantId, wbsAreas, staff, readOnly, onChanged }: {
  expense: InvoiceExpense; et: string; eid: string; invoiceId: string; grantId: string;
  wbsAreas: WBSArea[]; staff: Personnel[]; readOnly: boolean; onChanged: () => void;
}) {
  const [showWbs, setShowWbs] = useState(false);
  const [local, setLocal] = useState(expense);
  useEffect(() => setLocal(expense), [expense]);

  const save = useMutation({ mutationFn: (patch: Partial<InvoiceExpense>) => api.invoiceCoding.updateExpense(et, eid, invoiceId, expense.id, patch), onSuccess: () => onChanged() });
  const del = useMutation({ mutationFn: () => api.invoiceCoding.deleteExpense(et, eid, invoiceId, expense.id), onSuccess: () => onChanged() });
  const setWbs = useMutation({ mutationFn: (allocs: InvoiceExpenseWBS[]) => api.invoiceCoding.setExpenseWBS(et, eid, invoiceId, expense.id, allocs), onSuccess: () => onChanged() });

  const pct = wbsPct(local);
  const rem = Math.max(0, 100 - pct);
  const isUncat = local.line_type === 'uncategorized' || !local.line_type;
  const isPersonnelLine = PERSONNEL_CATEGORIES.includes(local.line_type);

  // Selecting a person seeds the WBS split from their default (if this line has none yet).
  const selectPerson = async (personId: string) => {
    save.mutate({ personnel_id: personId || undefined });
    if (personId && (expense.wbs?.length ?? 0) === 0) {
      const dw = await api.personnel.defaultWBS(grantId, personId);
      if (dw.length) setWbs.mutate(dw.map(w => ({ wbs_area_id: w.wbs_area_id, allocation_percent: w.percent })));
    }
  };

  return (
    <>
      <tr className={isUncat ? 'bg-red-50/60' : ''}>
        <td className="px-2 py-1.5">
          <select disabled={readOnly} value={local.line_type} onChange={e => save.mutate({ line_type: e.target.value })}
            className={`border rounded px-1.5 py-1 text-xs ${isUncat ? 'border-red-300 text-red-700' : ''}`}>
            {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        </td>
        <td className="px-2 py-1.5">
          <input disabled={readOnly} value={local.description ?? ''} onChange={e => setLocal({ ...local, description: e.target.value })}
            onBlur={() => local.description !== expense.description && save.mutate({ description: local.description })}
            className="w-full border rounded px-1.5 py-1 text-xs" />
        </td>
        <td className="px-2 py-1.5">
          {isPersonnelLine ? (
            <select disabled={readOnly} value={local.personnel_id ?? ''} onChange={e => selectPerson(e.target.value)} className="border rounded px-1.5 py-1 text-xs max-w-[10rem]">
              <option value="">— person —</option>
              {staff.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          ) : <span className="text-gray-300 text-xs">—</span>}
        </td>
        <td className="px-2 py-1.5 text-right">
          <input disabled={readOnly} type="number" value={local.amount || ''} onChange={e => setLocal({ ...local, amount: parseFloat(e.target.value) || 0 })}
            onBlur={() => local.amount !== expense.amount && save.mutate({ amount: local.amount })}
            className="w-24 border rounded px-1.5 py-1 text-xs text-right" />
        </td>
        <td className="px-2 py-1.5">
          <button onClick={() => setShowWbs(!showWbs)} className={`text-xs ${rem > 0.5 ? 'text-red-600' : 'text-green-600'}`}>
            {pct >= 99.95 ? 'fully allocated' : rem >= 99.95 ? '⚠ no WBS' : `⚠ ${rem.toFixed(0)}% uncategorized`}
          </button>
        </td>
        <td className="px-2 py-1.5 text-right">{!readOnly && <button onClick={() => del.mutate()} className="text-red-400 hover:text-red-600 text-xs">✕</button>}</td>
      </tr>
      {showWbs && (
        <tr><td colSpan={6} className="px-4 py-2 bg-gray-50">
          <WbsSplitEditor expense={expense} wbsAreas={wbsAreas} readOnly={readOnly} onSave={(allocs) => { setWbs.mutate(allocs); setShowWbs(false); }} />
        </td></tr>
      )}
    </>
  );
}

function WbsSplitEditor({ expense, wbsAreas, readOnly, onSave }: {
  expense: InvoiceExpense; wbsAreas: WBSArea[]; readOnly: boolean; onSave: (a: InvoiceExpenseWBS[]) => void;
}) {
  const [allocs, setAllocs] = useState<Record<string, number>>(() => {
    const m: Record<string, number> = {};
    for (const w of expense.wbs ?? []) m[w.wbs_area_id] = w.allocation_percent;
    return m;
  });
  const total = Object.values(allocs).reduce((s, v) => s + (v || 0), 0);
  return (
    <div className="space-y-1.5">
      <p className="text-xs text-gray-500">Allocate across WBS areas. Any remainder is left uncategorized (allowed, but flagged).</p>
      {wbsAreas.map(w => (
        <div key={w.id} className="flex items-center gap-2 text-xs">
          <span className="w-48 truncate">{w.code} · {w.name}</span>
          <input disabled={readOnly} type="number" className="w-20 border rounded px-1.5 py-0.5 text-right" value={allocs[w.id] ?? ''} onChange={e => setAllocs({ ...allocs, [w.id]: parseFloat(e.target.value) || 0 })} />
          <span className="text-gray-400">%</span>
        </div>
      ))}
      <div className="flex items-center justify-between pt-1">
        <span className={`text-xs ${Math.abs(total - 100) < 0.05 ? 'text-green-600' : total > 100 ? 'text-red-600' : 'text-amber-600'}`}>{total.toFixed(0)}% allocated{total < 100 ? ` · ${(100 - total).toFixed(0)}% uncategorized` : ''}</span>
        {!readOnly && <button onClick={() => onSave(Object.entries(allocs).filter(([, v]) => v > 0).map(([wbs_area_id, allocation_percent]) => ({ wbs_area_id, allocation_percent })))} className="px-2 py-1 text-xs bg-nsf-light text-white rounded">Save WBS</button>}
      </div>
    </div>
  );
}

// A live-saving text/date/number detail field.
function DetailField({ label, value, type = 'text', disabled, onSave }: { label: string; value: string; type?: string; disabled?: boolean; onSave: (v: string) => void }) {
  const [v, setV] = useState(value);
  useEffect(() => setV(value), [value]);
  const commit = () => { if (v !== value) onSave(v); };
  return (
    <Field label={label}>
      {type === 'date'
        ? <input type="date" disabled={disabled} value={v} onChange={e => { setV(e.target.value); }} onBlur={commit} className="w-full border rounded px-2 py-1.5 text-sm" />
        : <input type={type} disabled={disabled} value={v} onChange={e => setV(e.target.value)} onBlur={commit} className="w-full border rounded px-2 py-1.5 text-sm" />}
    </Field>
  );
}

function Section({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <section className="border rounded-lg">
      <div className="flex items-center gap-2 border-b px-3 py-2 bg-gray-50">
        <span className="w-5 h-5 rounded-full bg-nsf-blue text-white text-xs flex items-center justify-center">{n}</span>
        <h3 className="text-sm font-semibold text-gray-700">{title}</h3>
      </div>
      <div className="p-3">{children}</div>
    </section>
  );
}

function Modal({ title, children, onClose, wide }: { title: string; children: React.ReactNode; onClose: () => void; wide?: boolean }) {
  return (
    <div className="fixed inset-0 bg-black/30 flex items-start justify-center p-4 z-50 overflow-y-auto" onClick={onClose}>
      <div className={`bg-white rounded-lg shadow-xl w-full ${wide ? 'max-w-4xl' : 'max-w-lg'} my-8`} onClick={e => e.stopPropagation()}>
        <div className="flex items-center justify-between border-b px-4 py-3"><h3 className="font-semibold text-nsf-blue">{title}</h3><button onClick={onClose} className="text-gray-400 hover:text-gray-600">✕</button></div>
        <div className="p-4">{children}</div>
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return <label className="block"><span className="block text-xs font-medium text-gray-600 mb-1">{label}</span>{children}</label>;
}
