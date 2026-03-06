'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  Grant,
  Subaward,
  Invoice,
  StatementOfWork,
  Personnel,
  InstitutionFringeRate,
  InstitutionBudget,
  OverheadRate,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useState, useMemo, useCallback } from 'react';
import Link from 'next/link';
import CurrencyInput from '@/components/CurrencyInput';

const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

export default function InstitutionsPage() {
  const { grant, grantId, isLoading: grantLoading } = useGrant();
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [leadExpanded, setLeadExpanded] = useState(false);

  const { data: subawards, isLoading } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  const { data: personnel } = useQuery({
    queryKey: ['personnel', grantId],
    queryFn: () => api.personnel.list(grantId!),
    enabled: !!grantId,
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<Subaward>) => api.subawards.create(grantId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['subawards', grantId] });
      setShowForm(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.subawards.delete(grantId!, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['subawards', grantId] }),
  });

  // Group personnel by institution
  const personnelByInstitution = useMemo(() => {
    const map: Record<string, Personnel[]> = {};
    for (const p of personnel ?? []) {
      const inst = p.institution || 'Unassigned';
      if (!map[inst]) map[inst] = [];
      map[inst].push(p);
    }
    return map;
  }, [personnel]);

  if (grantLoading || isLoading) return <div className="p-4">Loading institutions...</div>;
  if (!grantId || !grant) {
    return (
      <div className="p-4">
        No project configured.{' '}
        <Link href="/settings" className="text-nsf-light hover:underline">Set up project</Link>
      </div>
    );
  }

  const totalSubawards = (subawards ?? []).reduce((sum, s) => sum + s.total_amount, 0);

  return (
    <div className="max-w-6xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-nsf-blue">Institutions</h1>
          <p className="text-sm text-gray-500">
            Lead institution + {subawards?.length ?? 0} subaward institution{(subawards?.length ?? 0) !== 1 ? 's' : ''} · Subaward total: ${totalSubawards.toLocaleString()}
          </p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors"
        >
          {showForm ? 'Cancel' : '+ Add Subaward Institution'}
        </button>
      </div>

      {showForm && (
        <SubawardForm
          grant={grant}
          onSubmit={(data) => createMutation.mutate(data)}
          isLoading={createMutation.isPending}
        />
      )}

      {/* Lead Institution */}
      <LeadInstitutionCard
        grant={grant}
        expanded={leadExpanded}
        onToggle={() => setLeadExpanded(!leadExpanded)}
        personnel={personnelByInstitution[grant.institution] ?? []}
      />

      {/* Subaward Institutions */}
      <div className="space-y-4">
        {subawards?.map((sub) => (
          <SubawardCard
            key={sub.id}
            subaward={sub}
            grantId={grantId}
            expanded={expandedId === sub.id}
            personnel={personnelByInstitution[sub.institution] ?? []}
            onToggle={() => setExpandedId(expandedId === sub.id ? null : sub.id)}
            onDelete={() => {
              if (confirm(`Delete subaward institution ${sub.institution}?`)) deleteMutation.mutate(sub.id);
            }}
          />
        ))}
        {(!subawards || subawards.length === 0) && (
          <div className="bg-white rounded-lg border p-8 text-center text-gray-400">
            No subaward institutions yet. Click &quot;+ Add Subaward Institution&quot; to add one.
          </div>
        )}
      </div>
    </div>
  );
}

/* ─── Status Badge ─── */

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: 'bg-green-100 text-green-800',
    pending: 'bg-yellow-100 text-yellow-800',
    completed: 'bg-blue-100 text-blue-800',
    closed: 'bg-gray-100 text-gray-800',
    draft: 'bg-gray-100 text-gray-800',
    approved: 'bg-green-100 text-green-800',
    paid: 'bg-blue-100 text-blue-800',
    rejected: 'bg-red-100 text-red-800',
    signed: 'bg-green-100 text-green-800',
  };
  return (
    <span className={`inline-block px-2 py-0.5 rounded-full text-xs font-medium ${colors[status] || 'bg-gray-100 text-gray-800'}`}>
      {status}
    </span>
  );
}

/* ─── Lead Institution Card ─── */

function LeadInstitutionCard({
  grant,
  expanded,
  onToggle,
  personnel,
}: {
  grant: Grant;
  expanded: boolean;
  onToggle: () => void;
  personnel: Personnel[];
}) {
  const [tab, setTab] = useState<'personnel' | 'rates' | 'budgets'>('personnel');

  return (
    <div className="bg-white rounded-lg border overflow-hidden border-nsf-blue/30">
      <div
        className="px-6 py-4 flex items-center justify-between cursor-pointer hover:bg-gray-50"
        onClick={onToggle}
      >
        <div>
          <div className="flex items-center gap-3">
            <h3 className="font-semibold text-nsf-blue">{grant.institution || '(No lead institution set)'}</h3>
            <span className="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-nsf-blue/10 text-nsf-blue">
              Lead
            </span>
          </div>
          <p className="text-sm text-gray-500">
            PI: {grant.pi_name}
            {personnel.length > 0 ? ` · ${personnel.length} personnel` : ''}
          </p>
        </div>
        <span className="text-gray-400">{expanded ? '▲' : '▼'}</span>
      </div>

      {expanded && (
        <div className="border-t">
          <div className="flex border-b bg-gray-50">
            {(['personnel', 'rates', 'budgets'] as const).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  tab === t ? 'text-nsf-blue border-b-2 border-nsf-blue bg-white' : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                {t === 'personnel' ? 'Personnel' : t === 'rates' ? 'Overhead & Fringe' : 'Budgets (Versioned)'}
              </button>
            ))}
          </div>
          <div className="p-4">
            {tab === 'personnel' && (
              <PersonnelPanel personnel={personnel} institution={grant.institution} />
            )}
            {tab === 'rates' && (
              <RatesPanel
                entityType="grant"
                entityId={grant.id}
              />
            )}
            {tab === 'budgets' && (
              <InstitutionBudgetsPanel entityType="grant" entityId={grant.id} />
            )}
          </div>
          {!grant.institution && (
            <div className="px-6 pb-4">
              <p className="text-sm text-amber-600">
                No lead institution set.{' '}
                <Link href="/settings" className="text-nsf-light hover:underline">
                  Configure it in Settings
                </Link>
              </p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/* ─── Subaward Institution Card ─── */

function SubawardCard({
  subaward,
  grantId,
  expanded,
  personnel,
  onToggle,
  onDelete,
}: {
  subaward: Subaward;
  grantId: string;
  expanded: boolean;
  personnel: Personnel[];
  onToggle: () => void;
  onDelete: () => void;
}) {
  const [tab, setTab] = useState<'personnel' | 'rates' | 'budgets' | 'invoices' | 'sow'>('personnel');

  return (
    <div className="bg-white rounded-lg border overflow-hidden">
      <div
        className="px-6 py-4 flex items-center justify-between cursor-pointer hover:bg-gray-50"
        onClick={onToggle}
      >
        <div>
          <div className="flex items-center gap-3">
            <h3 className="font-semibold text-nsf-blue">{subaward.institution}</h3>
            <StatusBadge status={subaward.status} />
            <span className="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-orange-100 text-orange-800">
              Subaward
            </span>
          </div>
          <p className="text-sm text-gray-500">
            Sub-PI: {subaward.pi_name} · ${subaward.total_amount.toLocaleString()}
            {' · '}{subaward.start_date} to {subaward.end_date}
            {personnel.length > 0 ? ` · ${personnel.length} personnel` : ''}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            className="text-red-500 hover:underline text-xs"
          >
            Delete
          </button>
          <span className="text-gray-400">{expanded ? '▲' : '▼'}</span>
        </div>
      </div>

      {expanded && (
        <div className="border-t">
          <div className="flex border-b bg-gray-50 flex-wrap">
            {(['personnel', 'rates', 'budgets', 'invoices', 'sow'] as const).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={`px-4 py-2 text-sm font-medium transition-colors ${
                  tab === t ? 'text-nsf-blue border-b-2 border-nsf-blue bg-white' : 'text-gray-500 hover:text-gray-700'
                }`}
              >
                {t === 'personnel' ? 'Personnel'
                  : t === 'rates' ? 'Overhead & Fringe'
                  : t === 'budgets' ? 'Budgets (Versioned)'
                  : t === 'invoices' ? 'Invoices'
                  : 'Statements of Work'}
              </button>
            ))}
          </div>
          <div className="p-4">
            {tab === 'personnel' && (
              <PersonnelPanel personnel={personnel} institution={subaward.institution} />
            )}
            {tab === 'rates' && (
              <RatesPanel
                entityType="subaward"
                entityId={subaward.id}
              />
            )}
            {tab === 'budgets' && (
              <InstitutionBudgetsPanel entityType="subaward" entityId={subaward.id} />
            )}
            {tab === 'invoices' && <InvoicePanel grantId={grantId} subawardId={subaward.id} />}
            {tab === 'sow' && <SOWPanel grantId={grantId} subawardId={subaward.id} />}
          </div>
        </div>
      )}
    </div>
  );
}

/* ─── Personnel Panel (read-only, links to personnel page) ─── */

function PersonnelPanel({ personnel, institution }: { personnel: Personnel[]; institution: string }) {
  if (personnel.length === 0) {
    return (
      <p className="text-sm text-gray-400">
        No personnel assigned to {institution}.{' '}
        <Link href="/personnel" className="text-nsf-light hover:underline">Manage personnel</Link>
      </p>
    );
  }

  return (
    <div>
      <table className="w-full text-sm">
        <thead className="bg-gray-50 border-b">
          <tr>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Name</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Role</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Title</th>
            <th className="text-right px-3 py-2 font-medium text-gray-600">Salary</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {personnel.map((p) => (
            <tr key={p.id}>
              <td className="px-3 py-2 font-medium">{p.name}</td>
              <td className="px-3 py-2">{p.role}</td>
              <td className="px-3 py-2">{p.title || '—'}</td>
              <td className="px-3 py-2 text-right">${p.annual_salary.toLocaleString()}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-xs text-gray-400 mt-2">
        <Link href="/personnel" className="text-nsf-light hover:underline">Edit personnel</Link>
      </p>
    </div>
  );
}

/* ─── Rates Panel: Overhead + Fringe Rates ─── */

function RatesPanel({
  entityType,
  entityId,
}: {
  entityType: string;
  entityId: string;
}) {
  const queryClient = useQueryClient();

  // Overhead rates
  const { data: overheadRates, isLoading: overheadLoading } = useQuery({
    queryKey: ['overhead-rates', entityType, entityId],
    queryFn: () => api.overheadRates.list(entityType, entityId),
  });

  const createOverhead = useMutation({
    mutationFn: (data: Partial<OverheadRate>) =>
      api.overheadRates.create(entityType, entityId, data),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['overhead-rates', entityType, entityId] }),
  });

  const deleteOverhead = useMutation({
    mutationFn: (id: string) => api.overheadRates.delete(entityType, entityId, id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['overhead-rates', entityType, entityId] }),
  });

  const [newOverhead, setNewOverhead] = useState({ rate_name: '', rate: 0, description: '' });
  const [showOverheadForm, setShowOverheadForm] = useState(false);

  // Fringe rates
  const { data: fringeRates, isLoading } = useQuery({
    queryKey: ['fringe-rates', entityType, entityId],
    queryFn: () => api.fringeRates.list(entityType, entityId),
  });

  const upsertFringe = useMutation({
    mutationFn: (data: Partial<InstitutionFringeRate>) =>
      api.fringeRates.upsert(entityType, entityId, data),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['fringe-rates', entityType, entityId] }),
  });

  const deleteFringe = useMutation({
    mutationFn: (id: string) => api.fringeRates.delete(entityType, entityId, id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['fringe-rates', entityType, entityId] }),
  });

  // Group fringe rates by year for display
  const fringeByYear = useMemo(() => {
    const map: Record<number, InstitutionFringeRate[]> = {};
    for (const fr of fringeRates ?? []) {
      if (!map[fr.fiscal_year]) map[fr.fiscal_year] = [];
      map[fr.fiscal_year].push(fr);
    }
    return map;
  }, [fringeRates]);

  const [addingYear, setAddingYear] = useState<number | null>(null);
  const [newRate, setNewRate] = useState({ rate_name: '', rate: 0 });

  // Find the nearest prior year with fringe rates defined
  const findPriorYearRates = useCallback((year: number): InstitutionFringeRate[] => {
    for (let y = year - 1; y >= 1; y--) {
      const rates = fringeByYear[y];
      if (rates && rates.length > 0) return rates;
    }
    return [];
  }, [fringeByYear]);

  // Auto-copy all fringe rates from the nearest prior year
  const copyFromPriorYear = useCallback(async (year: number) => {
    const priorRates = findPriorYearRates(year);
    for (const pr of priorRates) {
      await api.fringeRates.upsert(entityType, entityId, {
        fiscal_year: year, rate_name: pr.rate_name, rate: pr.rate,
      });
    }
    queryClient.invalidateQueries({ queryKey: ['fringe-rates', entityType, entityId] });
  }, [findPriorYearRates, entityType, entityId, queryClient]);

  // When opening the add form, pre-fill from the first prior-year rate not yet in this year
  const openAddRate = useCallback((year: number) => {
    if (addingYear === year) { setAddingYear(null); return; }
    const currentRates = fringeByYear[year] ?? [];
    const priorRates = findPriorYearRates(year);
    const currentNames = new Set(currentRates.map(r => r.rate_name));
    const missing = priorRates.find(pr => !currentNames.has(pr.rate_name));
    setNewRate(missing ? { rate_name: missing.rate_name, rate: missing.rate } : { rate_name: '', rate: 0 });
    setAddingYear(year);
  }, [addingYear, fringeByYear, findPriorYearRates]);

  return (
    <div className="space-y-6">
      {/* Overhead (F&A) Rates — editable */}
      <div>
        <div className="flex items-center justify-between mb-2">
          <h4 className="font-medium text-sm text-gray-700">Overhead (F&amp;A) Rates</h4>
          <button
            onClick={() => setShowOverheadForm(!showOverheadForm)}
            className="text-xs text-nsf-light hover:underline"
          >
            {showOverheadForm ? 'Cancel' : '+ Add Rate'}
          </button>
        </div>
        {overheadLoading ? (
          <p className="text-sm text-gray-400">Loading overhead rates...</p>
        ) : (overheadRates ?? []).length === 0 && !showOverheadForm ? (
          <p className="text-xs text-gray-400">No overhead rates defined. Add rates like &ldquo;MTDC On-Campus&rdquo;, &ldquo;MTDC Off-Campus&rdquo;, etc.</p>
        ) : (
          <div className="space-y-1 mb-2">
            {(overheadRates ?? []).map((r) => (
              <div key={r.id} className="flex items-center justify-between bg-gray-50 rounded border p-2 text-sm">
                <div>
                  <span className="font-medium">{r.rate_name}</span>
                  {r.description && <span className="text-gray-400 ml-2">— {r.description}</span>}
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-nsf-blue">{(r.rate * 100).toFixed(2)}%</span>
                  <button
                    onClick={() => { if (confirm(`Delete "${r.rate_name}"?`)) deleteOverhead.mutate(r.id); }}
                    className="text-red-400 hover:text-red-600 text-xs"
                  >
                    ✕
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
        {showOverheadForm && (
          <div className="flex items-end gap-2 bg-gray-50 p-2 rounded border">
            <div className="flex-1">
              <label className="block text-xs text-gray-500 mb-0.5">Rate Name</label>
              <input type="text" value={newOverhead.rate_name}
                onChange={(e) => setNewOverhead({ ...newOverhead, rate_name: e.target.value })}
                placeholder="e.g. MTDC On-Campus" className="w-full border rounded px-2 py-1 text-sm" />
            </div>
            <div className="w-24">
              <label className="block text-xs text-gray-500 mb-0.5">Rate (%)</label>
              <input type="number" step="0.01"
                value={newOverhead.rate === 0 ? '' : (newOverhead.rate * 100)}
                onChange={(e) => setNewOverhead({ ...newOverhead, rate: (parseFloat(e.target.value) || 0) / 100 })}
                placeholder="55.5" className="w-full border rounded px-2 py-1 text-sm" />
            </div>
            <div className="flex-1">
              <label className="block text-xs text-gray-500 mb-0.5">Description</label>
              <input type="text" value={newOverhead.description}
                onChange={(e) => setNewOverhead({ ...newOverhead, description: e.target.value })}
                placeholder="optional" className="w-full border rounded px-2 py-1 text-sm" />
            </div>
            <button
              onClick={() => {
                if (newOverhead.rate_name) {
                  createOverhead.mutate(newOverhead);
                  setNewOverhead({ rate_name: '', rate: 0, description: '' });
                  setShowOverheadForm(false);
                }
              }}
              disabled={createOverhead.isPending}
              className="px-2 py-1 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50"
            >
              Save
            </button>
          </div>
        )}
      </div>

      {/* Fringe Rates by Year */}
      <div>
        <h4 className="font-medium text-sm text-gray-700 mb-2">Fringe Benefit Rates (by Year)</h4>
        {isLoading ? (
          <p className="text-sm text-gray-400">Loading fringe rates...</p>
        ) : (
          <div className="space-y-3">
            {PROJECT_YEARS.map((year) => {
              const rates = fringeByYear[year] ?? [];
              return (
                <div key={year} className="bg-gray-50 rounded border p-3">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-nsf-blue">{YEAR_LABELS[year]}</span>
                    <div className="flex items-center gap-2">
                      {rates.length === 0 && findPriorYearRates(year).length > 0 && addingYear !== year && (
                        <button
                          onClick={() => copyFromPriorYear(year)}
                          className="text-xs text-green-600 hover:underline"
                        >
                          Copy from Prior Year
                        </button>
                      )}
                      <button
                        onClick={() => openAddRate(year)}
                        className="text-xs text-nsf-light hover:underline"
                      >
                        {addingYear === year ? 'Cancel' : '+ Add Rate'}
                      </button>
                    </div>
                  </div>
                  {rates.length === 0 && addingYear !== year && (
                    <p className="text-xs text-gray-400">No fringe rates defined for this year.</p>
                  )}
                  {rates.length > 0 && (
                    <div className="space-y-1">
                      {rates.map((fr) => (
                        <div key={fr.id} className="flex items-center justify-between text-sm">
                          <span className="text-gray-700">{fr.rate_name}</span>
                          <div className="flex items-center gap-2">
                            <span className="font-medium">{(fr.rate * 100).toFixed(2)}%</span>
                            <button
                              onClick={() => {
                                if (confirm(`Delete "${fr.rate_name}" rate?`)) deleteFringe.mutate(fr.id);
                              }}
                              className="text-red-400 hover:text-red-600 text-xs"
                            >
                              ✕
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                  {addingYear === year && (
                    <div className="mt-2 flex items-end gap-2">
                      <div className="flex-1">
                        <label className="block text-xs text-gray-500 mb-0.5">Rate Name</label>
                        <input
                          type="text"
                          value={newRate.rate_name}
                          onChange={(e) => setNewRate({ ...newRate, rate_name: e.target.value })}
                          placeholder="e.g. Benefits, Student Health"
                          className="w-full border rounded px-2 py-1 text-sm"
                        />
                      </div>
                      <div className="w-28">
                        <label className="block text-xs text-gray-500 mb-0.5">Rate (%)</label>
                        <input
                          type="number"
                          step="0.01"
                          value={newRate.rate === 0 ? '' : (newRate.rate * 100)}
                          onChange={(e) => setNewRate({ ...newRate, rate: (parseFloat(e.target.value) || 0) / 100 })}
                          placeholder="e.g. 32.5"
                          className="w-full border rounded px-2 py-1 text-sm"
                        />
                      </div>
                      <button
                        onClick={() => {
                          if (newRate.rate_name) {
                            upsertFringe.mutate({ fiscal_year: year, rate_name: newRate.rate_name, rate: newRate.rate });
                            setAddingYear(null);
                            setNewRate({ rate_name: '', rate: 0 });
                          }
                        }}
                        disabled={upsertFringe.isPending}
                        className="px-2 py-1 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50"
                      >
                        Save
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

/* ─── Institution Budgets Panel (Versioned) ─── */

function InstitutionBudgetsPanel({
  entityType,
  entityId,
}: {
  entityType: string;
  entityId: string;
}) {
  const queryClient = useQueryClient();
  const [showAll, setShowAll] = useState(false);
  const [showForm, setShowForm] = useState(false);

  const { data: budgets, isLoading } = useQuery({
    queryKey: ['institution-budgets', entityType, entityId, showAll],
    queryFn: () => api.institutionBudgets.list(entityType, entityId, !showAll),
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<InstitutionBudget>) =>
      api.institutionBudgets.create(entityType, entityId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['institution-budgets', entityType, entityId] });
      setShowForm(false);
    },
  });

  // Group by year and sort latest version first
  const budgetsByYear = useMemo(() => {
    const map: Record<number, InstitutionBudget[]> = {};
    for (const b of budgets ?? []) {
      if (!map[b.fiscal_year]) map[b.fiscal_year] = [];
      map[b.fiscal_year].push(b);
    }
    // Sort each year's budgets by version descending
    for (const year of Object.keys(map)) {
      map[parseInt(year)].sort((a, b) => b.version - a.version);
    }
    return map;
  }, [budgets]);

  const latestTotal = PROJECT_YEARS.reduce((sum, y) => {
    const latest = budgetsByYear[y]?.find((b) => b.is_latest);
    return sum + (latest?.budget ?? 0);
  }, 0);

  if (isLoading) return <p className="text-sm text-gray-400">Loading budgets...</p>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-500">
          Latest budget total: ${latestTotal.toLocaleString()}
        </p>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-1 text-xs text-gray-500 cursor-pointer">
            <input
              type="checkbox"
              checked={showAll}
              onChange={(e) => setShowAll(e.target.checked)}
              className="rounded"
            />
            Show all versions
          </label>
          <button
            onClick={() => setShowForm(!showForm)}
            className="text-sm px-3 py-1 bg-nsf-light text-white rounded hover:bg-nsf-blue"
          >
            {showForm ? 'Cancel' : '+ New Budget Version'}
          </button>
        </div>
      </div>

      {showForm && (
        <NewBudgetVersionForm
          onSubmit={(data) => createMutation.mutate(data)}
          isLoading={createMutation.isPending}
        />
      )}

      <div className="grid grid-cols-5 gap-3">
        {PROJECT_YEARS.map((year) => {
          const yearBudgets = budgetsByYear[year] ?? [];
          const latest = yearBudgets.find((b) => b.is_latest);
          return (
            <div key={year} className="bg-gray-50 rounded border p-3 text-xs">
              <div className="font-medium text-nsf-blue mb-1">{YEAR_LABELS[year]}</div>
              {latest ? (
                <div>
                  <div className="text-lg font-semibold">${latest.budget.toLocaleString()}</div>
                  <div className="text-gray-400">v{latest.version} (latest)</div>
                  {latest.notes && <div className="text-gray-500 mt-0.5 truncate">{latest.notes}</div>}
                </div>
              ) : (
                <div className="text-gray-400">No budget</div>
              )}
              {showAll && yearBudgets.filter((b) => !b.is_latest).length > 0 && (
                <div className="mt-2 border-t pt-1 space-y-1">
                  {yearBudgets
                    .filter((b) => !b.is_latest)
                    .map((b) => (
                      <div key={b.id} className="text-gray-400">
                        v{b.version}: ${b.budget.toLocaleString()}
                        {b.notes && <span className="ml-1">— {b.notes}</span>}
                      </div>
                    ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function NewBudgetVersionForm({
  onSubmit,
  isLoading,
}: {
  onSubmit: (data: Partial<InstitutionBudget>) => void;
  isLoading: boolean;
}) {
  const [form, setForm] = useState({
    fiscal_year: 1,
    budget: 0,
    notes: '',
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); onSubmit(form); }}
      className="bg-gray-50 p-4 rounded border space-y-3"
    >
      <div className="grid grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Fiscal Year</label>
          <select
            value={form.fiscal_year}
            onChange={(e) => setForm({ ...form, fiscal_year: parseInt(e.target.value) })}
            className="w-full border rounded px-2 py-1.5 text-sm"
          >
            {PROJECT_YEARS.map((y) => (
              <option key={y} value={y}>{YEAR_LABELS[y]}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Budget ($)</label>
          <CurrencyInput
            value={form.budget}
            required
            onChange={(val) => setForm({ ...form, budget: val })}
            className="w-full border rounded px-2 py-1.5 text-sm"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Notes (optional)</label>
          <input
            type="text"
            value={form.notes}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm"
            placeholder="e.g. revised per amendment 2"
          />
        </div>
      </div>
      <button
        type="submit"
        disabled={isLoading}
        className="px-3 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50"
      >
        {isLoading ? 'Creating...' : 'Create Budget Version'}
      </button>
    </form>
  );
}

/* ─── Invoice Panel ─── */

function InvoicePanel({ grantId, subawardId }: { grantId: string; subawardId: string }) {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);

  const { data: invoices, isLoading } = useQuery({
    queryKey: ['invoices', grantId, subawardId],
    queryFn: () => api.invoices.list(grantId, subawardId),
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<Invoice>) => api.invoices.create(grantId, subawardId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['invoices', grantId, subawardId] });
      setShowForm(false);
    },
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({ invoiceId, status }: { invoiceId: string; status: string }) =>
      api.invoices.updateStatus(grantId, subawardId, invoiceId, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['invoices', grantId, subawardId] }),
  });

  if (isLoading) return <div className="text-sm text-gray-500">Loading invoices...</div>;

  const totalInvoiced = (invoices ?? []).reduce((sum, inv) => sum + inv.amount, 0);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-500">
          {invoices?.length ?? 0} invoices · Total: ${totalInvoiced.toLocaleString()}
        </p>
        <button
          onClick={() => setShowForm(!showForm)}
          className="text-sm px-3 py-1 bg-nsf-light text-white rounded hover:bg-nsf-blue"
        >
          {showForm ? 'Cancel' : '+ Add Invoice'}
        </button>
      </div>

      {showForm && (
        <InvoiceForm
          onSubmit={(data) => createMutation.mutate(data)}
          isLoading={createMutation.isPending}
        />
      )}

      <table className="w-full text-sm">
        <thead className="bg-gray-50 border-b">
          <tr>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Invoice #</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Date</th>
            <th className="text-right px-3 py-2 font-medium text-gray-600">Amount</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Period</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Status</th>
            <th className="text-center px-3 py-2 font-medium text-gray-600">Actions</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {invoices?.map((inv) => (
            <tr key={inv.id}>
              <td className="px-3 py-2">{inv.invoice_number || '—'}</td>
              <td className="px-3 py-2">{inv.invoice_date}</td>
              <td className="px-3 py-2 text-right">${inv.amount.toLocaleString()}</td>
              <td className="px-3 py-2">
                {inv.period_start && inv.period_end ? `${inv.period_start} — ${inv.period_end}` : '—'}
              </td>
              <td className="px-3 py-2"><StatusBadge status={inv.status} /></td>
              <td className="px-3 py-2 text-center">
                <select
                  value={inv.status}
                  onChange={(e) => updateStatusMutation.mutate({ invoiceId: inv.id, status: e.target.value })}
                  className="text-xs border rounded px-1 py-0.5"
                >
                  <option value="pending">pending</option>
                  <option value="approved">approved</option>
                  <option value="paid">paid</option>
                  <option value="rejected">rejected</option>
                </select>
              </td>
            </tr>
          ))}
          {(!invoices || invoices.length === 0) && (
            <tr><td colSpan={6} className="px-3 py-4 text-center text-gray-400">No invoices yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

/* ─── Invoice Form ─── */

function InvoiceForm({
  onSubmit,
  isLoading,
}: {
  onSubmit: (data: Partial<Invoice>) => void;
  isLoading: boolean;
}) {
  const [form, setForm] = useState({
    invoice_number: '',
    invoice_date: '',
    amount: 0,
    period_start: '',
    period_end: '',
    notes: '',
    status: 'pending',
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); onSubmit(form); }}
      className="bg-gray-50 p-4 rounded border space-y-3"
    >
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Invoice #</label>
          <input type="text" value={form.invoice_number}
            onChange={(e) => setForm({ ...form, invoice_number: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Date</label>
          <input type="date" required value={form.invoice_date}
            onChange={(e) => setForm({ ...form, invoice_date: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Amount ($)</label>
          <CurrencyInput value={form.amount} required
            onChange={(val) => setForm({ ...form, amount: val })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Period Start</label>
          <input type="date" value={form.period_start}
            onChange={(e) => setForm({ ...form, period_start: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Period End</label>
          <input type="date" value={form.period_end}
            onChange={(e) => setForm({ ...form, period_end: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Notes</label>
          <input type="text" value={form.notes}
            onChange={(e) => setForm({ ...form, notes: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
      </div>
      <button type="submit" disabled={isLoading}
        className="px-3 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50">
        {isLoading ? 'Adding...' : 'Add Invoice'}
      </button>
    </form>
  );
}

/* ─── SOW Panel ─── */

function SOWPanel({ grantId, subawardId }: { grantId: string; subawardId: string }) {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);

  const { data: sows, isLoading } = useQuery({
    queryKey: ['sow', grantId, subawardId],
    queryFn: () => api.sow.list(grantId, subawardId),
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<StatementOfWork>) => api.sow.create(grantId, subawardId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sow', grantId, subawardId] });
      setShowForm(false);
    },
  });

  if (isLoading) return <div className="text-sm text-gray-500">Loading...</div>;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <p className="text-sm text-gray-500">{sows?.length ?? 0} statements of work</p>
        <button onClick={() => setShowForm(!showForm)}
          className="text-sm px-3 py-1 bg-nsf-light text-white rounded hover:bg-nsf-blue">
          {showForm ? 'Cancel' : '+ Add SOW'}
        </button>
      </div>

      {showForm && (
        <SOWForm
          onSubmit={(data) => createMutation.mutate(data)}
          isLoading={createMutation.isPending}
        />
      )}

      <table className="w-full text-sm">
        <thead className="bg-gray-50 border-b">
          <tr>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Year</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Period</th>
            <th className="text-right px-3 py-2 font-medium text-gray-600">Budget</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Status</th>
            <th className="text-left px-3 py-2 font-medium text-gray-600">Scope</th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {sows?.map((sow) => (
            <tr key={sow.id}>
              <td className="px-3 py-2">{YEAR_LABELS[sow.fiscal_year] ?? `Year ${sow.fiscal_year}`}</td>
              <td className="px-3 py-2">{sow.period_start} — {sow.period_end}</td>
              <td className="px-3 py-2 text-right">${sow.budget_amount.toLocaleString()}</td>
              <td className="px-3 py-2"><StatusBadge status={sow.status} /></td>
              <td className="px-3 py-2 max-w-xs truncate">{sow.scope_text || '—'}</td>
            </tr>
          ))}
          {(!sows || sows.length === 0) && (
            <tr><td colSpan={5} className="px-3 py-4 text-center text-gray-400">No SOWs yet</td></tr>
          )}
        </tbody>
      </table>
    </div>
  );
}

/* ─── SOW Form ─── */

function SOWForm({
  onSubmit,
  isLoading,
}: {
  onSubmit: (data: Partial<StatementOfWork>) => void;
  isLoading: boolean;
}) {
  const [form, setForm] = useState({
    fiscal_year: 1,
    period_start: '',
    period_end: '',
    budget_amount: 0,
    scope_text: '',
    status: 'draft',
  });

  return (
    <form onSubmit={(e) => { e.preventDefault(); onSubmit(form); }} className="bg-gray-50 p-4 rounded border space-y-3">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Fiscal Year</label>
          <select value={form.fiscal_year}
            onChange={(e) => setForm({ ...form, fiscal_year: parseInt(e.target.value) })}
            className="w-full border rounded px-2 py-1.5 text-sm">
            {PROJECT_YEARS.map((y) => (
              <option key={y} value={y}>{YEAR_LABELS[y]}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Period Start</label>
          <input type="date" required value={form.period_start}
            onChange={(e) => setForm({ ...form, period_start: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Period End</label>
          <input type="date" required value={form.period_end}
            onChange={(e) => setForm({ ...form, period_end: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">Budget ($)</label>
          <CurrencyInput value={form.budget_amount} required
            onChange={(val) => setForm({ ...form, budget_amount: val })}
            className="w-full border rounded px-2 py-1.5 text-sm" />
        </div>
        <div className="col-span-2">
          <label className="block text-xs font-medium text-gray-600 mb-1">Scope</label>
          <input type="text" value={form.scope_text}
            onChange={(e) => setForm({ ...form, scope_text: e.target.value })}
            className="w-full border rounded px-2 py-1.5 text-sm"
            placeholder="Brief description of work scope" />
        </div>
      </div>
      <button type="submit" disabled={isLoading}
        className="px-3 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50">
        {isLoading ? 'Adding...' : 'Add SOW'}
      </button>
    </form>
  );
}

/* ─── Subaward Institution Form ─── */

function SubawardForm({
  grant,
  onSubmit,
  isLoading,
}: {
  grant: Grant;
  onSubmit: (data: Partial<Subaward>) => void;
  isLoading: boolean;
}) {
  const [form, setForm] = useState({
    institution: '',
    pi_name: '',
    total_amount: 0,
    salary_escalation_rate: 0.03,
    start_date: grant.start_date || '2026-05-01',
    end_date: grant.end_date || '2031-04-30',
    status: 'active',
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); onSubmit(form); }}
      className="bg-white p-6 rounded-lg border space-y-4"
    >
      <h2 className="font-semibold text-lg">New Subaward Institution</h2>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Institution Name</label>
          <input type="text" required value={form.institution}
            onChange={(e) => setForm({ ...form, institution: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Sub-PI Name</label>
          <input type="text" required value={form.pi_name}
            onChange={(e) => setForm({ ...form, pi_name: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Total Amount ($)</label>
          <CurrencyInput value={form.total_amount} required
            onChange={(val) => setForm({ ...form, total_amount: val })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Salary Escalation (%)</label>
          <input type="number" step="0.1" min="0" max="20"
            value={form.salary_escalation_rate === 0 ? '' : (form.salary_escalation_rate * 100).toFixed(1)}
            onChange={(e) => setForm({ ...form, salary_escalation_rate: (parseFloat(e.target.value) || 0) / 100 })}
            className="w-full border rounded-md px-3 py-2 text-sm"
            placeholder="e.g. 3.0" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
          <input type="date" required value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
          <input type="date" required value={form.end_date}
            onChange={(e) => setForm({ ...form, end_date: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
          <select value={form.status}
            onChange={(e) => setForm({ ...form, status: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm">
            <option value="active">Active</option>
            <option value="pending">Pending</option>
            <option value="completed">Completed</option>
            <option value="closed">Closed</option>
          </select>
        </div>
      </div>
      <p className="text-xs text-gray-400">
        After creating, add overhead and fringe rates via the Overhead &amp; Fringe tab.
      </p>
      <button type="submit" disabled={isLoading}
        className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50">
        {isLoading ? 'Creating...' : 'Create Subaward Institution'}
      </button>
    </form>
  );
}
