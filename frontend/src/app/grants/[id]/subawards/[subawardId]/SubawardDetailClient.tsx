'use client';

import { ScrollableTable } from '@/components/ScrollableTable';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, StatementOfWork } from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useParams } from 'next/navigation';
import { useState } from 'react';
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

export default function SubawardDetailClient() {
  const { grantId } = useGrant();
  const params = useParams();
  const subawardId = params.subawardId as string;
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);

  const { data: subawards, isLoading: subLoading } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });
  const subaward = subawards?.find((s) => s.id === subawardId);

  const { data: sows, isLoading: sowsLoading } = useQuery({
    queryKey: ['sow', grantId, subawardId],
    queryFn: () => api.sow.list(grantId!, subawardId),
    enabled: !!grantId,
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<StatementOfWork>) =>
      api.sow.create(grantId!, subawardId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['sow', grantId, subawardId] });
      setShowForm(false);
    },
  });

  if (subLoading || !grantId)
    return <div className="p-4 text-gray-500">Loading...</div>;
  if (!subaward)
    return <div className="p-4 text-gray-500">Subaward not found</div>;

  return (
    <div className="max-w-5xl space-y-8">
      {/* Breadcrumb + Header */}
      <div>
        <div className="flex gap-2 text-sm text-gray-500 mb-2">
          <Link href={`/grants/${grantId}`} className="text-nsf-light hover:underline">
            Grant
          </Link>
          <span>&rsaquo;</span>
          <Link href="/institutions" className="text-nsf-light hover:underline">
            Institutions
          </Link>
          <span>&rsaquo;</span>
          <span>{subaward.institution}</span>
        </div>
        <h1 className="text-2xl font-bold text-nsf-blue">
          {subaward.institution}
        </h1>
        <p className="text-gray-500 mt-1">
          Sub-PI: {subaward.pi_name} &middot; ${subaward.total_amount.toLocaleString()} &middot;{' '}
          {subaward.start_date} to {subaward.end_date}
        </p>
      </div>

      {/* Quick links */}
      <div className="flex flex-wrap gap-3">
        <Link
          href="/institutions"
          className="px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
        >
          Manage Budgets &amp; Rates
        </Link>
        <Link
          href="/sow"
          className="px-3 py-1.5 text-sm bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
        >
          SOW Management
        </Link>
      </div>

      {/* Statements of Work */}
      <section className="bg-white rounded-lg border">
        <div className="px-6 py-4 border-b flex items-center justify-between">
          <h2 className="font-semibold text-lg">Statements of Work</h2>
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

        <ScrollableTable>
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b">
              <tr>
                <th className="text-left px-4 py-2 font-medium text-gray-600">Year</th>
                <th className="text-left px-4 py-2 font-medium text-gray-600">Period</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Budget</th>
                <th className="text-left px-4 py-2 font-medium text-gray-600">Status</th>
                <th className="text-right px-4 py-2 font-medium text-gray-600">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {sows?.map((sow) => (
                <tr key={sow.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3">
                    {YEAR_LABELS[sow.fiscal_year] ?? `Year ${sow.fiscal_year}`}
                  </td>
                  <td className="px-4 py-3">
                    {sow.period_start} &mdash; {sow.period_end}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {sow.budget_id ? 'Linked' : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={sow.status} />
                  </td>
                  <td className="px-4 py-3 text-right space-x-3 whitespace-nowrap">
                    <a
                      href="/sow"
                      className="text-nsf-light hover:underline font-medium"
                    >
                      Edit Descriptions
                    </a>
                    <a
                      href={api.sow.renderUrl(grantId!, subawardId, sow.id)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-nsf-light hover:underline font-medium"
                    >
                      Preview
                    </a>
                  </td>
                </tr>
              ))}
              {(!sows || sows.length === 0) && (
                <tr>
                  <td colSpan={5} className="px-4 py-6 text-center text-gray-400">
                    No statements of work yet. Click &ldquo;+ New SOW&rdquo; to create one.
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

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    draft: 'bg-yellow-100 text-yellow-800',
    active: 'bg-green-100 text-green-800',
    final: 'bg-green-100 text-green-800',
    signed: 'bg-blue-100 text-blue-800',
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
    budget_amount: 0,
    scope_text: '',
    status: 'draft',
  });

  // Auto-calculate end date when fiscal year changes
  const handleYearChange = (year: number) => {
    // Attempt to compute period from subaward start + year offsets
    const start = new Date(subawardStart);
    const periodStart = new Date(start);
    periodStart.setFullYear(start.getFullYear() + (year - 1));
    const periodEnd = new Date(periodStart);
    periodEnd.setFullYear(periodStart.getFullYear() + 1);
    periodEnd.setDate(periodEnd.getDate() - 1);

    // Clamp to subaward end
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
        <div>
          <label className="block text-xs font-medium text-gray-600 mb-1">
            Budget ($)
          </label>
          <CurrencyInput
            value={form.budget_amount}
            required
            onChange={(val) => setForm({ ...form, budget_amount: val })}
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
        className="px-4 py-1.5 bg-nsf-light text-white rounded text-sm hover:bg-nsf-blue disabled:opacity-50"
      >
        {isLoading ? 'Creating...' : 'Create SOW'}
      </button>
    </form>
  );
}
