'use client';

import { useQuery } from '@tanstack/react-query';
import {
  api,
  BudgetOverviewResponse,
  BudgetOverviewInstitution,
  BudgetOverviewWBS,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { InstitutionFilter } from '@/components/InstitutionFilter';
import { useState, useMemo, useCallback, useEffect } from 'react';
import Link from 'next/link';

const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

const CATEGORY_LABELS: Record<string, string> = {
  personnel: 'Personnel (Salary)',
  fringe: 'Fringe Benefits',
  travel: 'Travel',
  equipment: 'Equipment',
  supplies: 'Supplies',
  contractual: 'Contractual',
  other: 'Other Direct Costs',
  tuition: 'Tuition',
  participant_support: 'Participant Support',
};

const CATEGORY_ORDER = [
  'personnel', 'fringe', 'travel', 'equipment', 'supplies',
  'contractual', 'other', 'tuition', 'participant_support',
];

function fmt(n: number): string {
  return n.toLocaleString('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 0, maximumFractionDigits: 0 });
}

function fmtDetailed(n: number): string {
  return n.toLocaleString('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

type SortDir = 'asc' | 'desc';

function SortIcon({ active, dir }: { active: boolean; dir: SortDir }) {
  if (!active) return <span className="ml-1 text-gray-300">↕</span>;
  return <span className="ml-1">{dir === 'asc' ? '↑' : '↓'}</span>;
}

export default function BudgetOverviewPage() {
  const { grantId, grant, isLoading: grantLoading } = useGrant();
  const { isSubawardAdmin, permittedInstitutions } = useAuth();

  // Institution filter
  const [selectedInstitutions, setSelectedInstitutions] = useState<string[]>([]);

  // Fetch subawards to build institution list
  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  const allInstitutions = useMemo(() => {
    const names: string[] = [];
    if (grant?.institution) names.push(grant.institution);
    for (const s of subawards ?? []) {
      if (!names.includes(s.institution)) names.push(s.institution);
    }
    return names;
  }, [grant, subawards]);

  // For subaward admins, restrict to permitted institutions only
  const visibleInstitutions = isSubawardAdmin
    ? allInstitutions.filter((name) => permittedInstitutions.includes(name))
    : allInstitutions;

  // Auto-set filter for subaward admins
  useEffect(() => {
    if (isSubawardAdmin && permittedInstitutions.length > 0 && selectedInstitutions.length === 0) {
      setSelectedInstitutions(permittedInstitutions);
    }
  }, [isSubawardAdmin, permittedInstitutions]); // eslint-disable-line react-hooks/exhaustive-deps

  const { data: overview, isLoading } = useQuery({
    queryKey: ['budget-overview', grantId, selectedInstitutions],
    queryFn: () => api.budget.overview(grantId!, selectedInstitutions.length > 0 ? selectedInstitutions : undefined),
    enabled: !!grantId,
  });

  if (grantLoading || isLoading) {
    return <div className="p-6 text-gray-500">Loading budget overview...</div>;
  }

  if (!overview) {
    return <div className="p-6 text-gray-500">No budget data available.</div>;
  }

  return (
    <div className="p-6 space-y-8 max-w-[1400px]">
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Budget Overview</h1>
          {grant && (
            <p className="text-sm text-gray-500 mt-1">
              {grant.title} &mdash; Award: {fmt(overview.award_total)}
            </p>
          )}
        </div>
        {visibleInstitutions.length > 1 && (
          <InstitutionFilter
            allInstitutions={visibleInstitutions}
            selected={selectedInstitutions}
            onChange={setSelectedInstitutions}
          />
        )}
      </div>

      {/* Grand Total Summary */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-4">
        <SummaryCard label="Award Total" value={overview.award_total} />
        <SummaryCard label="Total Budgeted" value={overview.grand_total} />
        <SummaryCard label="Direct Costs" value={overview.grand_direct} />
        <SummaryCard label="Indirect (F&A)" value={overview.grand_indirect} />
        <SummaryCard label="Remaining" value={overview.award_total - overview.grand_total}
          className={overview.grand_total > overview.award_total ? 'text-red-600' : 'text-green-600'} />
      </div>

      {/* By Institution */}
      <InstitutionTable overview={overview} />

      {/* By WBS Area */}
      <WBSTable overview={overview} />
    </div>
  );
}

function SummaryCard({ label, value, className, isCurrency = true }: {
  label: string; value: number; className?: string; isCurrency?: boolean;
}) {
  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-4">
      <div className="text-xs text-gray-500 uppercase tracking-wide">{label}</div>
      <div className={`text-xl font-semibold mt-1 ${className ?? 'text-gray-900'}`}>
        {isCurrency ? fmt(value) : value}
      </div>
    </div>
  );
}

function InstitutionTable({ overview }: { overview: BudgetOverviewResponse }) {
  const [expandedInst, setExpandedInst] = useState<Set<string>>(new Set());
  const [sortCol, setSortCol] = useState<'name' | 'total' | number>('name');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

  const toggleSort = useCallback((col: typeof sortCol) => {
    setSortCol(prev => {
      if (prev === col) {
        setSortDir(d => d === 'asc' ? 'desc' : 'asc');
        return col;
      }
      setSortDir(typeof col === 'number' || col === 'total' ? 'desc' : 'asc');
      return col;
    });
  }, []);

  const sorted = useMemo(() => {
    const list = [...overview.institutions];
    const dir = sortDir === 'asc' ? 1 : -1;
    list.sort((a, b) => {
      if (sortCol === 'name') return dir * a.name.localeCompare(b.name);
      if (sortCol === 'total') return dir * (a.total - b.total);
      const ay = a.years[String(sortCol)]?.total ?? 0;
      const by = b.years[String(sortCol)]?.total ?? 0;
      return dir * (ay - by);
    });
    return list;
  }, [overview.institutions, sortCol, sortDir]);

  const toggle = (key: string) => {
    setExpandedInst(prev => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <div>
      <h2 className="text-lg font-semibold text-gray-800 mb-3">By Institution</h2>
      <div className="overflow-x-auto">
        <table className="min-w-full bg-white border border-gray-200 rounded-lg shadow-sm text-sm">
          <thead>
            <tr className="bg-gray-50 border-b border-gray-200">
              <th
                className="text-left px-4 py-3 font-medium text-gray-600 min-w-[220px] cursor-pointer select-none hover:text-gray-900"
                onClick={() => toggleSort('name')}
              >
                Institution <SortIcon active={sortCol === 'name'} dir={sortDir} />
              </th>
              {PROJECT_YEARS.map(y => (
                <th
                  key={y}
                  className="text-right px-4 py-3 font-medium text-gray-600 min-w-[110px] cursor-pointer select-none hover:text-gray-900"
                  onClick={() => toggleSort(y)}
                >
                  {YEAR_LABELS[y]} <SortIcon active={sortCol === y} dir={sortDir} />
                </th>
              ))}
              <th
                className="text-right px-4 py-3 font-medium text-gray-700 min-w-[120px] bg-gray-100 cursor-pointer select-none hover:text-gray-900"
                onClick={() => toggleSort('total')}
              >
                Total <SortIcon active={sortCol === 'total'} dir={sortDir} />
              </th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((inst) => {
              const key = `${inst.entity_type}:${inst.entity_id}`;
              const isExpanded = expandedInst.has(key);
              return (
                <InstitutionRow key={key} inst={inst} isExpanded={isExpanded} toggle={() => toggle(key)} />
              );
            })}
          </tbody>
          <tfoot>
            <tr className="bg-gray-100 border-t-2 border-gray-300 font-semibold">
              <td className="px-4 py-3 text-gray-800">Total</td>
              {PROJECT_YEARS.map(y => (
                <td key={y} className="text-right px-4 py-3 text-gray-800">
                  {fmt(overview.yearly_totals[String(y)] ?? 0)}
                </td>
              ))}
              <td className="text-right px-4 py-3 text-gray-900 bg-gray-200">
                {fmt(overview.grand_total)}
              </td>
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  );
}

function InstitutionRow({ inst, isExpanded, toggle }: {
  inst: BudgetOverviewInstitution; isExpanded: boolean; toggle: () => void;
}) {
  // Gather categories for this institution
  const categoriesPresent = new Set<string>();
  for (const yr of Object.values(inst.years)) {
    for (const cat of Object.keys(yr.by_category)) {
      categoriesPresent.add(cat);
    }
  }
  const orderedCategories = CATEGORY_ORDER.filter(c => categoriesPresent.has(c));
  const hasIndirect = inst.indirect_total > 0;

  const budgetHref = (y: number) =>
    `/budget?year=${y}&entity=${inst.entity_type}:${inst.entity_id}`;

  return (
    <>
      <tr
        className="border-b border-gray-100 hover:bg-blue-50/40 cursor-pointer"
        onClick={toggle}
      >
        <td className="px-4 py-2.5 font-medium text-gray-800">
          <span className="mr-2 text-gray-400 text-xs">{isExpanded ? '▼' : '▶'}</span>
          {inst.name}
          {inst.is_lead && (
            <span className="ml-2 text-xs bg-blue-100 text-blue-700 px-1.5 py-0.5 rounded">Lead</span>
          )}
        </td>
        {PROJECT_YEARS.map(y => {
          const yrData = inst.years[String(y)];
          return (
            <td key={y} className="text-right px-4 py-2.5 tabular-nums">
              {yrData ? (
                <Link
                  href={budgetHref(y)}
                  className="text-blue-600 hover:text-blue-800 hover:underline"
                  onClick={(e) => e.stopPropagation()}
                >
                  {fmt(yrData.total)}
                </Link>
              ) : (
                <span className="text-gray-300">—</span>
              )}
              {yrData && yrData.status === 'draft' && (
                <span className="ml-1 text-[10px] text-amber-600">(draft)</span>
              )}
            </td>
          );
        })}
        <td className="text-right px-4 py-2.5 font-semibold text-gray-800 bg-gray-50 tabular-nums">
          {fmt(inst.total)}
        </td>
      </tr>
      {isExpanded && (
        <>
          {/* Direct cost subtotal */}
          {hasIndirect && (
            <tr className="border-b border-gray-50 bg-blue-50/30">
              <td className="px-4 py-1.5 pl-10 text-gray-600 text-xs font-medium">
                Direct Costs
              </td>
              {PROJECT_YEARS.map(y => {
                const yrData = inst.years[String(y)];
                return (
                  <td key={y} className="text-right px-4 py-1.5 text-gray-600 text-xs tabular-nums font-medium">
                    {yrData?.direct_costs ? fmtDetailed(yrData.direct_costs) : ''}
                  </td>
                );
              })}
              <td className="text-right px-4 py-1.5 text-gray-700 text-xs bg-gray-50 tabular-nums font-medium">
                {fmtDetailed(inst.direct_total)}
              </td>
            </tr>
          )}
          {/* Category breakdown */}
          {orderedCategories.map(cat => (
            <tr key={cat} className="border-b border-gray-50 bg-gray-50/50">
              <td className="px-4 py-1.5 pl-14 text-gray-500 text-xs">
                {CATEGORY_LABELS[cat] ?? cat}
              </td>
              {PROJECT_YEARS.map(y => {
                const yrData = inst.years[String(y)];
                const val = yrData?.by_category[cat];
                return (
                  <td key={y} className="text-right px-4 py-1.5 text-gray-500 text-xs tabular-nums">
                    {val ? fmtDetailed(val) : ''}
                  </td>
                );
              })}
              <td className="text-right px-4 py-1.5 text-gray-600 text-xs bg-gray-50 tabular-nums font-medium">
                {fmtDetailed(
                  PROJECT_YEARS.reduce((sum, y) => sum + (inst.years[String(y)]?.by_category[cat] ?? 0), 0)
                )}
              </td>
            </tr>
          ))}
          {/* Indirect costs (F&A) row */}
          {hasIndirect && (
            <tr className="border-b border-gray-50 bg-blue-50/30">
              <td className="px-4 py-1.5 pl-10 text-gray-600 text-xs font-medium">
                Indirect Costs (F&amp;A)
              </td>
              {PROJECT_YEARS.map(y => {
                const yrData = inst.years[String(y)];
                return (
                  <td key={y} className="text-right px-4 py-1.5 text-gray-600 text-xs tabular-nums font-medium">
                    {yrData?.indirect_costs ? fmtDetailed(yrData.indirect_costs) : ''}
                  </td>
                );
              })}
              <td className="text-right px-4 py-1.5 text-gray-700 text-xs bg-gray-50 tabular-nums font-medium">
                {fmtDetailed(inst.indirect_total)}
              </td>
            </tr>
          )}
        </>
      )}
    </>
  );
}

function WBSTable({ overview }: { overview: BudgetOverviewResponse }) {
  const [sortCol, setSortCol] = useState<'name' | 'total' | number>('name');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

  const toggleSort = useCallback((col: typeof sortCol) => {
    setSortCol(prev => {
      if (prev === col) {
        setSortDir(d => d === 'asc' ? 'desc' : 'asc');
        return col;
      }
      setSortDir(typeof col === 'number' || col === 'total' ? 'desc' : 'asc');
      return col;
    });
  }, []);

  const sorted = useMemo(() => {
    const list = [...overview.wbs_areas];
    const dir = sortDir === 'asc' ? 1 : -1;
    list.sort((a, b) => {
      // "Unassigned" always stays last
      if (a.wbs_area_id === null && b.wbs_area_id !== null) return 1;
      if (a.wbs_area_id !== null && b.wbs_area_id === null) return -1;
      if (sortCol === 'name') return dir * (`${a.code} ${a.name}`).localeCompare(`${b.code} ${b.name}`);
      if (sortCol === 'total') return dir * (a.total - b.total);
      const ay = a.years[String(sortCol)] ?? 0;
      const by = b.years[String(sortCol)] ?? 0;
      return dir * (ay - by);
    });
    return list;
  }, [overview.wbs_areas, sortCol, sortDir]);

  return (
    <div>
      <h2 className="text-lg font-semibold text-gray-800 mb-1">By WBS Area</h2>
      <p className="text-xs text-gray-500 mb-3">Direct costs only — indirect (F&amp;A) costs are computed at the institution level.</p>
      <div className="overflow-x-auto">
        <table className="min-w-full bg-white border border-gray-200 rounded-lg shadow-sm text-sm">
          <thead>
            <tr className="bg-gray-50 border-b border-gray-200">
              <th
                className="text-left px-4 py-3 font-medium text-gray-600 min-w-[220px] cursor-pointer select-none hover:text-gray-900"
                onClick={() => toggleSort('name')}
              >
                WBS Area <SortIcon active={sortCol === 'name'} dir={sortDir} />
              </th>
              {PROJECT_YEARS.map(y => (
                <th
                  key={y}
                  className="text-right px-4 py-3 font-medium text-gray-600 min-w-[110px] cursor-pointer select-none hover:text-gray-900"
                  onClick={() => toggleSort(y)}
                >
                  {YEAR_LABELS[y]} <SortIcon active={sortCol === y} dir={sortDir} />
                </th>
              ))}
              <th
                className="text-right px-4 py-3 font-medium text-gray-700 min-w-[120px] bg-gray-100 cursor-pointer select-none hover:text-gray-900"
                onClick={() => toggleSort('total')}
              >
                Total <SortIcon active={sortCol === 'total'} dir={sortDir} />
              </th>
            </tr>
          </thead>
          <tbody>
            {sorted.map((wbs) => (
              <WBSRow key={wbs.wbs_area_id ?? 'unassigned'} wbs={wbs} />
            ))}
          </tbody>
          <tfoot>
            <tr className="bg-gray-100 border-t-2 border-gray-300 font-semibold">
              <td className="px-4 py-3 text-gray-800">Total</td>
              {PROJECT_YEARS.map(y => (
                <td key={y} className="text-right px-4 py-3 text-gray-800">
                  {fmt(overview.yearly_totals[String(y)] ?? 0)}
                </td>
              ))}
              <td className="text-right px-4 py-3 text-gray-900 bg-gray-200">
                {fmt(overview.grand_total)}
              </td>
            </tr>
          </tfoot>
        </table>
      </div>
    </div>
  );
}

function WBSRow({ wbs }: { wbs: BudgetOverviewWBS }) {
  const isUnassigned = wbs.wbs_area_id === null;

  return (
    <tr className={`border-b border-gray-100 ${isUnassigned ? 'bg-amber-50/50' : 'hover:bg-blue-50/40'}`}>
      <td className="px-4 py-2.5 text-gray-800">
        {isUnassigned ? (
          <span className="italic text-amber-700">Unassigned</span>
        ) : (
          <>
            <span className="font-mono text-gray-500 mr-2">{wbs.code}</span>
            <span className="font-medium">{wbs.name}</span>
          </>
        )}
      </td>
      {PROJECT_YEARS.map(y => {
        const val = wbs.years[String(y)];
        return (
          <td key={y} className="text-right px-4 py-2.5 text-gray-700 tabular-nums">
            {val ? fmt(val) : <span className="text-gray-300">—</span>}
          </td>
        );
      })}
      <td className={`text-right px-4 py-2.5 font-semibold tabular-nums bg-gray-50 ${isUnassigned ? 'text-amber-700' : 'text-gray-800'}`}>
        {fmt(wbs.total)}
      </td>
    </tr>
  );
}
