'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  WBSArea,
  BudgetLineItem,
  BudgetLineItemWBS,
  InstitutionBudget,
  InstitutionFringeRate,
  OverheadRate,
  Personnel,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useState, useMemo, useCallback, Suspense, useRef, useEffect } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import CurrencyInput from '@/components/CurrencyInput';
import { ValidationError } from '@/lib/api';

const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

const LINE_TYPES = [
  { value: 'personnel', label: 'Personnel (Salary)' },
  { value: 'fringe', label: 'Fringe Benefits' },
  { value: 'travel', label: 'Travel' },
  { value: 'equipment', label: 'Equipment' },
  { value: 'supplies', label: 'Supplies' },
  { value: 'contractual', label: 'Contractual' },
  { value: 'other', label: 'Other Direct Costs' },
  { value: 'tuition', label: 'Tuition' },
  { value: 'participant_support', label: 'Participant Support' },
];

interface InstitutionOption {
  entityType: string;
  entityId: string;
  label: string;
  salaryEscalationRate: number;
}

export default function BudgetPage() {
  return (
    <Suspense fallback={<div className="p-4">Loading budget...</div>}>
      <BudgetPageInner />
    </Suspense>
  );
}

function BudgetPageInner() {
  const { grantId, isLoading: grantLoading } = useGrant();
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  const initialYear = searchParams.get('year');
  const [selectedYear, setSelectedYear] = useState<number>(initialYear ? parseInt(initialYear) : 1);
  const [tab, setTab] = useState<'lineItems' | 'wbs'>('lineItems');
  const [showWbsForm, setShowWbsForm] = useState(false);

  const { data: grant } = useQuery({
    queryKey: ['grant', grantId], queryFn: () => api.grants.get(grantId!), enabled: !!grantId,
  });
  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId], queryFn: () => api.subawards.list(grantId!), enabled: !!grantId,
  });
  const { data: wbsAreas, isLoading: wbsLoading } = useQuery({
    queryKey: ['wbs', grantId], queryFn: () => api.wbs.list(grantId!), enabled: !!grantId,
  });
  const { data: personnel } = useQuery({
    queryKey: ['personnel', grantId], queryFn: () => api.personnel.list(grantId!), enabled: !!grantId,
  });

  const institutions = useMemo<InstitutionOption[]>(() => {
    const opts: InstitutionOption[] = [];
    if (grant) opts.push({ entityType: 'grant', entityId: grant.id, label: `${grant.institution} (Lead)`, salaryEscalationRate: grant.salary_escalation_rate ?? 0 });
    for (const s of subawards ?? []) opts.push({ entityType: 'subaward', entityId: s.id, label: s.institution, salaryEscalationRate: s.salary_escalation_rate ?? 0 });
    return opts;
  }, [grant, subawards]);

  const [selectedInst, setSelectedInst] = useState<string>('');
  const activeInst = institutions.find((i) => `${i.entityType}:${i.entityId}` === selectedInst) || institutions[0];

  const createWbsMutation = useMutation({
    mutationFn: (data: Partial<WBSArea>) => api.wbs.create(grantId!, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['wbs', grantId] }); setShowWbsForm(false); },
  });
  const deleteWbsMutation = useMutation({
    mutationFn: (id: string) => api.wbs.delete(grantId!, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['wbs', grantId] }),
  });

  if (grantLoading || wbsLoading) return <div className="p-4">Loading budget...</div>;
  if (!grantId) return <div className="p-4">No project configured. <Link href="/settings" className="text-nsf-light hover:underline">Set up project</Link></div>;

  return (
    <div className="max-w-6xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">Budget</h1>
        <p className="text-sm text-gray-500">Manage institution budgets by year with versioned line items</p>
      </div>

      {/* Tabs */}
      <div className="flex border-b">
        {(['lineItems', 'wbs'] as const).map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              tab === t ? 'text-nsf-blue border-nsf-blue' : 'text-gray-500 border-transparent hover:text-gray-700'
            }`}>
            {t === 'lineItems' ? 'Budget Line Items' : 'WBS Areas'}
          </button>
        ))}
      </div>

      {tab === 'lineItems' && (
        <div className="space-y-4">
          <div className="flex flex-wrap items-end gap-4">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Institution</label>
              <select value={activeInst ? `${activeInst.entityType}:${activeInst.entityId}` : ''}
                onChange={(e) => setSelectedInst(e.target.value)}
                className="border rounded-md px-3 py-2 text-sm min-w-[200px]">
                {institutions.map((i) => (
                  <option key={`${i.entityType}:${i.entityId}`} value={`${i.entityType}:${i.entityId}`}>{i.label}</option>
                ))}
              </select>
            </div>
            <div className="flex flex-wrap gap-1">
              {PROJECT_YEARS.map((year) => (
                <button key={year} onClick={() => setSelectedYear(year)}
                  className={`px-3 py-2 rounded-md text-sm font-medium transition-colors ${
                    selectedYear === year ? 'bg-nsf-blue text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                  }`}>{YEAR_LABELS[year]}</button>
              ))}
            </div>
          </div>
          {activeInst && (
            <InstitutionBudgetPanel entityType={activeInst.entityType} entityId={activeInst.entityId}
              institutionLabel={activeInst.label} fiscalYear={selectedYear}
              salaryEscalationRate={activeInst.salaryEscalationRate}
              wbsAreas={wbsAreas ?? []} personnel={personnel ?? []} />
          )}
        </div>
      )}

      {tab === 'wbs' && (
        <div className="space-y-4">
          <div className="flex justify-end">
            <button onClick={() => setShowWbsForm(!showWbsForm)}
              className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors">
              {showWbsForm ? 'Cancel' : '+ Add WBS Area'}
            </button>
          </div>
          {showWbsForm && <WBSForm onSubmit={(data) => createWbsMutation.mutate(data)} isLoading={createWbsMutation.isPending} />}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {wbsAreas?.map((wbs) => (
              <div key={wbs.id} className="bg-white rounded-lg border p-4">
                <div className="flex items-center justify-between">
                  <div className="font-medium text-nsf-blue">{wbs.code}: {wbs.name}</div>
                  <button onClick={() => { if (confirm(`Delete WBS area ${wbs.code}?`)) deleteWbsMutation.mutate(wbs.id); }}
                    className="text-red-500 hover:underline text-xs">Delete</button>
                </div>
                {wbs.description && <p className="text-sm text-gray-500 mt-1">{wbs.description}</p>}
                <p className="text-sm text-gray-600 mt-2">Total Budget: ${wbs.budget.toLocaleString()}</p>
              </div>
            ))}
            {(!wbsAreas || wbsAreas.length === 0) && <p className="text-gray-400 text-sm col-span-full">No WBS areas defined yet.</p>}
          </div>
        </div>
      )}
    </div>
  );
}

/* ======== Institution Budget Panel ======== */

function InstitutionBudgetPanel({
  entityType, entityId, institutionLabel, fiscalYear, salaryEscalationRate, wbsAreas, personnel,
}: {
  entityType: string; entityId: string; institutionLabel: string;
  fiscalYear: number; salaryEscalationRate: number; wbsAreas: WBSArea[]; personnel: Personnel[];
}) {
  const queryClient = useQueryClient();
  const [showLineItemForm, setShowLineItemForm] = useState(false);
  const [finalizeErrors, setFinalizeErrors] = useState<string[]>([]);
  const budgetsKey = ['institution-budgets', entityType, entityId];

  const { data: allBudgets, isLoading: budgetsLoading } = useQuery({
    queryKey: budgetsKey,
    queryFn: () => api.institutionBudgets.list(entityType, entityId, false),
  });

  const budget = useMemo(
    () => (allBudgets ?? []).find((b) => b.fiscal_year === fiscalYear && b.is_latest),
    [allBudgets, fiscalYear]
  );

  const olderVersions = useMemo(
    () => (allBudgets ?? []).filter((b) => b.fiscal_year === fiscalYear && !b.is_latest).sort((a, b) => b.version - a.version),
    [allBudgets, fiscalYear]
  );

  const createBudgetMutation = useMutation({
    mutationFn: () => api.institutionBudgets.create(entityType, entityId, { fiscal_year: fiscalYear, budget: 0 }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: budgetsKey }),
  });

  const finalizeMutation = useMutation({
    mutationFn: () => api.institutionBudgets.finalize(entityType, entityId, budget!.id),
    onSuccess: () => { setFinalizeErrors([]); queryClient.invalidateQueries({ queryKey: budgetsKey }); },
    onError: (err) => {
      if (err instanceof ValidationError) {
        setFinalizeErrors(err.validationErrors);
      }
    },
  });

  const duplicateMutation = useMutation({
    mutationFn: () => api.institutionBudgets.duplicate(entityType, entityId, budget!.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: budgetsKey }),
  });

  const deleteBudgetMutation = useMutation({
    mutationFn: () => api.institutionBudgets.delete(entityType, entityId, budget!.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: budgetsKey }),
  });

  const lineItemsKey = ['line-items', entityType, entityId, budget?.id];
  const { data: lineItems, isLoading: lineItemsLoading } = useQuery({
    queryKey: lineItemsKey,
    queryFn: () => api.budgetLineItems.list(entityType, entityId, budget!.id),
    enabled: !!budget,
  });

  const { data: overheadRates } = useQuery({
    queryKey: ['overhead-rates', entityType, entityId],
    queryFn: () => api.overheadRates.list(entityType, entityId),
  });

  const { data: fringeRates } = useQuery({
    queryKey: ['fringe-rates', entityType, entityId],
    queryFn: () => api.fringeRates.list(entityType, entityId),
  });

  const createLineItem = useMutation({
    mutationFn: (data: Partial<BudgetLineItem>) =>
      api.budgetLineItems.create(entityType, entityId, budget!.id, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: lineItemsKey }); setShowLineItemForm(false); },
  });

  const updateLineItem = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<BudgetLineItem> }) =>
      api.budgetLineItems.update(entityType, entityId, budget!.id, id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: lineItemsKey }),
  });

  const deleteLineItem = useMutation({
    mutationFn: (id: string) => api.budgetLineItems.delete(entityType, entityId, budget!.id, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: lineItemsKey }),
  });

  const yearFringeRates = useMemo(
    () => (fringeRates ?? []).filter((fr) => fr.fiscal_year === fiscalYear),
    [fringeRates, fiscalYear]
  );

  const handleAddPersonnel = useCallback(async (personId: string, effortMonths: number) => {
    const person = personnel.find((p) => p.id === personId);
    if (!person || !budget) return;
    // Apply salary escalation: Y1 = base, Y2 = base*(1+rate), etc.
    const escalatedSalary = person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1);
    const monthlySalary = escalatedSalary / 12;
    const salaryAmount = Math.round(monthlySalary * effortMonths * 100) / 100;

    await api.budgetLineItems.create(entityType, entityId, budget.id, {
      line_type: 'personnel', description: `${person.name} — Salary`,
      personnel_id: personId, effort_months: effortMonths, amount: salaryAmount,
    });

    for (const fr of yearFringeRates) {
      const fringeAmount = Math.round(salaryAmount * fr.rate * 100) / 100;
      await api.budgetLineItems.create(entityType, entityId, budget.id, {
        line_type: 'fringe', description: `${person.name} — ${fr.rate_name}`,
        personnel_id: personId, amount: fringeAmount,
        notes: `${(fr.rate * 100).toFixed(2)}% of $${salaryAmount.toLocaleString()}`,
      });
    }

    queryClient.invalidateQueries({ queryKey: lineItemsKey });
    setShowLineItemForm(false);
  }, [personnel, budget, entityType, entityId, yearFringeRates, queryClient, lineItemsKey, salaryEscalationRate, fiscalYear]);

  // Cascade effort changes: update salary amount and all associated fringe lines
  const handleEffortUpdate = useCallback(async (salaryLineItem: BudgetLineItem, newEffort: number) => {
    if (!budget) return;
    const person = personnel.find(p => p.id === salaryLineItem.personnel_id);
    if (!person) return;
    const escalatedSalary = person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1);
    const newSalaryAmount = Math.round((escalatedSalary / 12) * newEffort * 100) / 100;

    await api.budgetLineItems.update(entityType, entityId, budget.id, salaryLineItem.id, {
      effort_months: newEffort, amount: newSalaryAmount,
    });

    const fringeItems = (lineItems ?? []).filter(
      li => li.line_type === 'fringe' && li.personnel_id === salaryLineItem.personnel_id
    );
    for (const fli of fringeItems) {
      const matchingRate = yearFringeRates.find(fr => fli.description?.includes(fr.rate_name));
      if (matchingRate) {
        const newFringeAmount = Math.round(newSalaryAmount * matchingRate.rate * 100) / 100;
        await api.budgetLineItems.update(entityType, entityId, budget.id, fli.id, {
          amount: newFringeAmount,
          notes: `${(matchingRate.rate * 100).toFixed(2)}% of $${newSalaryAmount.toLocaleString()}`,
        });
      }
    }

    queryClient.invalidateQueries({ queryKey: lineItemsKey });
  }, [budget, personnel, lineItems, yearFringeRates, entityType, entityId, salaryEscalationRate, fiscalYear, queryClient, lineItemsKey]);

  // Delete all salary + fringe lines for a given person (bundle delete)
  const handleDeletePersonnelBundle = useCallback(async (personnelId: string) => {
    if (!budget) return;
    const personName = personnel.find(p => p.id === personnelId)?.name ?? 'this person';
    if (!confirm(`Delete salary and all fringe lines for ${personName}?`)) return;
    const bundleItems = (lineItems ?? []).filter(
      li => (li.line_type === 'personnel' || li.line_type === 'fringe') && li.personnel_id === personnelId
    );
    for (const li of bundleItems) {
      await api.budgetLineItems.delete(entityType, entityId, budget.id, li.id);
    }
    queryClient.invalidateQueries({ queryKey: lineItemsKey });
  }, [budget, personnel, lineItems, entityType, entityId, queryClient, lineItemsKey]);

  if (budgetsLoading) return <div className="text-sm text-gray-400 py-4">Loading budgets...</div>;

  if (!budget) {
    return (
      <div className="bg-white rounded-lg border p-8 text-center space-y-3">
        <p className="text-gray-500">
          No budget exists for <strong>{institutionLabel}</strong> in <strong>{YEAR_LABELS[fiscalYear]}</strong>.
        </p>
        <button onClick={() => createBudgetMutation.mutate()} disabled={createBudgetMutation.isPending}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue disabled:opacity-50">
          {createBudgetMutation.isPending ? 'Creating...' : 'Create Draft Budget'}
        </button>
      </div>
    );
  }

  const totalAmount = (lineItems ?? []).reduce((sum, li) => sum + li.amount, 0);
  const isDraft = budget.status === 'draft';
  const personnelAndFringeItems = (lineItems ?? []).filter(li => li.line_type === 'personnel' || li.line_type === 'fringe');
  const otherDirectItems = (lineItems ?? []).filter(li => li.line_type !== 'personnel' && li.line_type !== 'fringe');
  const personnelSubtotal = personnelAndFringeItems.reduce((s, li) => s + li.amount, 0);
  const otherSubtotal = otherDirectItems.reduce((s, li) => s + li.amount, 0);

  return (
    <div className="space-y-4">
      {/* Budget header */}
      <div className={`rounded-lg border p-4 flex flex-wrap items-center justify-between gap-3 ${isDraft ? 'bg-amber-50 border-amber-300' : 'bg-white'}`}>
        <div className="flex items-center gap-3 flex-wrap">
          <span className="text-sm text-gray-500">{institutionLabel}</span>
          <span className="text-gray-300">|</span>
          <span className="text-sm text-gray-500">{YEAR_LABELS[fiscalYear]}</span>
          <span className="text-gray-300">|</span>
          <span className="text-sm text-gray-400">v{budget.version}</span>
          <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${isDraft ? 'bg-amber-200 text-amber-800' : 'bg-green-100 text-green-800'}`}>
            {isDraft ? 'DRAFT' : 'FINAL'}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-lg font-semibold text-nsf-blue">${totalAmount.toLocaleString()}</span>
          <div className="flex gap-1.5">
            <NSF1030Dropdown entityType={entityType} entityId={entityId} fiscalYear={fiscalYear} />
            {isDraft && (
              <button onClick={() => { setFinalizeErrors([]); finalizeMutation.mutate(); }}
                disabled={finalizeMutation.isPending}
                className="px-3 py-1.5 bg-green-600 text-white rounded text-xs hover:bg-green-700 disabled:opacity-50">
                {finalizeMutation.isPending ? '...' : 'Finalize'}
              </button>
            )}
            {!isDraft && (
              <button onClick={() => duplicateMutation.mutate()} disabled={duplicateMutation.isPending}
                className="px-3 py-1.5 bg-nsf-light text-white rounded text-xs hover:bg-nsf-blue disabled:opacity-50">
                {duplicateMutation.isPending ? '...' : 'Duplicate as New Draft'}
              </button>
            )}
            {isDraft && (
              <button onClick={() => { if (confirm('Delete this draft budget and all its line items?')) deleteBudgetMutation.mutate(); }}
                disabled={deleteBudgetMutation.isPending}
                className="px-3 py-1.5 bg-red-500 text-white rounded text-xs hover:bg-red-600 disabled:opacity-50">
                Delete Draft
              </button>
            )}
          </div>
        </div>
      </div>

      {finalizeErrors.length > 0 && (
        <div className="bg-red-50 border border-red-300 rounded-lg p-4">
          <p className="text-sm font-medium text-red-800 mb-1">Cannot finalize budget:</p>
          <ul className="list-disc list-inside text-sm text-red-700 space-y-0.5">
            {finalizeErrors.map((e, i) => <li key={i}>{e}</li>)}
          </ul>
        </div>
      )}

      {olderVersions.length > 0 && (
        <p className="text-xs text-gray-400">
          {olderVersions.length} older version{olderVersions.length > 1 ? 's' : ''}: {olderVersions.map((b) => `v${b.version} (${b.status})`).join(', ')}
        </p>
      )}

      {isDraft && (
        <div className="flex justify-end">
          <button onClick={() => setShowLineItemForm(!showLineItemForm)}
            className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors">
            {showLineItemForm ? 'Cancel' : '+ Add Line Item'}
          </button>
        </div>
      )}

      {showLineItemForm && isDraft && (
        <LineItemForm personnel={personnel} overheadRates={overheadRates ?? []}
          yearFringeRates={yearFringeRates}
          fiscalYear={fiscalYear} salaryEscalationRate={salaryEscalationRate}
          onSubmit={(data) => createLineItem.mutate(data)}
          onAddPersonnel={handleAddPersonnel} isLoading={createLineItem.isPending} />
      )}

      {/* Table 1: Personnel & Fringe */}
      <div>
        <h3 className="text-sm font-semibold text-gray-700 mb-2">Personnel & Fringe Benefits</h3>
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Type</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Description</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Person</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Effort</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Amount</th>
                <th className="text-center px-4 py-3 text-xs font-medium text-gray-600 uppercase">WBS</th>
                {isDraft && <th className="px-2 py-3 w-16"></th>}
              </tr>
            </thead>
            <tbody className="divide-y">
              {lineItemsLoading && (
                <tr><td colSpan={isDraft ? 7 : 6} className="px-4 py-4 text-gray-400 text-sm">Loading...</td></tr>
              )}
              {!lineItemsLoading && personnelAndFringeItems.map((li) => (
                <LineItemRow key={li.id} lineItem={li}
                  entityType={entityType} entityId={entityId} budgetId={budget.id}
                  isDraft={isDraft} personnel={personnel} wbsAreas={wbsAreas}
                  overheadRates={overheadRates ?? []}
                  tableMode="personnel"
                  salaryEscalationRate={salaryEscalationRate} fiscalYear={fiscalYear}
                  onUpdate={(data) => updateLineItem.mutate({ id: li.id, data })}
                  onEffortUpdate={handleEffortUpdate}
                  onDelete={() => { if (li.personnel_id) handleDeletePersonnelBundle(li.personnel_id); else deleteLineItem.mutate(li.id); }} />
              ))}
              {!lineItemsLoading && personnelAndFringeItems.length === 0 && (
                <tr><td colSpan={isDraft ? 7 : 6} className="px-4 py-8 text-center text-gray-400 text-sm">
                  No personnel line items. {isDraft ? 'Use "+ Add Line Item" → "Add Personnel + Fringe" above.' : ''}
                </td></tr>
              )}
            </tbody>
            {personnelAndFringeItems.length > 0 && (
              <tfoot className="bg-gray-50 border-t">
                <tr>
                  <td colSpan={4} className="px-4 py-3 text-sm font-semibold text-gray-700 text-right">Subtotal (Personnel + Fringe)</td>
                  <td className="px-4 py-3 text-sm font-semibold text-nsf-blue text-right">${personnelSubtotal.toLocaleString()}</td>
                  <td colSpan={isDraft ? 2 : 1}></td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      </div>

      {/* Table 2: Other Direct Costs */}
      <div>
        <h3 className="text-sm font-semibold text-gray-700 mb-2">Other Direct Costs</h3>
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Type</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Description</th>
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Overhead</th>
                <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Amount</th>
                <th className="text-center px-4 py-3 text-xs font-medium text-gray-600 uppercase">WBS</th>
                {isDraft && <th className="px-2 py-3 w-16"></th>}
              </tr>
            </thead>
            <tbody className="divide-y">
              {lineItemsLoading && (
                <tr><td colSpan={isDraft ? 6 : 5} className="px-4 py-4 text-gray-400 text-sm">Loading...</td></tr>
              )}
              {!lineItemsLoading && otherDirectItems.map((li) => (
                <LineItemRow key={li.id} lineItem={li}
                  entityType={entityType} entityId={entityId} budgetId={budget.id}
                  isDraft={isDraft} personnel={personnel} wbsAreas={wbsAreas}
                  overheadRates={overheadRates ?? []}
                  tableMode="other"
                  onUpdate={(data) => updateLineItem.mutate({ id: li.id, data })}
                  onDelete={() => { if (confirm('Delete this line item?')) deleteLineItem.mutate(li.id); }} />
              ))}
              {!lineItemsLoading && otherDirectItems.length === 0 && (
                <tr><td colSpan={isDraft ? 6 : 5} className="px-4 py-8 text-center text-gray-400 text-sm">
                  No other direct cost items. {isDraft ? 'Click "+ Add Line Item" and select "Add Other Line Item".' : ''}
                </td></tr>
              )}
            </tbody>
            {otherDirectItems.length > 0 && (
              <tfoot className="bg-gray-50 border-t">
                <tr>
                  <td colSpan={3} className="px-4 py-3 text-sm font-semibold text-gray-700 text-right">Subtotal (Other Direct)</td>
                  <td className="px-4 py-3 text-sm font-semibold text-nsf-blue text-right">${otherSubtotal.toLocaleString()}</td>
                  <td colSpan={isDraft ? 2 : 1}></td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      </div>

      {/* Table 3: Budget Summary */}
      <BudgetSummary lineItems={lineItems ?? []} overheadRates={overheadRates ?? []} />
    </div>
  );
}

/* ======== Line Item Row (view + inline edit) ======== */

function LineItemRow({
  lineItem, entityType, entityId, budgetId, isDraft,
  personnel, wbsAreas, overheadRates, onUpdate, onDelete,
  tableMode = 'other', salaryEscalationRate = 0, fiscalYear = 1,
  onEffortUpdate,
}: {
  lineItem: BudgetLineItem; entityType: string; entityId: string; budgetId: string;
  isDraft: boolean; personnel: Personnel[]; wbsAreas: WBSArea[]; overheadRates: OverheadRate[];
  onUpdate: (data: Partial<BudgetLineItem>) => void; onDelete: () => void;
  tableMode?: 'personnel' | 'other';
  salaryEscalationRate?: number; fiscalYear?: number;
  onEffortUpdate?: (lineItem: BudgetLineItem, newEffort: number) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [showWbs, setShowWbs] = useState(false);
  const [form, setForm] = useState({
    line_type: lineItem.line_type, description: lineItem.description || '',
    personnel_id: lineItem.personnel_id || '', effort_months: lineItem.effort_months,
    amount: lineItem.amount, overhead_rate_id: lineItem.overhead_rate_id || '',
    notes: lineItem.notes || '',
  });

  const person = personnel.find((p) => p.id === lineItem.personnel_id);
  const rate = overheadRates.find((r) => r.id === lineItem.overhead_rate_id);
  const lineTypeLabel = LINE_TYPES.find((t) => t.value === lineItem.line_type)?.label ?? lineItem.line_type;
  const isPersonnelTable = tableMode === 'personnel';
  const colCount = isPersonnelTable ? (isDraft ? 7 : 6) : (isDraft ? 6 : 5);

  const { data: wbsAllocations } = useQuery({
    queryKey: ['line-item-wbs', lineItem.id],
    queryFn: () => api.budgetLineItems.listWBS(entityType, entityId, budgetId, lineItem.id),
  });

  const wbsTotal = useMemo(() => (wbsAllocations ?? []).reduce((s, a) => s + a.allocation_percent, 0), [wbsAllocations]);
  const hasWbs = (wbsAllocations ?? []).length > 0;
  const wbsComplete = Math.abs(wbsTotal - 100) < 0.01;

  // Compute escalated salary for effort auto-calc
  const escalatedSalary = useMemo(() => {
    if (!person) return 0;
    return person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1);
  }, [person, salaryEscalationRate, fiscalYear]);

  const handleSave = () => {
    if (isPersonnelTable && lineItem.line_type === 'personnel' && onEffortUpdate && form.effort_months !== lineItem.effort_months) {
      onEffortUpdate(lineItem, form.effort_months);
      setEditing(false);
      return;
    }
    onUpdate({ ...form, personnel_id: form.personnel_id || undefined, overhead_rate_id: form.overhead_rate_id || undefined });
    setEditing(false);
  };

  const handleCancel = () => {
    setForm({
      line_type: lineItem.line_type, description: lineItem.description || '',
      personnel_id: lineItem.personnel_id || '', effort_months: lineItem.effort_months,
      amount: lineItem.amount, overhead_rate_id: lineItem.overhead_rate_id || '',
      notes: lineItem.notes || '',
    });
    setEditing(false);
  };

  const handleEffortChange = (months: number) => {
    if (isPersonnelTable && lineItem.line_type === 'personnel') {
      const computed = Math.round((escalatedSalary / 12) * months * 100) / 100;
      setForm({ ...form, effort_months: months, amount: computed });
    } else {
      setForm({ ...form, effort_months: months });
    }
  };

  // --- EDIT MODE ---
  if (editing && isDraft) {
    if (isPersonnelTable) {
      return (
        <>
          <tr className="bg-blue-50/60">
            <td className="px-3 py-2 text-xs text-gray-500">{lineTypeLabel}</td>
            <td className="px-3 py-2">
              <input type="text" value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full border rounded px-2 py-1 text-xs" />
            </td>
            <td className="px-3 py-2 text-xs text-gray-600">{person?.name ?? '—'}</td>
            <td className="px-3 py-2">
              {lineItem.line_type === 'personnel' ? (
                <input type="number" step="0.1" min="0"
                  value={form.effort_months === 0 ? '' : form.effort_months}
                  onChange={(e) => handleEffortChange(parseFloat(e.target.value) || 0)}
                  className="w-full border rounded px-2 py-1 text-xs text-right" />
              ) : <span className="text-xs text-gray-400">—</span>}
            </td>
            <td className="px-3 py-2">
              <div className="border rounded px-2 py-1 text-xs text-right bg-gray-100 text-gray-600">
                ${form.amount.toLocaleString()}
              </div>
            </td>
            <td className="px-3 py-2 text-center">
              <WBSIndicator hasWbs={hasWbs} wbsComplete={wbsComplete} onClick={() => setShowWbs(!showWbs)} />
            </td>
            <td className="px-2 py-2">
              <div className="flex flex-col gap-0.5">
                <button onClick={handleSave} className="text-green-600 text-xs font-medium hover:underline">Save</button>
                <button onClick={handleCancel} className="text-gray-400 text-xs hover:underline">Cancel</button>
              </div>
            </td>
          </tr>
          {showWbs && (
            <tr><td colSpan={colCount} className="px-4 py-3 bg-blue-50/40">
              <WBSAllocationEditor lineItemId={lineItem.id} entityType={entityType} entityId={entityId}
                budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []} />
            </td></tr>
          )}
        </>
      );
    } else {
      // Other items edit mode
      return (
        <>
          <tr className="bg-blue-50/60">
            <td className="px-3 py-2">
              <select value={form.line_type} onChange={(e) => setForm({ ...form, line_type: e.target.value })}
                className="w-full border rounded px-2 py-1 text-xs">
                {LINE_TYPES.filter(t => t.value !== 'personnel' && t.value !== 'fringe').map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </td>
            <td className="px-3 py-2">
              <input type="text" value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full border rounded px-2 py-1 text-xs" />
            </td>
            <td className="px-3 py-2">
              <select value={form.overhead_rate_id}
                onChange={(e) => setForm({ ...form, overhead_rate_id: e.target.value })}
                className="w-full border rounded px-2 py-1 text-xs">
                <option value="">None</option>
                {overheadRates.map((r) => (
                  <option key={r.id} value={r.id}>{r.rate_name} ({(r.rate * 100).toFixed(1)}%)</option>
                ))}
              </select>
            </td>
            <td className="px-3 py-2">
              <CurrencyInput value={form.amount}
                onChange={(val) => setForm({ ...form, amount: val })}
                className="w-full border rounded px-2 py-1 text-xs text-right" />
            </td>
            <td className="px-3 py-2 text-center">
              <WBSIndicator hasWbs={hasWbs} wbsComplete={wbsComplete} onClick={() => setShowWbs(!showWbs)} />
            </td>
            <td className="px-2 py-2">
              <div className="flex flex-col gap-0.5">
                <button onClick={handleSave} className="text-green-600 text-xs font-medium hover:underline">Save</button>
                <button onClick={handleCancel} className="text-gray-400 text-xs hover:underline">Cancel</button>
              </div>
            </td>
          </tr>
          {showWbs && (
            <tr><td colSpan={colCount} className="px-4 py-3 bg-blue-50/40">
              <WBSAllocationEditor lineItemId={lineItem.id} entityType={entityType} entityId={entityId}
                budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []} />
            </td></tr>
          )}
        </>
      );
    }
  }

  // --- VIEW MODE ---
  if (isPersonnelTable) {
    const isSalaryLine = lineItem.line_type === 'personnel';
    return (
      <>
        <tr className={`hover:bg-gray-50 ${isDraft ? 'cursor-pointer' : ''}`}
          onDoubleClick={() => isDraft && isSalaryLine && setEditing(true)}>
          <td className="px-4 py-3 text-sm">{lineTypeLabel}</td>
          <td className="px-4 py-3 text-sm">{lineItem.description || '—'}</td>
          <td className="px-4 py-3 text-sm">{person ? person.name : '—'}</td>
          <td className="px-4 py-3 text-sm text-right">
            {lineItem.effort_months > 0 ? `${lineItem.effort_months.toFixed(1)} mo` : '—'}
          </td>
          <td className="px-4 py-3 text-sm text-right font-medium">${lineItem.amount.toLocaleString()}</td>
          <td className="px-4 py-3 text-center">
            <WBSIndicator hasWbs={hasWbs} wbsComplete={wbsComplete} onClick={() => setShowWbs(!showWbs)} />
          </td>
          {isDraft && (
            <td className="px-2 py-3">
              {isSalaryLine ? (
                <div className="flex gap-1">
                  <button onClick={() => setEditing(true)} className="text-nsf-light hover:underline text-xs">Edit</button>
                  <button onClick={onDelete} className="text-red-400 hover:text-red-600 text-xs" title="Delete salary + fringe for this person">✕</button>
                </div>
              ) : (
                <span className="text-xs text-gray-300" title="Fringe lines are managed with salary">—</span>
              )}
            </td>
          )}
        </tr>
        {showWbs && (
          <tr><td colSpan={colCount} className="px-4 py-3 bg-blue-50/40">
            <WBSAllocationEditor lineItemId={lineItem.id} entityType={entityType} entityId={entityId}
              budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []}
              readOnly={!isDraft} />
          </td></tr>
        )}
      </>
    );
  }

  // Other items view mode
  return (
    <>
      <tr className={`hover:bg-gray-50 ${isDraft ? 'cursor-pointer' : ''}`}
        onDoubleClick={() => isDraft && setEditing(true)}>
        <td className="px-4 py-3 text-sm">{lineTypeLabel}</td>
        <td className="px-4 py-3 text-sm">{lineItem.description || '—'}</td>
        <td className="px-4 py-3 text-sm">
          {rate ? `${rate.rate_name} (${(rate.rate * 100).toFixed(1)}%)` : '—'}
        </td>
        <td className="px-4 py-3 text-sm text-right font-medium">${lineItem.amount.toLocaleString()}</td>
        <td className="px-4 py-3 text-center">
          <WBSIndicator hasWbs={hasWbs} wbsComplete={wbsComplete} onClick={() => setShowWbs(!showWbs)} />
        </td>
        {isDraft && (
          <td className="px-2 py-3">
            <div className="flex gap-1">
              <button onClick={() => setEditing(true)} className="text-nsf-light hover:underline text-xs">Edit</button>
              <button onClick={onDelete} className="text-red-400 hover:text-red-600 text-xs">✕</button>
            </div>
          </td>
        )}
      </tr>
      {showWbs && (
        <tr><td colSpan={colCount} className="px-4 py-3 bg-blue-50/40">
          <WBSAllocationEditor lineItemId={lineItem.id} entityType={entityType} entityId={entityId}
            budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []}
            readOnly={!isDraft} />
        </td></tr>
      )}
    </>
  );
}

/* ======== Budget Summary ======== */

function BudgetSummary({ lineItems, overheadRates }: { lineItems: BudgetLineItem[]; overheadRates: OverheadRate[] }) {
  const summary = useMemo(() => {
    const salaries = lineItems.filter(li => li.line_type === 'personnel').reduce((s, li) => s + li.amount, 0);
    const fringe = lineItems.filter(li => li.line_type === 'fringe').reduce((s, li) => s + li.amount, 0);
    const travel = lineItems.filter(li => li.line_type === 'travel').reduce((s, li) => s + li.amount, 0);
    const equipment = lineItems.filter(li => li.line_type === 'equipment').reduce((s, li) => s + li.amount, 0);
    const supplies = lineItems.filter(li => li.line_type === 'supplies').reduce((s, li) => s + li.amount, 0);
    const contractual = lineItems.filter(li => li.line_type === 'contractual').reduce((s, li) => s + li.amount, 0);
    const other = lineItems.filter(li => li.line_type === 'other').reduce((s, li) => s + li.amount, 0);
    const tuition = lineItems.filter(li => li.line_type === 'tuition').reduce((s, li) => s + li.amount, 0);
    const participantSupport = lineItems.filter(li => li.line_type === 'participant_support').reduce((s, li) => s + li.amount, 0);

    const totalDirect = salaries + fringe + travel + equipment + supplies + contractual + other + tuition + participantSupport;

    // Indirect costs: sum(line.amount * overhead_rate) for lines with overhead rates assigned
    const indirectCosts = lineItems.reduce((sum, li) => {
      if (!li.overhead_rate_id) return sum;
      const rate = overheadRates.find(r => r.id === li.overhead_rate_id);
      return sum + (rate ? Math.round(li.amount * rate.rate * 100) / 100 : 0);
    }, 0);

    return { salaries, fringe, travel, equipment, supplies, contractual, other, tuition, participantSupport, totalDirect, indirectCosts };
  }, [lineItems, overheadRates]);

  if (lineItems.length === 0) return null;

  const rows: [string, number, 'bold' | 'sub' | 'normal'][] = [
    ['A. Salaries & Wages', summary.salaries, 'normal'],
    ['B. Fringe Benefits', summary.fringe, 'normal'],
    ['Total Salaries + Fringe (A+B)', summary.salaries + summary.fringe, 'bold'],
    ['C. Equipment', summary.equipment, 'normal'],
    ['D. Travel', summary.travel, 'normal'],
    ['E. Participant Support', summary.participantSupport, 'normal'],
    ['F. Other Direct Costs', summary.supplies + summary.contractual + summary.other + summary.tuition, 'normal'],
    ['Supplies', summary.supplies, 'sub'],
    ['Contractual', summary.contractual, 'sub'],
    ['Tuition', summary.tuition, 'sub'],
    ['Other', summary.other, 'sub'],
    ['G. Total Direct Costs', summary.totalDirect, 'bold'],
    ['H. Indirect Costs (F&A)', summary.indirectCosts, 'normal'],
    ['I. Total Costs (G+H)', summary.totalDirect + summary.indirectCosts, 'bold'],
  ];

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 mb-2">Budget Summary</h3>
      <div className="bg-white rounded-lg border overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Category</th>
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Amount</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {rows.map(([label, amount, style], i) => (
              <tr key={i} className={style === 'bold' ? 'bg-gray-50' : ''}>
                <td className={`px-4 py-2 text-sm ${
                  style === 'bold' ? 'font-semibold text-gray-800' :
                  style === 'sub' ? 'text-gray-500 pl-10' :
                  'text-gray-700'
                }`}>
                  {label}
                </td>
                <td className={`px-4 py-2 text-sm text-right ${style === 'bold' ? 'font-semibold text-nsf-blue' : ''}`}>
                  ${amount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ======== WBS Incomplete Indicator ======== */

function WBSIndicator({ hasWbs, wbsComplete, onClick }: { hasWbs: boolean; wbsComplete: boolean; onClick: () => void }) {
  if (!hasWbs) {
    return (
      <button onClick={onClick} className="inline-flex items-center gap-1 text-xs text-amber-600 hover:underline"
        title="No WBS allocation — click to assign">
        <span className="inline-block w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
        WBS
      </button>
    );
  }
  if (!wbsComplete) {
    return (
      <button onClick={onClick} className="inline-flex items-center gap-1 text-xs text-amber-600 hover:underline"
        title="WBS allocation does not total 100%">
        <span className="inline-block w-2 h-2 rounded-full bg-amber-400" />
        Incomplete
      </button>
    );
  }
  return (
    <button onClick={onClick} className="inline-flex items-center gap-1 text-xs text-green-600 hover:underline"
      title="WBS allocation is complete (100%)">
      <span className="inline-block w-2 h-2 rounded-full bg-green-500" />
      ✓
    </button>
  );
}

/* ======== WBS Allocation Editor ======== */

function WBSAllocationEditor({
  lineItemId, entityType, entityId, budgetId, wbsAreas, allocations, readOnly = false,
}: {
  lineItemId: string; entityType: string; entityId: string; budgetId: string;
  wbsAreas: WBSArea[]; allocations: BudgetLineItemWBS[]; readOnly?: boolean;
}) {
  const queryClient = useQueryClient();
  const allocMap = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of allocations) m[a.wbs_area_id] = a.allocation_percent;
    return m;
  }, [allocations]);

  const [localAllocs, setLocalAllocs] = useState<Record<string, number>>(allocMap);
  const [dirty, setDirty] = useState(false);
  useMemo(() => { if (!dirty) setLocalAllocs(allocMap); }, [allocMap, dirty]);

  const total = Object.values(localAllocs).reduce((s, v) => s + v, 0);
  const isComplete = Math.abs(total - 100) < 0.01;

  const saveMutation = useMutation({
    mutationFn: () => {
      const allocs = Object.entries(localAllocs).filter(([, pct]) => pct > 0)
        .map(([wbs_area_id, allocation_percent]) => ({ wbs_area_id, allocation_percent }));
      return api.budgetLineItems.setWBS(entityType, entityId, budgetId, lineItemId, allocs);
    },
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['line-item-wbs', lineItemId] }); setDirty(false); },
  });

  if (wbsAreas.length === 0) return <p className="text-xs text-gray-400">No WBS areas defined. Create them in the WBS Areas tab.</p>;

  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-gray-600">WBS Cost Allocation (%)</div>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-2">
        {wbsAreas.map((w) => (
          <div key={w.id} className="flex items-center gap-1.5">
            <label className="text-xs text-gray-600 min-w-[60px]">{w.code}</label>
            <input type="number" min={0} max={100} step={1}
              value={localAllocs[w.id] ?? 0} disabled={readOnly}
              onChange={(e) => { setLocalAllocs({ ...localAllocs, [w.id]: parseFloat(e.target.value) || 0 }); setDirty(true); }}
              className="w-16 border rounded px-1.5 py-0.5 text-xs text-right disabled:bg-gray-100" />
            <span className="text-xs text-gray-400">%</span>
          </div>
        ))}
      </div>
      <div className="flex items-center gap-3">
        <span className={`text-xs font-medium ${isComplete ? 'text-green-600' : 'text-amber-600'}`}>
          Total: {total.toFixed(1)}% {isComplete ? '✓' : '(should be 100%)'}
        </span>
        {!readOnly && (
          <button onClick={() => saveMutation.mutate()} disabled={saveMutation.isPending || !dirty}
            className="px-2 py-1 bg-nsf-light text-white rounded text-xs hover:bg-nsf-blue disabled:opacity-50">
            {saveMutation.isPending ? 'Saving...' : 'Save Allocations'}
          </button>
        )}
      </div>
    </div>
  );
}

/* ======== Line Item Form (with Personnel auto-fringe) ======== */

function LineItemForm({
  personnel, overheadRates, yearFringeRates, fiscalYear, salaryEscalationRate,
  onSubmit, onAddPersonnel, isLoading,
}: {
  personnel: Personnel[]; overheadRates: OverheadRate[];
  yearFringeRates: InstitutionFringeRate[];
  fiscalYear: number; salaryEscalationRate: number;
  onSubmit: (data: Partial<BudgetLineItem>) => void;
  onAddPersonnel: (personId: string, effortMonths: number) => void;
  isLoading: boolean;
}) {
  const [mode, setMode] = useState<'personnel' | 'other'>('personnel');
  const [personId, setPersonId] = useState('');
  const [effortMonths, setEffortMonths] = useState(0);
  const [form, setForm] = useState({
    line_type: 'travel', description: '', amount: 0, overhead_rate_id: '', notes: '',
  });

  const selectedPerson = personnel.find((p) => p.id === personId);
  const escalatedSalary = selectedPerson ? selectedPerson.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1) : 0;
  const computedSalary = selectedPerson ? Math.round((escalatedSalary / 12) * effortMonths * 100) / 100 : 0;

  return (
    <div className="bg-white p-6 rounded-lg border space-y-4">
      <div className="flex gap-3 border-b pb-3">
        <button onClick={() => setMode('personnel')}
          className={`px-3 py-1.5 rounded text-sm font-medium ${mode === 'personnel' ? 'bg-nsf-blue text-white' : 'bg-gray-100 text-gray-600'}`}>
          Add Personnel (+ Fringe)
        </button>
        <button onClick={() => setMode('other')}
          className={`px-3 py-1.5 rounded text-sm font-medium ${mode === 'other' ? 'bg-nsf-blue text-white' : 'bg-gray-100 text-gray-600'}`}>
          Add Other Line Item
        </button>
      </div>

      {mode === 'personnel' && (
        <div className="space-y-3">
          <p className="text-sm text-gray-500">
            Select a person and effort. Salary is computed from annual salary and fringe lines are
            auto-created from the institution&apos;s fringe rates for this year.
          </p>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Person</label>
              <select value={personId} onChange={(e) => setPersonId(e.target.value)}
                className="w-full border rounded-md px-3 py-2 text-sm">
                <option value="">— Select —</option>
                {personnel.map((p) => (
                  <option key={p.id} value={p.id}>{p.name} ({p.role}) — ${p.annual_salary.toLocaleString()}/yr</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Effort (months)</label>
              <input type="number" step="0.1" min="0"
                value={effortMonths === 0 ? '' : effortMonths}
                onChange={(e) => setEffortMonths(parseFloat(e.target.value) || 0)}
                className="w-full border rounded-md px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Computed Salary</label>
              <div className="border rounded-md px-3 py-2 text-sm bg-gray-50 font-medium">
                ${computedSalary.toLocaleString()}
              </div>
            </div>
          </div>
          {yearFringeRates.length > 0 && (
            <div className="bg-blue-50 rounded p-3">
              <p className="text-xs font-medium text-blue-700 mb-1">Fringe lines that will be auto-created:</p>
              {yearFringeRates.map((fr) => (
                <div key={fr.id} className="text-xs text-blue-600">
                  {fr.rate_name}: {(fr.rate * 100).toFixed(2)}% = ${(computedSalary * fr.rate).toLocaleString()}
                </div>
              ))}
            </div>
          )}
          {yearFringeRates.length === 0 && (
            <p className="text-xs text-amber-600">
              No fringe rates defined for this year/institution. Add them on the Institutions page first.
            </p>
          )}
          <button onClick={() => { if (personId && effortMonths > 0) onAddPersonnel(personId, effortMonths); }}
            disabled={!personId || effortMonths <= 0 || isLoading}
            className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue disabled:opacity-50">
            {isLoading ? 'Creating...' : 'Add Personnel + Fringe'}
          </button>
        </div>
      )}

      {mode === 'other' && (
        <form onSubmit={(e) => {
          e.preventDefault();
          onSubmit({ ...form, overhead_rate_id: form.overhead_rate_id || undefined });
        }} className="space-y-3">
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
              <select value={form.line_type} onChange={(e) => setForm({ ...form, line_type: e.target.value })}
                className="w-full border rounded-md px-3 py-2 text-sm">
                {LINE_TYPES.filter((t) => t.value !== 'personnel' && t.value !== 'fringe').map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
              <input type="text" value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full border rounded-md px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Amount ($)</label>
              <CurrencyInput value={form.amount} required
                onChange={(val) => setForm({ ...form, amount: val })}
                className="w-full border rounded-md px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Overhead Rate</label>
              <select value={form.overhead_rate_id}
                onChange={(e) => setForm({ ...form, overhead_rate_id: e.target.value })}
                className="w-full border rounded-md px-3 py-2 text-sm">
                <option value="">— None (no overhead) —</option>
                {overheadRates.map((r) => (
                  <option key={r.id} value={r.id}>{r.rate_name} ({(r.rate * 100).toFixed(1)}%)</option>
                ))}
              </select>
            </div>
            <div className="col-span-2">
              <label className="block text-sm font-medium text-gray-700 mb-1">Notes</label>
              <input type="text" value={form.notes}
                onChange={(e) => setForm({ ...form, notes: e.target.value })}
                className="w-full border rounded-md px-3 py-2 text-sm" />
            </div>
          </div>
          <button type="submit" disabled={isLoading}
            className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue disabled:opacity-50">
            {isLoading ? 'Creating...' : 'Add Line Item'}
          </button>
        </form>
      )}
    </div>
  );
}

/* ======== WBS Area Form ======== */

function WBSForm({ onSubmit, isLoading }: { onSubmit: (data: Partial<WBSArea>) => void; isLoading: boolean }) {
  const [form, setForm] = useState({ code: '', name: '', description: '', budget: 0 });
  return (
    <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="bg-white p-6 rounded-lg border space-y-4">
      <h2 className="font-semibold text-lg">New WBS Area</h2>
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Code</label>
          <input type="text" required value={form.code}
            onChange={(e) => setForm({ ...form, code: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" placeholder="e.g. 1.1" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
          <input type="text" required value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <input type="text" value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Total Budget ($)</label>
          <CurrencyInput value={form.budget}
            onChange={(val) => setForm({ ...form, budget: val })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
      </div>
      <button type="submit" disabled={isLoading}
        className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue disabled:opacity-50">
        {isLoading ? 'Creating...' : 'Create WBS Area'}
      </button>
    </form>
  );
}

/* ======== NSF 1030 Dropdown ======== */

function NSF1030Dropdown({ entityType, entityId, fiscalYear }: { entityType: string; entityId: string; fiscalYear: number }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  return (
    <div className="relative" ref={ref}>
      <button onClick={() => setOpen(!open)}
        className="px-3 py-1.5 bg-gray-700 text-white rounded text-xs hover:bg-gray-800">
        NSF 1030 ▾
      </button>
      {open && (
        <div className="absolute right-0 mt-1 w-48 bg-white border rounded shadow-lg z-10 text-sm">
          <a href={api.nsf1030.url(entityType, entityId, fiscalYear)} target="_blank" rel="noopener noreferrer"
            className="block px-4 py-2 hover:bg-gray-100" onClick={() => setOpen(false)}>
            Year {fiscalYear} Only
          </a>
          <a href={api.nsf1030.url(entityType, entityId)} target="_blank" rel="noopener noreferrer"
            className="block px-4 py-2 hover:bg-gray-100" onClick={() => setOpen(false)}>
            All Years + Cumulative
          </a>
        </div>
      )}
    </div>
  );
}
