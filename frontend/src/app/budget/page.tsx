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
  Subaward,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { useState, useMemo, useCallback, Suspense, useRef, useEffect } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import Link from 'next/link';
import CurrencyInput from '@/components/CurrencyInput';
import { WBSAllocEditor } from '@/components/WBSAllocationEditor';
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
  const { isSubawardAdmin, permittedInstitutions } = useAuth();
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const initialYear = searchParams.get('year');
  const initialEntity = searchParams.get('entity');
  const [selectedYear, setSelectedYear] = useState<number>(initialYear ? parseInt(initialYear) : 1);

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

  const allInstitutions = useMemo<InstitutionOption[]>(() => {
    const opts: InstitutionOption[] = [];
    if (grant) opts.push({ entityType: 'grant', entityId: grant.id, label: `${grant.institution} (Lead)`, salaryEscalationRate: grant.salary_escalation_rate ?? 0 });
    for (const s of subawards ?? []) opts.push({ entityType: 'subaward', entityId: s.id, label: s.institution, salaryEscalationRate: s.salary_escalation_rate ?? 0 });
    return opts;
  }, [grant, subawards]);

  // Subaward admins only see their permitted institutions
  const institutions = isSubawardAdmin
    ? allInstitutions.filter((i) => permittedInstitutions.some((p) => i.label.startsWith(p)))
    : allInstitutions;

  const [selectedInst, setSelectedInst] = useState<string>(initialEntity ?? '');
  const activeInst = institutions.find((i) => `${i.entityType}:${i.entityId}` === selectedInst) || institutions[0];

  // Sync selection to URL so page reloads preserve the chosen institution & year
  useEffect(() => {
    const key = activeInst ? `${activeInst.entityType}:${activeInst.entityId}` : '';
    const params = new URLSearchParams();
    if (key) params.set('entity', key);
    if (selectedYear !== 1) params.set('year', String(selectedYear));
    const qs = params.toString();
    const target = qs ? `${pathname}?${qs}` : pathname;
    if (target !== `${pathname}${searchParams.toString() ? '?' + searchParams.toString() : ''}`) {
      router.replace(target, { scroll: false });
    }
  }, [activeInst, selectedYear, pathname, router, searchParams]);

  // For subaward admins, only show personnel from their permitted institutions
  const visiblePersonnel = useMemo(() => {
    const all = personnel ?? [];
    if (!isSubawardAdmin || permittedInstitutions.length === 0) return all;
    return all.filter((p) => p.institution && permittedInstitutions.includes(p.institution));
  }, [personnel, isSubawardAdmin, permittedInstitutions]);

  if (grantLoading || wbsLoading) return <div className="p-4">Loading budget...</div>;
  if (!grantId) return <div className="p-4">No project configured. <Link href="/settings" className="text-nsf-light hover:underline">Set up project</Link></div>;

  return (
    <div className="max-w-6xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">Detailed Budget</h1>
        <p className="text-sm text-gray-500">Manage institution budgets by year with versioned line items</p>
      </div>

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
              wbsAreas={wbsAreas ?? []} personnel={visiblePersonnel}
              subawards={subawards ?? []} />
          )}
        </div>
    </div>
  );
}

/* ======== Institution Budget Panel ======== */

function InstitutionBudgetPanel({
  entityType, entityId, institutionLabel, fiscalYear, salaryEscalationRate, wbsAreas, personnel, subawards,
}: {
  entityType: string; entityId: string; institutionLabel: string;
  fiscalYear: number; salaryEscalationRate: number; wbsAreas: WBSArea[]; personnel: Personnel[];
  subawards: Subaward[];
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

  const handleAddPersonnel = useCallback(async (personId: string, effortMonths: number, selectedFringeRateIds: string[], overheadRateId: string | null) => {
    const person = personnel.find((p) => p.id === personId);
    if (!person || !budget) return;
    // Apply salary escalation: Y1 = base, Y2 = base*(1+rate), etc.
    const escalatedSalary = person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1);
    const monthlySalary = escalatedSalary / 12;
    const exactSalary = monthlySalary * effortMonths;
    const salaryAmount = Math.round(exactSalary);

    const salaryLine = await api.budgetLineItems.create(entityType, entityId, budget.id, {
      line_type: 'personnel', description: `${person.name} — Salary`,
      personnel_id: personId, effort_months: effortMonths, amount: salaryAmount,
      overhead_rate_id: overheadRateId,
    });

    // Fetch salary WBS allocations to copy to fringe lines
    const salaryWbs = await api.budgetLineItems.listWBS(entityType, entityId, budget.id, salaryLine.id);

    const selectedFringes = yearFringeRates.filter(fr => selectedFringeRateIds.includes(fr.id));
    for (const fr of selectedFringes) {
      // Fringe calculated on unrounded salary, then rounded to whole dollars
      const fringeAmount = Math.round(exactSalary * fr.rate);
      const fringeLine = await api.budgetLineItems.create(entityType, entityId, budget.id, {
        line_type: 'fringe', description: `${person.name} — ${fr.rate_name}`,
        personnel_id: personId, amount: fringeAmount,
        overhead_rate_id: salaryLine.overhead_rate_id,
        notes: `${(fr.rate * 100).toFixed(2)}% of $${salaryAmount.toLocaleString()}`,
      });
      if (salaryWbs.length > 0) {
        await api.budgetLineItems.setWBS(entityType, entityId, budget.id, fringeLine.id,
          salaryWbs.map(w => ({ wbs_area_id: w.wbs_area_id, allocation_percent: w.allocation_percent })));
      }
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
    const exactSalary = (escalatedSalary / 12) * newEffort;
    const newSalaryAmount = Math.round(exactSalary);

    await api.budgetLineItems.update(entityType, entityId, budget.id, salaryLineItem.id, {
      effort_months: newEffort, amount: newSalaryAmount,
    });

    const fringeItems = (lineItems ?? []).filter(
      li => li.line_type === 'fringe' && li.personnel_id === salaryLineItem.personnel_id
    );
    for (const fli of fringeItems) {
      const matchingRate = yearFringeRates.find(fr => fli.description?.includes(fr.rate_name));
      if (matchingRate) {
        // Fringe calculated on unrounded salary, then rounded to whole dollars
        const newFringeAmount = Math.round(exactSalary * matchingRate.rate);
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

  // Add missing fringe lines for a person who has a salary but no (or partial) fringe
  const handleAddMissingFringe = useCallback(async (personnelId: string) => {
    if (!budget) return;
    const salaryLine = (lineItems ?? []).find(
      li => li.line_type === 'personnel' && li.personnel_id === personnelId
    );
    if (!salaryLine) return;
    const existingFringe = (lineItems ?? []).filter(
      li => li.line_type === 'fringe' && li.personnel_id === personnelId
    );
    const person = personnel.find(p => p.id === personnelId);
    // Recompute exact (unrounded) salary for fringe calculation
    const exactSalary = person && salaryLine.effort_months > 0
      ? (person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1) / 12) * salaryLine.effort_months
      : salaryLine.amount;

    // Fetch salary WBS allocations to copy to new fringe lines
    const salaryWbs = await api.budgetLineItems.listWBS(entityType, entityId, budget.id, salaryLine.id);

    for (const fr of yearFringeRates) {
      const alreadyExists = existingFringe.some(ef => ef.description?.includes(fr.rate_name));
      if (!alreadyExists) {
        // Fringe calculated on unrounded salary, then rounded to whole dollars
        const fringeAmount = Math.round(exactSalary * fr.rate);
        const fringeLine = await api.budgetLineItems.create(entityType, entityId, budget.id, {
          line_type: 'fringe',
          description: person ? `${person.name} — ${fr.rate_name}` : fr.rate_name,
          personnel_id: personnelId,
          amount: fringeAmount,
          overhead_rate_id: salaryLine.overhead_rate_id,
          notes: `${(fr.rate * 100).toFixed(2)}% of $${salaryLine.amount.toLocaleString()}`,
        });
        if (salaryWbs.length > 0) {
          await api.budgetLineItems.setWBS(entityType, entityId, budget.id, fringeLine.id,
            salaryWbs.map(w => ({ wbs_area_id: w.wbs_area_id, allocation_percent: w.allocation_percent })));
        }
      }
    }
    queryClient.invalidateQueries({ queryKey: lineItemsKey });
  }, [budget, lineItems, personnel, yearFringeRates, entityType, entityId, queryClient, lineItemsKey, salaryEscalationRate, fiscalYear]);

  // Group fringe items immediately after their associated salary item
  const personnelAndFringeItems = useMemo(() => {
    const items = (lineItems ?? []).filter(li => li.line_type === 'personnel' || li.line_type === 'fringe');
    const salaryItems = items.filter(li => li.line_type === 'personnel');
    const fringeItems = items.filter(li => li.line_type === 'fringe');
    const grouped: BudgetLineItem[] = [];
    for (const sal of salaryItems) {
      grouped.push(sal);
      grouped.push(...fringeItems.filter(f => f.personnel_id === sal.personnel_id));
    }
    // Include any fringe without a matching salary (orphans)
    const usedIds = new Set(grouped.map(g => g.id));
    grouped.push(...fringeItems.filter(f => !usedIds.has(f.id)));
    return grouped;
  }, [lineItems]);

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
          currentInstitution={institutionLabel.replace(/ \(Lead\)$/, '')}
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
                <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">F&A Rate</th>
                <th className="text-center px-4 py-3 text-xs font-medium text-gray-600 uppercase">WBS</th>
                {isDraft && <th className="px-2 py-3 w-16"></th>}
              </tr>
            </thead>
            <tbody className="divide-y">
              {lineItemsLoading && (
                <tr><td colSpan={isDraft ? 8 : 7} className="px-4 py-4 text-gray-400 text-sm">Loading...</td></tr>
              )}
              {!lineItemsLoading && personnelAndFringeItems.map((li) => (
                <LineItemRow key={li.id} lineItem={li}
                  entityType={entityType} entityId={entityId} budgetId={budget.id}
                  isDraft={isDraft} personnel={personnel} wbsAreas={wbsAreas}
                  overheadRates={overheadRates ?? []}
                  yearFringeRates={yearFringeRates}
                  allLineItems={lineItems ?? []}
                  tableMode="personnel"
                  salaryEscalationRate={salaryEscalationRate} fiscalYear={fiscalYear}
                  onUpdate={(data) => updateLineItem.mutate({ id: li.id, data })}
                  onEffortUpdate={handleEffortUpdate}
                  onAddMissingFringe={handleAddMissingFringe}
                  onDelete={() => { if (li.personnel_id) handleDeletePersonnelBundle(li.personnel_id); else deleteLineItem.mutate(li.id); }} />
              ))}
              {!lineItemsLoading && personnelAndFringeItems.length === 0 && (
                <tr><td colSpan={isDraft ? 8 : 7} className="px-4 py-8 text-center text-gray-400 text-sm">
                  No personnel line items. {isDraft ? 'Use "+ Add Line Item" → "Add Personnel + Fringe" above.' : ''}
                </td></tr>
              )}
            </tbody>
            {personnelAndFringeItems.length > 0 && (
              <tfoot className="bg-gray-50 border-t">
                <tr>
                  <td colSpan={4} className="px-4 py-3 text-sm font-semibold text-gray-700 text-right">Subtotal (Personnel + Fringe)</td>
                  <td className="px-4 py-3 text-sm font-semibold text-nsf-blue text-right">${personnelSubtotal.toLocaleString()}</td>
                  <td colSpan={isDraft ? 3 : 2}></td>
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

      {/* Indirect Costs Breakdown */}
      <IndirectCostsTable lineItems={lineItems ?? []} overheadRates={overheadRates ?? []}
        entityType={entityType} fiscalYear={fiscalYear} subawards={subawards} />

      {/* Table 3: Budget Summary */}
      <BudgetSummary lineItems={lineItems ?? []} overheadRates={overheadRates ?? []}
        entityType={entityType} fiscalYear={fiscalYear} subawards={subawards} />
    </div>
  );
}

/* ======== Line Item Row (view + inline edit) ======== */

function LineItemRow({
  lineItem, entityType, entityId, budgetId, isDraft,
  personnel, wbsAreas, overheadRates, onUpdate, onDelete,
  tableMode = 'other', salaryEscalationRate = 0, fiscalYear = 1,
  onEffortUpdate, yearFringeRates = [], allLineItems = [],
  onAddMissingFringe,
}: {
  lineItem: BudgetLineItem; entityType: string; entityId: string; budgetId: string;
  isDraft: boolean; personnel: Personnel[]; wbsAreas: WBSArea[]; overheadRates: OverheadRate[];
  onUpdate: (data: Partial<BudgetLineItem>) => void; onDelete: () => void;
  tableMode?: 'personnel' | 'other';
  salaryEscalationRate?: number; fiscalYear?: number;
  onEffortUpdate?: (lineItem: BudgetLineItem, newEffort: number) => void;
  yearFringeRates?: InstitutionFringeRate[];
  allLineItems?: BudgetLineItem[];
  onAddMissingFringe?: (personnelId: string) => void;
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
  const colCount = isPersonnelTable ? (isDraft ? 8 : 7) : (isDraft ? 6 : 5);

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
      // Effort changed: cascade update to salary + all fringe lines
      onEffortUpdate(lineItem, form.effort_months);
      // Also save non-effort fields (description, overhead_rate_id) if changed
      const nonEffortChanges: Partial<BudgetLineItem> = {};
      if (form.description !== (lineItem.description || '')) nonEffortChanges.description = form.description;
      if ((form.overhead_rate_id || null) !== (lineItem.overhead_rate_id || null)) nonEffortChanges.overhead_rate_id = form.overhead_rate_id || null;
      if (Object.keys(nonEffortChanges).length > 0) onUpdate(nonEffortChanges);
    } else {
      onUpdate({ ...form, personnel_id: form.personnel_id || undefined, overhead_rate_id: form.overhead_rate_id || null });
    }
    setEditing(false);
  };

  // For salary lines: recalculate from the person's current annual_salary
  const handleUpdateFromLatestSalary = () => {
    if (!person || lineItem.line_type !== 'personnel') return;
    const newEscalated = person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1);
    const newAmount = Math.round((newEscalated / 12) * form.effort_months);
    setForm({ ...form, amount: newAmount });
  };

  // For fringe lines: find matching fringe rate from institution rates
  const matchedFringeRate = useMemo(() => {
    if (lineItem.line_type !== 'fringe') return undefined;
    // Try exact name match first
    const byName = yearFringeRates.find(fr => lineItem.description?.includes(fr.rate_name));
    if (byName) return byName;
    // Fall back to the only rate if there's just one
    if (yearFringeRates.length === 1) return yearFringeRates[0];
    return undefined;
  }, [lineItem, yearFringeRates]);

  // Compute the salary amount for this person (from the salary line item)
  const salaryLine = useMemo(() => {
    if (lineItem.line_type !== 'fringe' || !lineItem.personnel_id) return undefined;
    return allLineItems.find(
      li => li.line_type === 'personnel' && li.personnel_id === lineItem.personnel_id
    );
  }, [lineItem, allLineItems]);
  const personnelSalaryAmount = salaryLine?.amount ?? 0;

  // Compute exact (unrounded) salary for fringe calculation
  const exactSalaryForFringe = useMemo(() => {
    if (!salaryLine || !person || salaryLine.effort_months <= 0) return personnelSalaryAmount;
    return (person.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1) / 12) * salaryLine.effort_months;
  }, [salaryLine, person, personnelSalaryAmount, salaryEscalationRate, fiscalYear]);

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
    if (isPersonnelTable && lineItem.line_type === 'personnel') {
      return (
        <>
          <tr className="bg-blue-50/60">
            <td className="px-3 py-2 text-xs text-gray-500">{lineTypeLabel}</td>
            <td className="px-3 py-2">
              <input type="text" value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                className="w-full border rounded px-2 py-1 text-xs" />
            </td>
            <td className="px-3 py-2 text-xs text-gray-600">
              <div className="flex items-center gap-1">
                {person?.name ?? '—'}
                {person && (
                  <a href={`/personnel?person=${person.id}`} target="_blank" rel="noopener noreferrer"
                    className="text-nsf-light hover:text-nsf-blue" title="Edit personnel details">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                    </svg>
                  </a>
                )}
              </div>
            </td>
            <td className="px-3 py-2">
              <input type="number" step="0.1" min="0"
                value={form.effort_months === 0 ? '' : form.effort_months}
                onChange={(e) => handleEffortChange(parseFloat(e.target.value) || 0)}
                className="w-full border rounded px-2 py-1 text-xs text-right" />
            </td>
            <td className="px-3 py-2">
              <div className="flex flex-col gap-1">
                <div className="border rounded px-2 py-1 text-xs text-right bg-gray-100 text-gray-600">
                  ${form.amount.toLocaleString()}
                </div>
                {person && (
                  <div className="text-[10px] text-gray-500 text-right">
                    Using ${escalatedSalary.toLocaleString(undefined, { maximumFractionDigits: 0 })}/yr
                    {fiscalYear > 1 && salaryEscalationRate > 0 && (
                      <span> (base ${person.annual_salary.toLocaleString()} + {(salaryEscalationRate * 100).toFixed(1)}% × {fiscalYear - 1}yr)</span>
                    )}
                  </div>
                )}
                {person && (
                  <button onClick={handleUpdateFromLatestSalary}
                    className="text-[10px] text-nsf-light hover:underline text-left"
                    title={`Recalculate from current salary: $${person.annual_salary.toLocaleString()}/yr`}>
                    ↻ Update from latest salary
                  </button>
                )}
              </div>
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
                budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []} onSaved={() => setShowWbs(false)} />
            </td></tr>
          )}
        </>
      );
    } else if (isPersonnelTable && lineItem.line_type === 'fringe') {
      // Fringe line edit mode: allow changing the fringe rate
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
              {yearFringeRates.length > 1 ? (
                <select
                  value={matchedFringeRate?.id ?? ''}
                  onChange={(e) => {
                    const newRate = yearFringeRates.find(fr => fr.id === e.target.value);
                    if (newRate && personnelSalaryAmount > 0) {
                      const newAmount = Math.round(personnelSalaryAmount * newRate.rate * 100) / 100;
                      setForm({
                        ...form,
                        description: person ? `${person.name} — ${newRate.rate_name}` : newRate.rate_name,
                        amount: newAmount,
                        notes: `${(newRate.rate * 100).toFixed(2)}% of $${personnelSalaryAmount.toLocaleString()}`,
                      });
                    }
                  }}
                  className="w-full border rounded px-2 py-1 text-xs">
                  {yearFringeRates.map((fr) => (
                    <option key={fr.id} value={fr.id}>{fr.rate_name} ({(fr.rate * 100).toFixed(2)}%)</option>
                  ))}
                </select>
              ) : (
                <span className="text-xs text-gray-400">
                  {matchedFringeRate ? `${(matchedFringeRate.rate * 100).toFixed(2)}%` : '—'}
                </span>
              )}
            </td>
            <td className="px-3 py-2">
              <div className="flex flex-col gap-1">
                <div className="border rounded px-2 py-1 text-xs text-right bg-gray-100 text-gray-600">
                  ${form.amount.toLocaleString()}
                </div>
                {matchedFringeRate && personnelSalaryAmount > 0 && (() => {
                  // Fringe calculated on unrounded salary, then rounded to whole dollars
                  const expected = Math.round(exactSalaryForFringe * matchedFringeRate.rate);
                  const isStale = Math.abs(form.amount - expected) > 0.5;
                  return (
                    <div className="text-[10px] text-right">
                      <span className="text-gray-400">{(matchedFringeRate.rate * 100).toFixed(2)}% of ${personnelSalaryAmount.toLocaleString()}</span>
                      {isStale && (
                        <button onClick={() => setForm({
                          ...form, amount: expected,
                          notes: `${(matchedFringeRate.rate * 100).toFixed(2)}% of $${personnelSalaryAmount.toLocaleString()}`,
                        })}
                          className="ml-1 text-nsf-light hover:underline">
                          ↻ Recalculate
                        </button>
                      )}
                    </div>
                  );
                })()}
              </div>
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
                budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []}
                salaryLineItemId={salaryLine?.id} onSaved={() => setShowWbs(false)} />
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
                budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []} onSaved={() => setShowWbs(false)} />
            </td></tr>
          )}
        </>
      );
    }
  }

  // --- VIEW MODE ---
  if (isPersonnelTable) {
    const isSalaryLine = lineItem.line_type === 'personnel';
    // Check if stored amount matches what the formula would produce (whole-dollar rounding)
    const expectedSalaryAmount = isSalaryLine && person
      ? Math.round((escalatedSalary / 12) * lineItem.effort_months)
      : null;
    const isSalaryStale = expectedSalaryAmount !== null && Math.abs(lineItem.amount - expectedSalaryAmount) > 0.5;
    // Fringe: calculated on unrounded salary, then rounded to whole dollars
    const expectedFringeAmount = !isSalaryLine && matchedFringeRate && personnelSalaryAmount > 0
      ? Math.round(exactSalaryForFringe * matchedFringeRate.rate)
      : null;
    const isFringeStale = expectedFringeAmount !== null && Math.abs(lineItem.amount - expectedFringeAmount) > 0.5;
    return (
      <>
        <tr className={`hover:bg-gray-50 ${isDraft ? 'cursor-pointer' : ''}`}
          onDoubleClick={() => isDraft && setEditing(true)}>
          <td className="px-4 py-3 text-sm">{lineTypeLabel}</td>
          <td className="px-4 py-3 text-sm">{lineItem.description || '—'}</td>
          <td className="px-4 py-3 text-sm">
            <div className="flex items-center gap-1">
              {person ? person.name : '—'}
              {person && isSalaryLine && (
                <a href={`/personnel?person=${person?.id}`} target="_blank" rel="noopener noreferrer"
                  className="text-gray-400 hover:text-nsf-light" title="Edit personnel details"
                  onClick={(e) => e.stopPropagation()}>
                  <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                  </svg>
                </a>
              )}
            </div>
          </td>
          <td className="px-4 py-3 text-sm text-right">
            {lineItem.effort_months > 0 ? `${lineItem.effort_months.toFixed(1)} mo` : (
              matchedFringeRate ? (
                <span className="text-gray-500 text-xs">{(matchedFringeRate.rate * 100).toFixed(2)}%</span>
              ) : '—'
            )}
          </td>
          <td className="px-4 py-3 text-sm text-right font-medium">
            <span className={(isSalaryStale || isFringeStale) ? 'text-amber-600' : ''}>${lineItem.amount.toLocaleString()}</span>
            {isSalaryStale && (
              <div className="text-[10px] text-amber-600 font-normal" title={`Expected $${expectedSalaryAmount!.toLocaleString()} from current salary`}>
                ⚠ expected ${expectedSalaryAmount!.toLocaleString()}
              </div>
            )}
            {isFringeStale && (
              <div className="text-[10px] text-amber-600 font-normal" title={`Expected $${expectedFringeAmount!.toLocaleString()} from current salary × rate`}>
                ⚠ expected ${expectedFringeAmount!.toLocaleString()}
              </div>
            )}
          </td>
          <td className="px-4 py-3 text-sm text-gray-500">
            {rate ? `${rate.rate_name} (${(rate.rate * 100).toFixed(1)}%)` : <span className="text-gray-300">—</span>}
          </td>
          <td className="px-4 py-3 text-center">
            <WBSIndicator hasWbs={hasWbs} wbsComplete={wbsComplete} onClick={() => setShowWbs(!showWbs)} />
          </td>
          {isDraft && (
            <td className="px-2 py-3">
              {isSalaryLine ? (() => {
                const existingFringe = allLineItems.filter(
                  li => li.line_type === 'fringe' && li.personnel_id === lineItem.personnel_id
                );
                const hasMissingFringe = yearFringeRates.some(
                  fr => !existingFringe.some(ef => ef.description?.includes(fr.rate_name))
                );
                return (
                  <div className="flex flex-col gap-0.5">
                    <div className="flex gap-1">
                      <button onClick={() => setEditing(true)} className="text-nsf-light hover:underline text-xs">Edit</button>
                      <button onClick={onDelete} className="text-red-400 hover:text-red-600 text-xs" title="Delete salary + fringe for this person">✕</button>
                    </div>
                    {hasMissingFringe && onAddMissingFringe && lineItem.personnel_id && (
                      <button onClick={() => onAddMissingFringe(lineItem.personnel_id!)}
                        className="text-[10px] text-amber-600 hover:underline whitespace-nowrap" title="Add missing fringe line items">
                        + Add Fringe
                      </button>
                    )}
                  </div>
                );
              })() : (
                <div className="flex gap-1">
                  <button onClick={() => setEditing(true)} className="text-nsf-light hover:underline text-xs" title="Edit fringe line">Edit</button>
                  {!lineItem.personnel_id && (
                    <button onClick={onDelete} className="text-red-400 hover:text-red-600 text-xs" title="Delete orphaned fringe line">✕</button>
                  )}
                </div>
              )}
            </td>
          )}
        </tr>
        {showWbs && (
          <tr><td colSpan={colCount} className="px-4 py-3 bg-blue-50/40">
            <WBSAllocationEditor lineItemId={lineItem.id} entityType={entityType} entityId={entityId}
              budgetId={budgetId} wbsAreas={wbsAreas} allocations={wbsAllocations ?? []}
              readOnly={!isDraft}
              salaryLineItemId={lineItem.line_type === 'fringe' ? salaryLine?.id : undefined}
              onSaved={() => setShowWbs(false)} />
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
            readOnly={!isDraft}
            salaryLineItemId={lineItem.line_type === 'fringe' ? salaryLine?.id : undefined}
            onSaved={() => setShowWbs(false)} />
        </td></tr>
      )}
    </>
  );
}

/* ======== Indirect Costs Breakdown ======== */

function IndirectCostsTable({
  lineItems, overheadRates, entityType, fiscalYear, subawards,
}: {
  lineItems: BudgetLineItem[]; overheadRates: OverheadRate[];
  entityType: string; fiscalYear: number; subawards: Subaward[];
}) {
  // For lead institution (grant), load subaward Y1 budget totals (from line items) to compute MTDC
  const isLead = entityType === 'grant';
  const { data: subawardY1Totals } = useQuery({
    queryKey: ['subaward-y1-totals', subawards.map(s => s.id).join(',')],
    queryFn: async () => {
      const totals: { entityId: string; total: number }[] = [];
      for (const sub of subawards) {
        const budgets = await api.institutionBudgets.list('subaward', sub.id, true);
        const y1 = budgets.find(b => b.fiscal_year === 1);
        if (!y1) { totals.push({ entityId: sub.id, total: 0 }); continue; }
        const items = await api.budgetLineItems.list('subaward', sub.id, y1.id);
        const lineItemTotal = items.reduce((s, li) => s + li.amount, 0);
        totals.push({ entityId: sub.id, total: lineItemTotal });
      }
      return totals;
    },
    enabled: isLead && subawards.length > 0,
  });

  const rateMap = useMemo(() => {
    const m = new Map<string, OverheadRate>();
    for (const r of overheadRates) m.set(r.id, r);
    return m;
  }, [overheadRates]);

  const indirectRows = useMemo(() => {
    // Accumulate base per overhead rate from line items
    const bases = new Map<string, number>(); // rateId -> base
    for (const li of lineItems) {
      if (!li.overhead_rate_id) continue;
      const prev = bases.get(li.overhead_rate_id) ?? 0;
      bases.set(li.overhead_rate_id, prev + Math.round(li.amount));
    }

    // For lead institution, compute subaward MTDC (first $25K of each subaward, year 1 only)
    let subawardMTDCBase = 0;
    if (isLead && fiscalYear === 1 && subawardY1Totals) {
      const MTDC_CAP = 25000;
      for (const entry of subawardY1Totals) {
        if (entry.total > 0) {
          subawardMTDCBase += Math.min(entry.total, MTDC_CAP);
        }
      }
    }

    // Check if subaward MTDC indirect is already included in the budget line items
    // by looking for a line item that appears to represent subaward MTDC indirect
    let subawardMTDCAlreadyIncluded = false;
    for (const li of lineItems) {
      const desc = (li.description ?? '').toLowerCase();
      if (desc.includes('subaward') && desc.includes('mtdc') || desc.includes('subaward') && desc.includes('indirect')) {
        subawardMTDCAlreadyIncluded = true;
        break;
      }
    }

    // Build rows per rate
    const rows: { rateName: string; ratePercent: number; base: number; indirect: number }[] = [];
    for (const [rateId, base] of bases) {
      const rate = rateMap.get(rateId);
      if (!rate) continue;
      rows.push({
        rateName: rate.rate_name,
        ratePercent: rate.rate * 100,
        base,
        indirect: Math.round(base * rate.rate),
      });
    }

    // Add subaward MTDC row for lead institution year 1 if not already in budget
    if (isLead && fiscalYear === 1 && subawardMTDCBase > 0 && !subawardMTDCAlreadyIncluded) {
      // Use the rate with the largest base (same logic as backend)
      let bestRateId = '';
      let bestBase = -1;
      for (const [rateId, base] of bases) {
        if (base > bestBase) { bestBase = base; bestRateId = rateId; }
      }
      const bestRate = bestRateId ? rateMap.get(bestRateId) : overheadRates[0];
      if (bestRate) {
        const mtdcIndirect = Math.round(subawardMTDCBase * bestRate.rate);
        rows.push({
          rateName: `Subaward MTDC (first $25K each, ${bestRate.rate_name})`,
          ratePercent: bestRate.rate * 100,
          base: subawardMTDCBase,
          indirect: mtdcIndirect,
        });
      }
    }

    return rows;
  }, [lineItems, rateMap, isLead, fiscalYear, subawardY1Totals, overheadRates]);

  if (lineItems.length === 0 || indirectRows.length === 0) return null;

  const totalBase = indirectRows.reduce((s, r) => s + r.base, 0);
  const totalIndirect = indirectRows.reduce((s, r) => s + r.indirect, 0);

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 mb-2">Indirect Costs (F&amp;A) Breakdown</h3>
      <div className="bg-white rounded-lg border overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Rate</th>
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Rate %</th>
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Base</th>
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Indirect</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {indirectRows.map((row, i) => (
              <tr key={i}>
                <td className="px-4 py-2 text-sm text-gray-700">{row.rateName}</td>
                <td className="px-4 py-2 text-sm text-right text-gray-700">
                  {row.ratePercent % 1 === 0 ? `${row.ratePercent.toFixed(0)}%` : `${row.ratePercent.toFixed(1)}%`}
                </td>
                <td className="px-4 py-2 text-sm text-right">${row.base.toLocaleString()}</td>
                <td className="px-4 py-2 text-sm text-right">${row.indirect.toLocaleString()}</td>
              </tr>
            ))}
            <tr className="bg-gray-50 font-semibold">
              <td className="px-4 py-2 text-sm text-gray-800">Total</td>
              <td></td>
              <td className="px-4 py-2 text-sm text-right text-gray-800">${totalBase.toLocaleString()}</td>
              <td className="px-4 py-2 text-sm text-right text-nsf-blue">${totalIndirect.toLocaleString()}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ======== Budget Summary ======== */

function BudgetSummary({ lineItems, overheadRates, entityType, fiscalYear, subawards }: {
  lineItems: BudgetLineItem[]; overheadRates: OverheadRate[];
  entityType: string; fiscalYear: number; subawards: Subaward[];
}) {
  const isLead = entityType === 'grant';

  // For lead institution, fetch all subaward line items + overhead rates for the same fiscal year
  const { data: subawardData } = useQuery({
    queryKey: ['subaward-summary-data', fiscalYear, subawards.map(s => s.id).join(',')],
    queryFn: async () => {
      const allItems: BudgetLineItem[] = [];
      const allRates: OverheadRate[] = [];
      for (const sub of subawards) {
        const budgets = await api.institutionBudgets.list('subaward', sub.id, true);
        const budget = budgets.find(b => b.fiscal_year === fiscalYear);
        if (!budget) continue;
        const items = await api.budgetLineItems.list('subaward', sub.id, budget.id);
        allItems.push(...items);
        const rates = await api.overheadRates.list('subaward', sub.id);
        allRates.push(...rates);
      }
      return { items: allItems, rates: allRates };
    },
    enabled: isLead && subawards.length > 0,
  });

  // For lead institution, fetch subaward Y1 totals to compute MTDC indirect (same cache key as IndirectCostsTable)
  const { data: subawardY1Totals } = useQuery({
    queryKey: ['subaward-y1-totals', subawards.map(s => s.id).join(',')],
    queryFn: async () => {
      const totals: { entityId: string; total: number }[] = [];
      for (const sub of subawards) {
        const budgets = await api.institutionBudgets.list('subaward', sub.id, true);
        const y1 = budgets.find(b => b.fiscal_year === 1);
        if (!y1) { totals.push({ entityId: sub.id, total: 0 }); continue; }
        const items = await api.budgetLineItems.list('subaward', sub.id, y1.id);
        const lineItemTotal = items.reduce((s, li) => s + li.amount, 0);
        totals.push({ entityId: sub.id, total: lineItemTotal });
      }
      return totals;
    },
    enabled: isLead && subawards.length > 0,
  });

  // Compute subaward MTDC indirect charged to the lead (first $25K each subaward, Y1 only)
  const mtdcIndirect = useMemo(() => {
    if (!isLead || fiscalYear !== 1 || !subawardY1Totals) return 0;
    const MTDC_CAP = 25000;
    let subawardMTDCBase = 0;
    for (const entry of subawardY1Totals) {
      if (entry.total > 0) subawardMTDCBase += Math.min(entry.total, MTDC_CAP);
    }
    if (subawardMTDCBase === 0) return 0;
    // Check if already included in budget line items
    for (const li of lineItems) {
      const desc = (li.description ?? '').toLowerCase();
      if ((desc.includes('subaward') && desc.includes('mtdc')) || (desc.includes('subaward') && desc.includes('indirect'))) {
        return 0;
      }
    }
    // Use the overhead rate with the largest base (same logic as IndirectCostsTable)
    const bases = new Map<string, number>();
    for (const li of lineItems) {
      if (!li.overhead_rate_id) continue;
      bases.set(li.overhead_rate_id, (bases.get(li.overhead_rate_id) ?? 0) + Math.round(li.amount));
    }
    let bestRate: OverheadRate | undefined;
    let maxBase = -1;
    for (const [rateId, base] of bases) {
      if (base > maxBase) { maxBase = base; bestRate = overheadRates.find(r => r.id === rateId); }
    }
    if (!bestRate && overheadRates.length > 0) bestRate = overheadRates[0];
    return bestRate ? Math.round(subawardMTDCBase * bestRate.rate) : 0;
  }, [isLead, fiscalYear, subawardY1Totals, lineItems, overheadRates]);

  const summary = useMemo(() => {
    const roundSum = (items: BudgetLineItem[]) => items.reduce((s, li) => s + Math.round(li.amount), 0);
    const salaries = roundSum(lineItems.filter(li => li.line_type === 'personnel'));
    const fringe = roundSum(lineItems.filter(li => li.line_type === 'fringe'));
    const travel = roundSum(lineItems.filter(li => li.line_type === 'travel'));
    const equipment = roundSum(lineItems.filter(li => li.line_type === 'equipment'));
    const supplies = roundSum(lineItems.filter(li => li.line_type === 'supplies'));
    const contractual = roundSum(lineItems.filter(li => li.line_type === 'contractual'));
    const other = roundSum(lineItems.filter(li => li.line_type === 'other'));
    const tuition = roundSum(lineItems.filter(li => li.line_type === 'tuition'));
    const participantSupport = roundSum(lineItems.filter(li => li.line_type === 'participant_support'));

    const totalDirect = salaries + fringe + travel + equipment + supplies + contractual + other + tuition + participantSupport;

    const indirectCosts = lineItems.reduce((sum, li) => {
      if (!li.overhead_rate_id) return sum;
      const rate = overheadRates.find(r => r.id === li.overhead_rate_id);
      return sum + (rate ? Math.round(Math.round(li.amount) * rate.rate) : 0);
    }, 0);

    return { salaries, fringe, travel, equipment, supplies, contractual, other, tuition, participantSupport, totalDirect, indirectCosts };
  }, [lineItems, overheadRates]);

  const subSummary = useMemo(() => {
    if (!subawardData) return null;
    const { items, rates } = subawardData;
    if (items.length === 0) return null;
    const roundSum = (its: BudgetLineItem[]) => its.reduce((s, li) => s + Math.round(li.amount), 0);
    const salaries = roundSum(items.filter(li => li.line_type === 'personnel'));
    const fringe = roundSum(items.filter(li => li.line_type === 'fringe'));
    const travel = roundSum(items.filter(li => li.line_type === 'travel'));
    const equipment = roundSum(items.filter(li => li.line_type === 'equipment'));
    const supplies = roundSum(items.filter(li => li.line_type === 'supplies'));
    const contractual = roundSum(items.filter(li => li.line_type === 'contractual'));
    const other = roundSum(items.filter(li => li.line_type === 'other'));
    const tuition = roundSum(items.filter(li => li.line_type === 'tuition'));
    const participantSupport = roundSum(items.filter(li => li.line_type === 'participant_support'));
    const totalDirect = salaries + fringe + travel + equipment + supplies + contractual + other + tuition + participantSupport;

    const indirectCosts = items.reduce((sum, li) => {
      if (!li.overhead_rate_id) return sum;
      const rate = rates.find(r => r.id === li.overhead_rate_id);
      return sum + (rate ? Math.round(Math.round(li.amount) * rate.rate) : 0);
    }, 0);

    return { salaries, fringe, travel, equipment, supplies, contractual, other, tuition, participantSupport, totalDirect, indirectCosts };
  }, [subawardData]);

  if (lineItems.length === 0) return null;

  const showSub = isLead && subSummary != null;

  // Row definitions: [label, leadAmount, subAmount | null, style]
  const rows: [string, number, number | null, 'bold' | 'sub' | 'normal'][] = [
    ['A. Salaries & Wages', summary.salaries, showSub ? subSummary.salaries : null, 'normal'],
    ['B. Fringe Benefits', summary.fringe, showSub ? subSummary.fringe : null, 'normal'],
    ['Total Salaries + Fringe (A+B)', summary.salaries + summary.fringe, showSub ? subSummary.salaries + subSummary.fringe : null, 'bold'],
    ['C. Equipment', summary.equipment, showSub ? subSummary.equipment : null, 'normal'],
    ['D. Travel', summary.travel, showSub ? subSummary.travel : null, 'normal'],
    ['E. Participant Support', summary.participantSupport, showSub ? subSummary.participantSupport : null, 'normal'],
    ['F. Other Direct Costs', summary.supplies + summary.contractual + summary.other + summary.tuition,
      showSub ? subSummary.supplies + subSummary.contractual + subSummary.other + subSummary.tuition : null, 'normal'],
    ['Supplies', summary.supplies, showSub ? subSummary.supplies : null, 'sub'],
    ['Contractual', summary.contractual, showSub ? subSummary.contractual : null, 'sub'],
    ['Tuition', summary.tuition, showSub ? subSummary.tuition : null, 'sub'],
    ['Other', summary.other, showSub ? subSummary.other : null, 'sub'],
    ['G. Total Direct Costs', summary.totalDirect, showSub ? subSummary.totalDirect : null, 'bold'],
    ['H. Indirect Costs (F&A)', summary.indirectCosts + mtdcIndirect, showSub ? subSummary.indirectCosts : null, 'normal'],
    ['I. Total Costs (G+H)', summary.totalDirect + summary.indirectCosts + mtdcIndirect,
      showSub ? subSummary.totalDirect + subSummary.indirectCosts : null, 'bold'],
  ];

  const fmtDollar = (v: number) => `$${v.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}`;

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 mb-2">Budget Summary</h3>
      <div className="bg-white rounded-lg border overflow-hidden">
        <table className="w-full">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Category</th>
              <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Lead</th>
              {showSub && <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Subawards</th>}
              {showSub && <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Project Total</th>}
            </tr>
          </thead>
          <tbody className="divide-y">
            {rows.map(([label, amount, subAmount, style], i) => (
              <tr key={i} className={style === 'bold' ? 'bg-gray-50' : ''}>
                <td className={`px-4 py-2 text-sm ${
                  style === 'bold' ? 'font-semibold text-gray-800' :
                  style === 'sub' ? 'text-gray-500 pl-10' :
                  'text-gray-700'
                }`}>
                  {label}
                </td>
                <td className={`px-4 py-2 text-sm text-right ${style === 'bold' ? 'font-semibold text-nsf-blue' : ''}`}>
                  {fmtDollar(amount)}
                </td>
                {showSub && (
                  <td className={`px-4 py-2 text-sm text-right ${style === 'bold' ? 'font-semibold text-nsf-blue' : ''}`}>
                    {subAmount != null && subAmount !== 0 ? fmtDollar(subAmount) : ''}
                  </td>
                )}
                {showSub && (
                  <td className={`px-4 py-2 text-sm text-right ${style === 'bold' ? 'font-semibold text-nsf-blue' : ''}`}>
                    {subAmount != null ? fmtDollar(amount + subAmount) : fmtDollar(amount)}
                  </td>
                )}
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

/* ======== WBS Allocation Editor (wrapper around shared component) ======== */

function WBSAllocationEditor({
  lineItemId, entityType, entityId, budgetId, wbsAreas, allocations, readOnly = false, salaryLineItemId, onSaved,
}: {
  lineItemId: string; entityType: string; entityId: string; budgetId: string;
  wbsAreas: WBSArea[]; allocations: BudgetLineItemWBS[]; readOnly?: boolean;
  salaryLineItemId?: string; onSaved?: () => void;
}) {
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);

  const allocMap = useMemo(() => {
    const m: Record<string, number> = {};
    for (const a of allocations) m[a.wbs_area_id] = a.allocation_percent;
    return m;
  }, [allocations]);

  const handleSave = useCallback(async (allocs: Record<string, number>) => {
    setSaving(true);
    try {
      const arr = Object.entries(allocs).map(([wbs_area_id, allocation_percent]) => ({ wbs_area_id, allocation_percent }));
      await api.budgetLineItems.setWBS(entityType, entityId, budgetId, lineItemId, arr);
      queryClient.invalidateQueries({ queryKey: ['line-item-wbs', lineItemId] });
      onSaved?.();
    } finally {
      setSaving(false);
    }
  }, [entityType, entityId, budgetId, lineItemId, queryClient, onSaved]);

  const handleCopyFromSalary = useMemo(() => {
    if (!salaryLineItemId) return undefined;
    return async () => {
      const salaryWbs = await api.budgetLineItems.listWBS(entityType, entityId, budgetId, salaryLineItemId);
      const m: Record<string, number> = {};
      for (const w of salaryWbs) m[w.wbs_area_id] = w.allocation_percent;
      return m;
    };
  }, [salaryLineItemId, entityType, entityId, budgetId]);

  return (
    <WBSAllocEditor
      wbsAreas={wbsAreas}
      allocations={allocMap}
      onSave={handleSave}
      saving={saving}
      readOnly={readOnly}
      onCopyFrom={handleCopyFromSalary}
      copyLabel="Copy from Salary"
    />
  );
}

/* ======== Line Item Form (with Personnel auto-fringe) ======== */

function LineItemForm({
  personnel, overheadRates, yearFringeRates, fiscalYear, salaryEscalationRate,
  currentInstitution, onSubmit, onAddPersonnel, isLoading,
}: {
  personnel: Personnel[]; overheadRates: OverheadRate[];
  currentInstitution: string;
  yearFringeRates: InstitutionFringeRate[];
  fiscalYear: number; salaryEscalationRate: number;
  onSubmit: (data: Partial<BudgetLineItem>) => void;
  onAddPersonnel: (personId: string, effortMonths: number, selectedFringeRateIds: string[], overheadRateId: string | null) => void;
  isLoading: boolean;
}) {
  const [mode, setMode] = useState<'personnel' | 'other'>('personnel');
  const [personId, setPersonId] = useState('');
  const [effortMonths, setEffortMonths] = useState(0);
  const [selectedFringeIds, setSelectedFringeIds] = useState<string[]>(yearFringeRates.map(fr => fr.id));
  const [personnelOverheadRateId, setPersonnelOverheadRateId] = useState('');
  const [form, setForm] = useState({
    line_type: 'travel', description: '', amount: 0, overhead_rate_id: '', notes: '',
  });

  const selectedPerson = personnel.find((p) => p.id === personId);
  const escalatedSalary = selectedPerson ? selectedPerson.annual_salary * Math.pow(1 + salaryEscalationRate, fiscalYear - 1) : 0;
  const computedSalary = selectedPerson ? Math.round((escalatedSalary / 12) * effortMonths) : 0;

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
                {(() => {
                  const sorted = [...personnel].sort((a, b) => a.name.localeCompare(b.name));
                  const local = sorted.filter(p => p.institution === currentInstitution);
                  const external = sorted.filter(p => p.institution !== currentInstitution);
                  return (
                    <>
                      {local.map((p) => (
                        <option key={p.id} value={p.id}>{p.name} ({p.role}) — ${p.annual_salary.toLocaleString()}/yr</option>
                      ))}
                      {local.length > 0 && external.length > 0 && (
                        <option disabled>────────────────</option>
                      )}
                      {external.map((p) => (
                        <option key={p.id} value={p.id}>{p.name} ({p.role}) [{p.institution}] — ${p.annual_salary.toLocaleString()}/yr</option>
                      ))}
                    </>
                  );
                })()}
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
          {overheadRates.length > 0 && (
            <div className="bg-amber-50 rounded p-3">
              <label className="block text-xs font-medium text-amber-700 mb-1">Overhead / Indirect Rate</label>
              <select value={personnelOverheadRateId} onChange={(e) => setPersonnelOverheadRateId(e.target.value)}
                className="w-full border rounded-md px-3 py-2 text-sm">
                <option value="">— None (no overhead) —</option>
                {overheadRates.map((r) => (
                  <option key={r.id} value={r.id}>{r.rate_name} ({(r.rate * 100).toFixed(1)}%)</option>
                ))}
              </select>
            </div>
          )}
          {yearFringeRates.length > 0 && (
            <div className="bg-blue-50 rounded p-3">
              <p className="text-xs font-medium text-blue-700 mb-1">Fringe rates:</p>
              {yearFringeRates.map((fr) => {
                const checked = selectedFringeIds.includes(fr.id);
                const fringeAmt = checked ? Math.round((escalatedSalary / 12) * effortMonths * fr.rate) : 0;
                return (
                  <label key={fr.id} className="flex items-center gap-2 text-xs text-blue-600 py-0.5 cursor-pointer">
                    <input type="checkbox" checked={checked}
                      onChange={(e) => setSelectedFringeIds(
                        e.target.checked ? [...selectedFringeIds, fr.id] : selectedFringeIds.filter(id => id !== fr.id)
                      )} />
                    {fr.rate_name}: {(fr.rate * 100).toFixed(2)}%{checked && effortMonths > 0 ? ` = $${fringeAmt.toLocaleString()}` : ''}
                  </label>
                );
              })}
            </div>
          )}
          {yearFringeRates.length === 0 && (
            <p className="text-xs text-amber-600">
              No fringe rates defined for this year/institution. Add them on the Institutions page first.
            </p>
          )}
          <button onClick={() => { if (personId && effortMonths > 0) onAddPersonnel(personId, effortMonths, selectedFringeIds, personnelOverheadRateId || null); }}
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
