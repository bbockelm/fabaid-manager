'use client';

import { useQuery } from '@tanstack/react-query';
import { api, BudgetSummaryByYear, BudgetSummary } from '@/lib/api';
import { use, useMemo, useState } from 'react';
import Link from 'next/link';

// Required for Next.js static export (output: 'export').
// Returns [] because grant IDs are dynamic; the Go server's SPA
// fallback serves index.html and the client-side router takes over.
export function generateStaticParams() {
  return [];
}

// Project years: FabAID is a 5-year project starting 1 May 2026
const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

export default function GrantDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = use(params);
  const [selectedYear, setSelectedYear] = useState<number | 'all'>('all');

  const { data: grant, isLoading: grantLoading } = useQuery({
    queryKey: ['grant', id],
    queryFn: () => api.grants.get(id),
  });

  const { data: wbsAreas } = useQuery({
    queryKey: ['wbs', id],
    queryFn: () => api.wbs.list(id),
  });

  const { data: personnel } = useQuery({
    queryKey: ['personnel', id],
    queryFn: () => api.personnel.list(id),
  });

  const { data: subawards } = useQuery({
    queryKey: ['subawards', id],
    queryFn: () => api.subawards.list(id),
  });

  const { data: budgetSummary } = useQuery({
    queryKey: ['budget-summary', id],
    queryFn: () => api.budget.summary(id),
  });

  const { data: budgetByYear } = useQuery({
    queryKey: ['budget-summary-by-year', id],
    queryFn: () => api.budget.summaryByYear(id),
  });

  const { data: budgetItems } = useQuery({
    queryKey: ['budget-items', id, selectedYear === 'all' ? 0 : selectedYear],
    queryFn: () => api.budget.list(id, selectedYear === 'all' ? undefined : selectedYear),
  });

  // Group budget-by-year data for the selected year
  const filteredBudgetSummary = useMemo(() => {
    if (selectedYear === 'all') return budgetSummary ?? [];
    return (budgetByYear ?? []).filter(
      (row: BudgetSummaryByYear) => row.fiscal_year === selectedYear
    );
  }, [selectedYear, budgetSummary, budgetByYear]);

  // Compute per-year totals for the year tabs
  const yearTotals = useMemo(() => {
    const totals: Record<number, { planned: number; actual: number }> = {};
    for (const row of budgetByYear ?? []) {
      if (!totals[row.fiscal_year]) totals[row.fiscal_year] = { planned: 0, actual: 0 };
      totals[row.fiscal_year].planned += row.planned_total;
      totals[row.fiscal_year].actual += row.actual_total;
    }
    return totals;
  }, [budgetByYear]);

  if (grantLoading) return <div className="p-4">Loading grant...</div>;
  if (!grant) return <div className="p-4">Grant not found</div>;

  return (
    <div className="max-w-6xl space-y-8">
      {/* Header */}
      <div>
        <Link href="/grants" className="text-nsf-light text-sm hover:underline">
          ← Back to Grants
        </Link>
        <h1 className="text-2xl font-bold text-nsf-blue mt-2">{grant.title}</h1>
        <p className="text-gray-500">
          Award #{grant.award_number} · PI: {grant.pi_name} · {grant.agency}
        </p>
        <p className="text-sm text-gray-400 mt-1">
          {grant.start_date} to {grant.end_date} · Total Budget: $
          {grant.total_budget.toLocaleString()}
        </p>
      </div>

      {/* Year Selector Tabs */}
      <section>
        <h2 className="text-lg font-semibold mb-3">Budget by Year</h2>
        <div className="flex flex-wrap gap-2 mb-4">
          <button
            onClick={() => setSelectedYear('all')}
            className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
              selectedYear === 'all'
                ? 'bg-nsf-blue text-white'
                : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            All Years
          </button>
          {PROJECT_YEARS.map((year) => (
            <button
              key={year}
              onClick={() => setSelectedYear(year)}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                selectedYear === year
                  ? 'bg-nsf-blue text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
              }`}
            >
              {YEAR_LABELS[year]}
              {yearTotals[year] && (
                <span className="ml-1 text-xs opacity-75">
                  (${yearTotals[year].planned.toLocaleString()})
                </span>
              )}
            </button>
          ))}
        </div>

        {/* Budget Summary Table */}
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                {selectedYear === 'all' ? null : (
                  <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Year</th>
                )}
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Category</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">Planned</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">Actual</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">Remaining</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">% Spent</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {filteredBudgetSummary?.map((row: BudgetSummary | BudgetSummaryByYear, idx: number) => (
                <tr key={idx}>
                  {selectedYear === 'all' ? null : (
                    <td className="px-4 py-2 text-sm">
                      {'fiscal_year' in row ? YEAR_LABELS[(row as BudgetSummaryByYear).fiscal_year] ?? `Year ${(row as BudgetSummaryByYear).fiscal_year}` : ''}
                    </td>
                  )}
                  <td className="px-4 py-2 text-sm capitalize">
                    {row.category.replace(/_/g, ' ')}
                  </td>
                  <td className="px-4 py-2 text-sm text-right">
                    ${row.planned_total.toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-sm text-right">
                    ${row.actual_total.toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-sm text-right">
                    ${row.remaining.toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-sm text-right">{row.percent_spent}%</td>
                </tr>
              ))}
              {(!filteredBudgetSummary || filteredBudgetSummary.length === 0) && (
                <tr>
                  <td colSpan={selectedYear === 'all' ? 5 : 6} className="px-4 py-4 text-center text-gray-400 text-sm">
                    No budget data {selectedYear !== 'all' ? `for ${YEAR_LABELS[selectedYear]}` : 'yet'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Year Overview Cards (when viewing All Years) */}
      {selectedYear === 'all' && Object.keys(yearTotals).length > 0 && (
        <section>
          <h2 className="text-lg font-semibold mb-3">Annual Breakdown</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-4">
            {PROJECT_YEARS.map((year) => {
              const t = yearTotals[year];
              return (
                <button
                  key={year}
                  onClick={() => setSelectedYear(year)}
                  className="bg-white p-4 rounded-lg border hover:border-nsf-light transition-colors text-left"
                >
                  <div className="font-medium text-nsf-blue">{YEAR_LABELS[year]}</div>
                  <div className="text-sm text-gray-600 mt-1">
                    Planned: ${t ? t.planned.toLocaleString() : '0'}
                  </div>
                  <div className="text-sm text-gray-600">
                    Spent: ${t ? t.actual.toLocaleString() : '0'}
                  </div>
                  {t && t.planned > 0 && (
                    <div className="mt-2 w-full bg-gray-200 rounded-full h-2">
                      <div
                        className="bg-nsf-light h-2 rounded-full"
                        style={{ width: `${Math.min(100, (t.actual / t.planned) * 100)}%` }}
                      />
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        </section>
      )}

      {/* WBS Areas */}
      <section>
        <h2 className="text-lg font-semibold mb-3">WBS Areas</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {wbsAreas?.map((wbs) => (
            <div key={wbs.id} className="bg-white p-4 rounded-lg border">
              <div className="font-medium">{wbs.code}: {wbs.name}</div>
              {wbs.description && (
                <p className="text-sm text-gray-500 mt-1">{wbs.description}</p>
              )}
              <p className="text-sm text-gray-600 mt-2">
                Total Budget: ${wbs.budget.toLocaleString()}
              </p>
            </div>
          ))}
          {(!wbsAreas || wbsAreas.length === 0) && (
            <p className="text-gray-400 text-sm">No WBS areas defined</p>
          )}
        </div>
      </section>

      {/* Personnel */}
      <section>
        <h2 className="text-lg font-semibold mb-3">Personnel</h2>
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Name</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Role</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Title</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Institution</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">Salary</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {personnel?.map((p) => (
                <tr key={p.id}>
                  <td className="px-4 py-2 text-sm font-medium">{p.name}</td>
                  <td className="px-4 py-2 text-sm">{p.role}</td>
                  <td className="px-4 py-2 text-sm">{p.title || '-'}</td>
                  <td className="px-4 py-2 text-sm">{p.institution || '-'}</td>
                  <td className="px-4 py-2 text-sm text-right">${p.annual_salary.toLocaleString()}</td>
                </tr>
              ))}
              {(!personnel || personnel.length === 0) && (
                <tr>
                  <td colSpan={5} className="px-4 py-4 text-center text-gray-400 text-sm">
                    No personnel yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Subawards */}
      <section>
        <h2 className="text-lg font-semibold mb-3">Subawards</h2>
        <div className="bg-white rounded-lg border overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Institution</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">PI</th>
                <th className="text-right px-4 py-2 text-sm font-medium text-gray-600">Amount</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Period</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {subawards?.map((sub) => (
                <tr key={sub.id}>
                  <td className="px-4 py-2 text-sm font-medium">
                    <Link
                      href={`/grants/${id}/subawards/${sub.id}`}
                      className="text-nsf-light hover:underline"
                    >
                      {sub.institution}
                    </Link>
                  </td>
                  <td className="px-4 py-2 text-sm">{sub.pi_name}</td>
                  <td className="px-4 py-2 text-sm text-right">
                    ${sub.total_amount.toLocaleString()}
                  </td>
                  <td className="px-4 py-2 text-sm">
                    {sub.start_date} — {sub.end_date}
                  </td>
                  <td className="px-4 py-2 text-sm capitalize">{sub.status}</td>
                </tr>
              ))}
              {(!subawards || subawards.length === 0) && (
                <tr>
                  <td colSpan={5} className="px-4 py-4 text-center text-gray-400 text-sm">
                    No subawards yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
