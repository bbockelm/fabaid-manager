'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, Personnel, PersonnelBudgetEntry } from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { useState, useMemo, useRef, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import CurrencyInput from '@/components/CurrencyInput';
import { ScrollableTable } from '@/components/ScrollableTable';
import Link from 'next/link';

// NSF 1030 role categories — the value stored in the DB "role" column
const NSF_ROLES: { value: string; label: string }[] = [
  { value: 'pi', label: 'PI' },
  { value: 'co_pi', label: 'Co-PI' },
  { value: 'subaward_pi', label: 'Subaward PI' },
  { value: 'senior_personnel', label: 'Senior Personnel' },
  { value: 'postdoc', label: 'Postdoc' },
  { value: 'other_professional', label: 'Other Professional' },
  { value: 'graduate_student', label: 'Graduate Student' },
  { value: 'undergraduate_student', label: 'Undergraduate Student' },
  { value: 'clerical', label: 'Clerical' },
  { value: 'other', label: 'Other' },
];

const ROLE_LABEL_MAP: Record<string, string> = Object.fromEntries(
  NSF_ROLES.map((r) => [r.value, r.label])
);

export default function PersonnelPage() {
  return (
    <Suspense fallback={<div className="p-4">Loading personnel...</div>}>
      <PersonnelPageInner />
    </Suspense>
  );
}

function PersonnelPageInner() {
  const { grantId, isLoading: grantLoading } = useGrant();
  const { isSubawardAdmin, permittedInstitutions } = useAuth();
  const queryClient = useQueryClient();
  const searchParams = useSearchParams();
  const focusPersonId = searchParams.get('person');
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(focusPersonId);
  const [expandedId, setExpandedId] = useState<string | null>(focusPersonId);
  const didScroll = useRef(false);

  const { data: grant } = useQuery({
    queryKey: ['grant', grantId],
    queryFn: () => api.grants.get(grantId!),
    enabled: !!grantId,
  });

  const { data: personnel, isLoading } = useQuery({
    queryKey: ['personnel', grantId],
    queryFn: () => api.personnel.list(grantId!),
    enabled: !!grantId,
  });

  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  // Build unique list of institutions from lead grant + subawards
  const allInstitutions = useMemo(() => {
    const set = new Set<string>();
    if (grant?.institution) set.add(grant.institution);
    subawards?.forEach((s) => { if (s.institution) set.add(s.institution); });
    return Array.from(set).sort();
  }, [grant, subawards]);

  // Subaward admins only see their permitted institutions
  const institutions = isSubawardAdmin
    ? allInstitutions.filter((i) => permittedInstitutions.includes(i))
    : allInstitutions;

  // Filter personnel for subaward admins
  const visiblePersonnel = isSubawardAdmin
    ? (personnel ?? []).filter((p) => permittedInstitutions.includes(p.institution || ''))
    : personnel;

  const createMutation = useMutation({
    mutationFn: (data: Partial<Personnel>) => api.personnel.create(grantId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['personnel', grantId] });
      queryClient.invalidateQueries({ queryKey: ['personnel-titles', grantId] });
      setShowForm(false);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Personnel> }) =>
      api.personnel.update(grantId!, id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['personnel', grantId] });
      queryClient.invalidateQueries({ queryKey: ['personnel-titles', grantId] });
      setEditingId(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.personnel.delete(grantId!, id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['personnel', grantId] });
      queryClient.invalidateQueries({ queryKey: ['personnel-titles', grantId] });
    },
  });

  if (grantLoading || isLoading) return <div className="p-4">Loading personnel...</div>;
  if (!grantId) return <div className="p-4">No project configured. <Link href="/settings" className="text-nsf-light hover:underline">Set up project</Link></div>;

  return (
    <div className="max-w-6xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-nsf-blue">Personnel</h1>
          <p className="text-sm text-gray-500">Manage project personnel. Click a person to view budget details.</p>
        </div>
        <button
          onClick={() => { setShowForm(!showForm); setEditingId(null); }}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors"
        >
          {showForm ? 'Cancel' : '+ Add Person'}
        </button>
      </div>

      {showForm && (
        <PersonnelForm
          grantId={grantId}
          institutions={institutions}
          onSubmit={(data) => createMutation.mutate(data)}
          isLoading={createMutation.isPending}
        />
      )}

      <div className="space-y-6">
        {institutions.map((inst) => {
          const people = (visiblePersonnel ?? []).filter((p) => p.institution === inst);
          return (
            <ScrollableTable key={inst} className="bg-white rounded-lg border">
              <div className="px-4 py-3 bg-gray-50 border-b flex items-center gap-2">
                <h2 className="text-sm font-semibold text-nsf-blue">{inst}</h2>
                <span className="text-xs text-gray-400">({people.length} {people.length === 1 ? 'person' : 'people'})</span>
              </div>
              <table className="w-full">
                <thead className="bg-gray-50/50 border-b">
                  <tr>
                    <th className="w-6 px-2"></th>
                    <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Name</th>
                    <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Role</th>
                    <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Title</th>
                    <th className="text-right px-4 py-3 text-sm font-medium text-gray-600">Salary</th>
                    <th className="text-center px-4 py-3 text-sm font-medium text-gray-600">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {people.map((p) => (
                    <PersonnelRow
                      key={p.id}
                      person={p}
                      expanded={expandedId === p.id}
                      editing={editingId === p.id}
                      grantId={grantId}
                      institutions={institutions}
                      isFocused={p.id === focusPersonId && !didScroll.current}
                      onFocused={() => { didScroll.current = true; }}
                      onToggleExpand={() => setExpandedId(expandedId === p.id ? null : p.id)}
                      onEdit={() => setEditingId(editingId === p.id ? null : p.id)}
                      onUpdate={(data) => updateMutation.mutate({ id: p.id, data })}
                      onDelete={() => {
                        if (confirm(`Delete ${p.name}?`)) deleteMutation.mutate(p.id);
                      }}
                      isUpdating={updateMutation.isPending}
                    />
                  ))}
                  {people.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-4 py-4 text-center text-gray-400 text-sm">
                        No personnel at this institution.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </ScrollableTable>
          );
        })}
        {/* Unassigned personnel */}
        {(visiblePersonnel ?? []).some((p) => !p.institution || !institutions.includes(p.institution)) && (
          <ScrollableTable className="bg-white rounded-lg border">
            <div className="px-4 py-3 bg-gray-50 border-b">
              <h2 className="text-sm font-semibold text-gray-500">Unassigned</h2>
            </div>
            <table className="w-full">
              <thead className="bg-gray-50/50 border-b">
                <tr>
                  <th className="w-6 px-2"></th>
                  <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Name</th>
                  <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Role</th>
                  <th className="text-left px-4 py-3 text-sm font-medium text-gray-600">Title</th>
                  <th className="text-right px-4 py-3 text-sm font-medium text-gray-600">Salary</th>
                  <th className="text-center px-4 py-3 text-sm font-medium text-gray-600">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y">
                {(visiblePersonnel ?? []).filter((p) => !p.institution || !institutions.includes(p.institution)).map((p) => (
                  <PersonnelRow
                    key={p.id}
                    person={p}
                    expanded={expandedId === p.id}
                    editing={editingId === p.id}
                    grantId={grantId}
                    institutions={institutions}
                    isFocused={p.id === focusPersonId && !didScroll.current}
                    onFocused={() => { didScroll.current = true; }}
                    onToggleExpand={() => setExpandedId(expandedId === p.id ? null : p.id)}
                    onEdit={() => setEditingId(editingId === p.id ? null : p.id)}
                    onUpdate={(data) => updateMutation.mutate({ id: p.id, data })}
                    onDelete={() => {
                      if (confirm(`Delete ${p.name}?`)) deleteMutation.mutate(p.id);
                    }}
                    isUpdating={updateMutation.isPending}
                  />
                ))}
              </tbody>
            </table>
          </ScrollableTable>
        )}
        {(!visiblePersonnel || visiblePersonnel.length === 0) && institutions.length === 0 && (
          <div className="bg-white rounded-lg border p-8 text-center text-gray-400">
            No personnel yet. Click &quot;+ Add Person&quot; to get started.
          </div>
        )}
      </div>
    </div>
  );
}

// --- Expandable row ---

function PersonnelRow({
  person,
  expanded,
  editing,
  grantId,
  institutions,
  isFocused,
  onFocused,
  onToggleExpand,
  onEdit,
  onUpdate,
  onDelete,
  isUpdating,
}: {
  person: Personnel;
  expanded: boolean;
  editing: boolean;
  grantId: string;
  institutions: string[];
  isFocused?: boolean;
  onFocused?: () => void;
  onToggleExpand: () => void;
  onEdit: () => void;
  onUpdate: (data: Partial<Personnel>) => void;
  onDelete: () => void;
  isUpdating: boolean;
}) {
  const roleLabel = ROLE_LABEL_MAP[person.role] ?? person.role;
  const rowRef = useRef<HTMLTableRowElement>(null);

  useEffect(() => {
    if (isFocused && rowRef.current) {
      rowRef.current.scrollIntoView({ behavior: 'smooth', block: 'center' });
      onFocused?.();
    }
  }, [isFocused, onFocused]);

  return (
    <>
      <tr
        ref={rowRef}
        className={`hover:bg-gray-50 cursor-pointer ${expanded ? 'bg-blue-50/40' : ''} ${isFocused ? 'ring-2 ring-nsf-light ring-inset' : ''}`}
        onClick={onToggleExpand}
      >
        <td className="px-2 text-gray-400 text-xs select-none">{expanded ? '▼' : '▶'}</td>
        <td className="px-4 py-3 text-sm font-medium text-gray-900">{person.name}</td>
        <td className="px-4 py-3 text-sm">
          <span className="inline-block bg-blue-100 text-blue-800 text-xs font-medium px-2 py-0.5 rounded">
            {roleLabel}
          </span>
        </td>
        <td className="px-4 py-3 text-sm text-gray-600">{person.title || '—'}</td>
        <td className="px-4 py-3 text-sm text-gray-600 text-right">${person.annual_salary.toLocaleString()}</td>
        <td className="px-4 py-3 text-sm text-center space-x-2" onClick={(e) => e.stopPropagation()}>
          <button onClick={onEdit} className="text-nsf-light hover:underline text-xs">
            {editing ? 'Cancel' : 'Edit'}
          </button>
          <button onClick={onDelete} className="text-red-500 hover:underline text-xs">
            Delete
          </button>
        </td>
      </tr>
      {editing && (
        <tr>
          <td colSpan={6} className="px-4 py-4 bg-gray-50">
            <PersonnelForm
              grantId={grantId}
              initial={person}
              institutions={institutions}
              onSubmit={onUpdate}
              isLoading={isUpdating}
              submitLabel="Save Changes"
            />
          </td>
        </tr>
      )}
      {expanded && !editing && (
        <tr>
          <td colSpan={6} className="p-0">
            <PersonnelDetail person={person} grantId={grantId} />
          </td>
        </tr>
      )}
    </>
  );
}

// --- Expanded detail: salary + budget data grouped by institution ---

function PersonnelDetail({ person, grantId }: { person: Personnel; grantId: string }) {
  const { data: budgetEntries, isLoading } = useQuery({
    queryKey: ['personnel-budget', grantId, person.id],
    queryFn: () => api.personnel.budgetSummary(grantId, person.id),
  });

  // Group entries by institution
  const grouped = useMemo(() => {
    const map = new Map<string, PersonnelBudgetEntry[]>();
    (budgetEntries ?? []).forEach((e) => {
      const list = map.get(e.institution) ?? [];
      list.push(e);
      map.set(e.institution, list);
    });
    return map;
  }, [budgetEntries]);

  return (
    <div className="bg-blue-50/30 border-t px-8 py-4 space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4 text-sm">
        <div>
          <span className="text-gray-500">Annual Salary:</span>{' '}
          <span className="font-medium">${person.annual_salary.toLocaleString()}</span>
        </div>
        <div>
          <span className="text-gray-500">Total Funded Months:</span>{' '}
          <span className="font-medium">
            {(budgetEntries ?? []).reduce((s, e) => s + e.effort_months, 0).toFixed(1)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">WBS Area:</span>{' '}
          <span className="font-medium">{person.wbs_area_id || '—'}</span>
        </div>
      </div>

      {person.start_date && (
        <div className="text-sm text-gray-500">
          Period: {person.start_date} — {person.end_date || 'ongoing'}
        </div>
      )}

      <div>
        <h4 className="text-sm font-semibold text-gray-700 mb-2">Year-by-Year Budget</h4>
        {isLoading && <p className="text-xs text-gray-400">Loading budget data...</p>}
        {!isLoading && grouped.size === 0 && (
          <p className="text-xs text-gray-400">
            No budget line items found. Add this person to institution budgets on the{' '}
            <Link href="/institutions" className="text-nsf-light hover:underline">Institutions</Link> page.
          </p>
        )}
        {!isLoading && grouped.size > 0 && (
          <ScrollableTable className="space-y-3">
            {Array.from(grouped.entries()).map(([institution, entries]) => (
              <div key={institution}>
                <div className="text-xs font-medium text-gray-600 mb-1">{institution}</div>
                <table className="w-full text-xs" style={{minWidth: '500px'}}>
                  <thead>
                    <tr className="border-b">
                      <th className="text-left py-1 text-gray-500 font-medium">Year</th>
                      <th className="text-right py-1 text-gray-500 font-medium">Base Salary</th>
                      <th className="text-right py-1 text-gray-500 font-medium">Effort (mo)</th>
                      <th className="text-right py-1 text-gray-500 font-medium">Salary</th>
                      <th className="text-right py-1 text-gray-500 font-medium">Fringe</th>
                      <th className="text-right py-1 text-gray-500 font-medium">Fully Loaded</th>
                    </tr>
                  </thead>
                  <tbody>
                    {entries.map((e) => {
                      const escalated = person.annual_salary * Math.pow(1 + (e.salary_escalation_rate || 0), e.fiscal_year - 1);
                      return (
                        <tr key={e.fiscal_year} className="border-b border-gray-100">
                          <td className="py-1">Year {e.fiscal_year}</td>
                          <td className="py-1 text-right">${escalated.toLocaleString(undefined, { maximumFractionDigits: 0 })}</td>
                          <td className="py-1 text-right">{e.effort_months.toFixed(1)}</td>
                          <td className="py-1 text-right">${e.salary_amount.toLocaleString()}</td>
                          <td className="py-1 text-right">${e.fringe_amount.toLocaleString()}</td>
                          <td className="py-1 text-right">${(e.salary_amount + e.fringe_amount).toLocaleString()}</td>
                        </tr>
                      );
                    })}
                    <tr className="font-medium">
                      <td className="py-1">Total</td>
                      <td className="py-1"></td>
                      <td className="py-1 text-right">
                        {entries.reduce((s, e) => s + e.effort_months, 0).toFixed(1)}
                      </td>
                      <td className="py-1 text-right">
                        ${entries.reduce((s, e) => s + e.salary_amount, 0).toLocaleString()}
                      </td>
                      <td className="py-1 text-right">
                        ${entries.reduce((s, e) => s + e.fringe_amount, 0).toLocaleString()}
                      </td>
                      <td className="py-1 text-right">
                        ${entries.reduce((s, e) => s + e.salary_amount + e.fringe_amount, 0).toLocaleString()}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            ))}
          </ScrollableTable>
        )}
      </div>
    </div>
  );
}

// --- Add/Edit form ---

function PersonnelForm({
  grantId,
  initial,
  institutions,
  onSubmit,
  isLoading,
  submitLabel = 'Add Person',
}: {
  grantId: string;
  initial?: Partial<Personnel>;
  institutions: string[];
  onSubmit: (data: Partial<Personnel>) => void;
  isLoading: boolean;
  submitLabel?: string;
}) {
  const [form, setForm] = useState({
    name: initial?.name ?? '',
    role: initial?.role ?? 'other_professional',
    title: initial?.title ?? '',
    institution: initial?.institution ?? '',
    annual_salary: initial?.annual_salary ?? 0,
  });

  // Fetch existing titles for autocomplete
  const { data: knownTitles } = useQuery({
    queryKey: ['personnel-titles', grantId],
    queryFn: () => api.personnel.titles(grantId),
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(form);
      }}
      className="bg-white p-6 rounded-lg border space-y-4"
    >
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
          <input type="text" required value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm"
            placeholder="Full name" />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Role (NSF 1030) *</label>
          <select required value={form.role}
            onChange={(e) => setForm({ ...form, role: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm">
            {NSF_ROLES.map((r) => (
              <option key={r.value} value={r.value}>{r.label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
          <TitleAutocomplete
            value={form.title}
            onChange={(v) => setForm({ ...form, title: v })}
            suggestions={knownTitles ?? []}
            placeholder="e.g. Investigator, Programmer"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Institution</label>
          <select value={form.institution}
            onChange={(e) => setForm({ ...form, institution: e.target.value })}
            className="w-full border rounded-md px-3 py-2 text-sm">
            <option value="">— Select Institution —</option>
            {institutions.map((inst) => (
              <option key={inst} value={inst}>{inst}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Annual Salary ($)</label>
          <CurrencyInput value={form.annual_salary}
            onChange={(val) => setForm({ ...form, annual_salary: val })}
            className="w-full border rounded-md px-3 py-2 text-sm" />
        </div>
      </div>
      <button type="submit" disabled={isLoading}
        className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50">
        {isLoading ? 'Saving...' : submitLabel}
      </button>
    </form>
  );
}

// --- Title autocomplete input ---

function TitleAutocomplete({
  value,
  onChange,
  suggestions,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  suggestions: string[];
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const filtered = useMemo(() => {
    if (!value) return suggestions;
    const lower = value.toLowerCase();
    return suggestions.filter((s) => s.toLowerCase().includes(lower));
  }, [value, suggestions]);

  return (
    <div ref={ref} className="relative">
      <input
        type="text"
        value={value}
        onChange={(e) => { onChange(e.target.value); setOpen(true); }}
        onFocus={() => setOpen(true)}
        placeholder={placeholder}
        className="w-full border rounded-md px-3 py-2 text-sm"
      />
      {open && filtered.length > 0 && (
        <ul className="absolute z-10 mt-1 w-full bg-white border rounded-md shadow-lg max-h-40 overflow-auto text-sm">
          {filtered.map((s) => (
            <li
              key={s}
              className="px-3 py-1.5 hover:bg-blue-50 cursor-pointer"
              onMouseDown={(e) => { e.preventDefault(); onChange(s); setOpen(false); }}
            >
              {s}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
