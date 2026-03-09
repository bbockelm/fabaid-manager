'use client';

import Link from 'next/link';
import { useGrant } from '@/lib/grant-context';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { ScrollableTable } from '@/components/ScrollableTable';

const PROJECT_YEARS = [1, 2, 3, 4, 5];
const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

export default function DashboardPage() {
  const { grant, grantId, isLoading } = useGrant();

  const { data: personnel } = useQuery({
    queryKey: ['personnel', grantId],
    queryFn: () => api.personnel.list(grantId!),
    enabled: !!grantId,
  });

  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  const { data: wbsAreas } = useQuery({
    queryKey: ['wbs', grantId],
    queryFn: () => api.wbs.list(grantId!),
    enabled: !!grantId,
  });

  // Budget totals are now derived from institution budgets (versioned)
  // The dashboard shows high-level stats from the grant itself

  if (isLoading) return <div className="p-4">Loading project...</div>;

  if (!grant) {
    return (
      <div className="max-w-2xl mx-auto mt-12 text-center">
        <h1 className="text-2xl font-bold text-nsf-blue mb-4">Welcome to Project Tracker</h1>
        <p className="text-gray-600 mb-6">
          No project has been set up yet. Go to{' '}
          <Link href="/settings" className="text-nsf-light hover:underline">
            Settings
          </Link>{' '}
          to create the project grant.
        </p>
      </div>
    );
  }

  return (
    <div className="max-w-6xl space-y-8">
      {/* Project Header */}
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">{grant.title}</h1>
        <p className="text-gray-500">
          Award #{grant.award_number} · PI: {grant.pi_name} · {grant.agency}
        </p>
        <p className="text-sm text-gray-400 mt-1">
          {grant.start_date} to {grant.end_date} · Total Budget: $
          {grant.total_budget.toLocaleString()}
        </p>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <SummaryCard
          label="Total Budget"
          value={`$${grant.total_budget.toLocaleString()}`}
          sub="Award amount"
          color="blue"
        />
        <SummaryCard
          label="Personnel"
          value={`${personnel?.length ?? 0}`}
          sub="People on project"
          color="indigo"
        />
        <SummaryCard
          label="Institutions"
          value={`${(subawards?.length ?? 0) + (grant?.institution ? 1 : 0)}`}
          sub="Lead + subawards"
          color="amber"
        />
        <SummaryCard
          label="WBS Areas"
          value={`${wbsAreas?.length ?? 0}`}
          sub="Work breakdown areas"
          color="emerald"
        />
      </div>

      {/* Quick-nav cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <NavCard
          title="Personnel"
          count={personnel?.length ?? 0}
          unit="people"
          href="/personnel"
          icon="👥"
        />
        <NavCard
          title="Institutions"
          count={(subawards?.length ?? 0) + (grant?.institution ? 1 : 0)}
          unit="institutions"
          href="/institutions"
          icon="🏛️"
        />
        <NavCard
          title="WBS Areas"
          count={wbsAreas?.length ?? 0}
          unit="areas"
          href="/budget"
          icon="💰"
        />
        <NavCard
          title="Documents"
          count={null}
          unit="Manage files"
          href="/documents"
          icon="📄"
        />
      </div>

      {/* Annual Breakdown */}
      <section>
        <h2 className="text-lg font-semibold mb-3">Project Years</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-4">
          {PROJECT_YEARS.map((year) => (
            <Link
              key={year}
              href={`/budget?year=${year}`}
              className="bg-white p-4 rounded-lg border hover:border-nsf-light transition-colors"
            >
              <div className="font-medium text-nsf-blue">{YEAR_LABELS[year]}</div>
              <div className="text-xs text-gray-400 mt-1">View budgets &amp; line items</div>
            </Link>
          ))}
        </div>
      </section>

      {/* Institutions overview */}
      <section>
        <h2 className="text-lg font-semibold mb-3">Institutions</h2>
        <ScrollableTable className="bg-white rounded-lg border">
          <table className="w-full">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Institution</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Role</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">PI</th>
                <th className="text-left px-4 py-2 text-sm font-medium text-gray-600">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {grant.institution && (
                <tr>
                  <td className="px-4 py-2 text-sm font-medium">{grant.institution}</td>
                  <td className="px-4 py-2 text-sm"><span className="px-2 py-0.5 bg-nsf-blue/10 text-nsf-blue rounded-full text-xs">Lead</span></td>
                  <td className="px-4 py-2 text-sm">{grant.pi_name}</td>
                  <td className="px-4 py-2 text-sm"><span className="px-2 py-0.5 bg-green-100 text-green-800 rounded-full text-xs">{grant.status}</span></td>
                </tr>
              )}
              {subawards?.map((sub) => (
                <tr key={sub.id}>
                  <td className="px-4 py-2 text-sm font-medium">{sub.institution}</td>
                  <td className="px-4 py-2 text-sm"><span className="px-2 py-0.5 bg-orange-100 text-orange-800 rounded-full text-xs">Subaward</span></td>
                  <td className="px-4 py-2 text-sm">{sub.pi_name}</td>
                  <td className="px-4 py-2 text-sm"><span className="px-2 py-0.5 bg-green-100 text-green-800 rounded-full text-xs">{sub.status}</span></td>
                </tr>
              ))}
              {!grant.institution && (!subawards || subawards.length === 0) && (
                <tr>
                  <td colSpan={4} className="px-4 py-4 text-center text-gray-400 text-sm">
                    No institutions configured yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </ScrollableTable>
      </section>
    </div>
  );
}

function SummaryCard({
  label,
  value,
  sub,
  color,
}: {
  label: string;
  value: string;
  sub: string;
  color: string;
}) {
  const bg: Record<string, string> = {
    blue: 'bg-blue-50 border-blue-200',
    indigo: 'bg-indigo-50 border-indigo-200',
    amber: 'bg-amber-50 border-amber-200',
    emerald: 'bg-emerald-50 border-emerald-200',
  };
  return (
    <div className={`p-4 rounded-lg border ${bg[color] ?? 'bg-gray-50 border-gray-200'}`}>
      <div className="text-xs font-medium text-gray-500 uppercase tracking-wide">{label}</div>
      <div className="text-2xl font-bold mt-1">{value}</div>
      {sub && <div className="text-xs text-gray-500 mt-1">{sub}</div>}
    </div>
  );
}

function NavCard({
  title,
  count,
  unit,
  href,
  icon,
}: {
  title: string;
  count: number | null;
  unit: string;
  href: string;
  icon: string;
}) {
  return (
    <Link
      href={href}
      className="block p-5 bg-white rounded-lg border border-gray-200 shadow-sm hover:shadow-md hover:border-nsf-light transition-all"
    >
      <div className="text-2xl mb-2">{icon}</div>
      <h3 className="font-semibold text-gray-900">{title}</h3>
      <p className="text-sm text-gray-500">
        {count !== null ? `${count} ${unit}` : unit}
      </p>
    </Link>
  );
}
