'use client';

import React from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  WBSArea,
  WBSEffortSummary,
  Personnel,
  PersonnelDefaultWBS,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { InstitutionFilter } from '@/components/InstitutionFilter';
import { WBSAllocEditor, useRootColors } from '@/components/WBSAllocationEditor';
import { useState, useMemo, useEffect, useCallback } from 'react';

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

/** Build a map of parentID → children for the tree */
function buildTree(areas: WBSArea[]): Map<string | null, WBSArea[]> {
  const map = new Map<string | null, WBSArea[]>();
  for (const a of areas) {
    const key = a.parent_id ?? null;
    if (!map.has(key)) map.set(key, []);
    map.get(key)!.push(a);
  }
  return map;
}

/** Check if a WBS area is a leaf node (no children) */
function isLeaf(id: string, childMap: Map<string | null, WBSArea[]>): boolean {
  return !(childMap.get(id)?.length);
}

/** Format dollars */
function fmt$(n: number) {
  return n.toLocaleString('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 0 });
}

// ---------------------------------------------------------------------------
// WBS Tree Node
// ---------------------------------------------------------------------------

function WBSNode({
  area,
  childMap,
  depth,
  onEdit,
  onDelete,
  onAddChild,
}: {
  area: WBSArea;
  childMap: Map<string | null, WBSArea[]>;
  depth: number;
  onEdit?: (a: WBSArea) => void;
  onDelete?: (a: WBSArea) => void;
  onAddChild?: (parent: WBSArea) => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const children = childMap.get(area.id) ?? [];
  const hasChildren = children.length > 0;
  const leaf = isLeaf(area.id, childMap);

  return (
    <div>
      <div
        className="flex items-center gap-2 py-1.5 px-2 rounded hover:bg-gray-50 group"
        style={{ paddingLeft: `${depth * 1.5 + 0.5}rem` }}
      >
        {/* expand/collapse toggle */}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="w-5 text-center text-gray-400 hover:text-gray-600 flex-shrink-0"
        >
          {hasChildren ? (collapsed ? '▸' : '▾') : '•'}
        </button>

        {/* code */}
        <span className="font-mono text-sm font-semibold text-nsf-blue w-20 flex-shrink-0">
          {area.code}
        </span>

        {/* name */}
        <span className="flex-1 text-sm">{area.name}</span>

        {/* leaf badge */}
        {leaf && (
          <span className="text-[10px] px-1.5 py-0.5 bg-green-100 text-green-700 rounded-full flex-shrink-0">
            leaf
          </span>
        )}

        {/* budget */}
        {area.budget > 0 && (
          <span className="text-xs text-gray-500 flex-shrink-0">
            {fmt$(area.budget)}
          </span>
        )}

        {/* actions */}
        {(onAddChild || onEdit || onDelete) && (
        <div className="hidden group-hover:flex items-center gap-1 flex-shrink-0">
          {onAddChild && <button
            onClick={() => onAddChild(area)}
            className="text-xs text-blue-600 hover:underline"
            title="Add child WBS"
          >
            + child
          </button>}
          {onEdit && <button
            onClick={() => onEdit(area)}
            className="text-xs text-gray-600 hover:underline"
          >
            edit
          </button>}
          {onDelete && <button
            onClick={() => onDelete(area)}
            className="text-xs text-red-500 hover:underline"
          >
            delete
          </button>}
        </div>
        )}
      </div>

      {/* description */}
      {area.description && (
        <div
          className="text-xs text-gray-400 pl-2 pb-1"
          style={{ paddingLeft: `${depth * 1.5 + 2.75}rem` }}
        >
          {area.description}
        </div>
      )}

      {/* children */}
      {!collapsed &&
        children.map((child) => (
          <WBSNode
            key={child.id}
            area={child}
            childMap={childMap}
            depth={depth + 1}
            onEdit={onEdit}
            onDelete={onDelete}
            onAddChild={onAddChild}
          />
        ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// WBS Form (create / edit)
// ---------------------------------------------------------------------------

function WBSForm({
  initial,
  parentId,
  allAreas,
  onSave,
  onCancel,
}: {
  initial?: WBSArea;
  parentId?: string;
  allAreas: WBSArea[];
  onSave: (data: Partial<WBSArea>) => void;
  onCancel: () => void;
}) {
  const [code, setCode] = useState(initial?.code ?? '');
  const [name, setName] = useState(initial?.name ?? '');
  const [description, setDescription] = useState(initial?.description ?? '');
  const [budget, setBudget] = useState(initial?.budget ?? 0);
  const [selParent, setSelParent] = useState<string>(
    initial?.parent_id ?? parentId ?? ''
  );

  return (
    <div className="bg-white border rounded-lg p-4 space-y-3 shadow-sm">
      <h3 className="font-semibold text-sm">
        {initial ? 'Edit WBS Area' : 'New WBS Area'}
      </h3>
      <div className="grid grid-cols-2 gap-3">
        <label className="block">
          <span className="text-xs font-medium text-gray-600">Code</span>
          <input
            className="mt-1 block w-full border rounded px-2 py-1 text-sm"
            value={code}
            placeholder="e.g. 1.1.2"
            onChange={(e) => setCode(e.target.value)}
          />
        </label>
        <label className="block">
          <span className="text-xs font-medium text-gray-600">Name</span>
          <input
            className="mt-1 block w-full border rounded px-2 py-1 text-sm"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="block col-span-2">
          <span className="text-xs font-medium text-gray-600">Parent</span>
          <select
            className="mt-1 block w-full border rounded px-2 py-1 text-sm"
            value={selParent}
            onChange={(e) => setSelParent(e.target.value)}
          >
            <option value="">(top-level)</option>
            {allAreas
              .filter((a) => a.id !== initial?.id)
              .map((a) => (
                <option key={a.id} value={a.id}>
                  {a.code} — {a.name}
                </option>
              ))}
          </select>
        </label>
        <label className="block col-span-2">
          <span className="text-xs font-medium text-gray-600">Description</span>
          <input
            className="mt-1 block w-full border rounded px-2 py-1 text-sm"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <label className="block">
          <span className="text-xs font-medium text-gray-600">Budget ($)</span>
          <input
            type="number"
            className="mt-1 block w-full border rounded px-2 py-1 text-sm"
            value={budget}
            onChange={(e) => setBudget(parseFloat(e.target.value) || 0)}
          />
        </label>
      </div>
      <div className="flex gap-2 pt-1">
        <button
          onClick={() =>
            onSave({
              code,
              name,
              description,
              budget,
              parent_id: selParent || undefined,
            })
          }
          disabled={!code || !name}
          className="px-3 py-1 text-sm bg-nsf-blue text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {initial ? 'Save' : 'Create'}
        </button>
        <button
          onClick={onCancel}
          className="px-3 py-1 text-sm border rounded hover:bg-gray-50"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Effort Summary Table
// ---------------------------------------------------------------------------

function EffortSummaryTable({
  summaries,
  areas,
}: {
  summaries: WBSEffortSummary[];
  areas: WBSArea[];
}) {
  // determine unique fiscal years
  const years = useMemo(() => {
    const set = new Set(summaries.map((s) => s.fiscal_year).filter((y) => y > 0));
    return Array.from(set).sort();
  }, [summaries]);

  // group by WBS area
  const byWBS = useMemo(() => {
    const map = new Map<string, Map<number, WBSEffortSummary>>();
    for (const s of summaries) {
      if (!map.has(s.wbs_area_id)) map.set(s.wbs_area_id, new Map());
      if (s.fiscal_year > 0) map.get(s.wbs_area_id)!.set(s.fiscal_year, s);
    }
    return map;
  }, [summaries]);

  const childMap = useMemo(() => buildTree(areas), [areas]);
  const roots = useMemo(() => childMap.get(null) ?? [], [childMap]);
  const areaMap = useMemo(() => new Map(areas.map((a) => [a.id, a])), [areas]);

  // Find root ancestor of any area
  const rootOf = useMemo(() => {
    const cache = new Map<string, WBSArea | null>();
    function find(id: string): WBSArea | null {
      if (cache.has(id)) return cache.get(id)!;
      const a = areaMap.get(id);
      if (!a) { cache.set(id, null); return null; }
      if (!a.parent_id) { cache.set(id, a); return a; }
      const r = find(a.parent_id);
      cache.set(id, r);
      return r;
    }
    return find;
  }, [areaMap]);

  // Flatten tree for table ordering
  const orderedAreaIds = useMemo(() => {
    const result: string[] = [];
    function walk(parentId: string | null) {
      const children = childMap.get(parentId) ?? [];
      for (const c of children) {
        result.push(c.id);
        walk(c.id);
      }
    }
    walk(null);
    return result;
  }, [childMap]);

  if (years.length === 0) {
    return (
      <p className="text-sm text-gray-500 italic">
        No budget data allocated to WBS areas yet. Allocate budget line items to WBS areas in the Budget page.
      </p>
    );
  }

  const colCount = 2 + years.length * 2 + 2;
  const sectionColors: Record<string, string> = {};
  const palette = [
    'bg-blue-100 text-blue-900',
    'bg-amber-100 text-amber-900',
    'bg-emerald-100 text-emerald-900',
    'bg-purple-100 text-purple-900',
    'bg-rose-100 text-rose-900',
  ];
  roots.forEach((r, i) => { sectionColors[r.id] = palette[i % palette.length]; });

  // Build row elements with section headers
  const tableRows: React.ReactNode[] = [];
  let lastRootId: string | null = null;
  for (const areaId of orderedAreaIds) {
    const area = areaMap.get(areaId);
    if (!area) continue;
    // Only render leaf nodes (no children) that have effort data
    if (!isLeaf(areaId, childMap)) continue;
    const yearMap = byWBS.get(areaId);
    if (!yearMap) continue;
    let hasData = false;
    for (const s of yearMap.values()) {
      if ((s.effort_months ?? 0) > 0 || (s.amount ?? 0) > 0) { hasData = true; break; }
    }
    if (!hasData) continue;

    // Insert section header when root changes
    const root = rootOf(areaId);
    if (root && root.id !== lastRootId) {
      lastRootId = root.id;
      tableRows.push(
        <tr key={`hdr-${root.id}`} className={sectionColors[root.id] ?? palette[0]}>
          <td colSpan={colCount} className="px-3 py-1.5 font-semibold text-xs tracking-wide uppercase border-b">
            {root.code} — {root.name}
          </td>
        </tr>
      );
    }

    let totalMonths = 0;
    let totalAmount = 0;

    tableRows.push(
      <tr key={areaId} className="hover:bg-gray-50">
        <td className="px-3 py-1.5 border-b font-mono">{area.code}</td>
        <td className="px-3 py-1.5 border-b">{area.name}</td>
        {years.map((y) => {
          const s = yearMap?.get(y);
          const months = s?.effort_months ?? 0;
          const amount = s?.amount ?? 0;
          totalMonths += months;
          totalAmount += amount;
          return (
            <React.Fragment key={y}>
              <td className="text-right px-3 py-1.5 border-b tabular-nums">
                {months > 0 ? months.toFixed(1) : '—'}
              </td>
              <td className="text-right px-3 py-1.5 border-b tabular-nums">
                {amount > 0 ? fmt$(amount) : '—'}
              </td>
            </React.Fragment>
          );
        })}
        <td className="text-right px-3 py-1.5 border-b font-semibold tabular-nums">
          {totalMonths > 0 ? totalMonths.toFixed(1) : '—'}
        </td>
        <td className="text-right px-3 py-1.5 border-b font-semibold tabular-nums">
          {totalAmount > 0 ? fmt$(totalAmount) : '—'}
        </td>
      </tr>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm border">
        <thead>
          <tr className="bg-gray-50">
            <th className="text-left px-3 py-2 border-b font-medium">WBS</th>
            <th className="text-left px-3 py-2 border-b font-medium">Name</th>
            {years.map((y) => (
              <th key={y} className="text-right px-3 py-2 border-b font-medium" colSpan={2}>
                Year {y}
              </th>
            ))}
            <th className="text-right px-3 py-2 border-b font-medium" colSpan={2}>
              Total
            </th>
          </tr>
          <tr className="bg-gray-50 text-xs text-gray-500">
            <th className="border-b" />
            <th className="border-b" />
            {years.map((y) => (
              <React.Fragment key={y}>
                <th className="text-right px-3 py-1 border-b">Months</th>
                <th className="text-right px-3 py-1 border-b">Amount</th>
              </React.Fragment>
            ))}
            <th className="text-right px-3 py-1 border-b">Months</th>
            <th className="text-right px-3 py-1 border-b">Amount</th>
          </tr>
        </thead>
        <tbody>
          {tableRows}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Default WBS Editor (per person) — uses shared WBSAllocEditor
// ---------------------------------------------------------------------------

function DefaultWBSEditor({
  grantId,
  person,
  areas,
}: {
  grantId: string;
  person: Personnel;
  areas: WBSArea[];
}) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);

  const { data: defaults } = useQuery({
    queryKey: ['personnel-default-wbs', grantId, person.id],
    queryFn: () => api.personnel.defaultWBS(grantId, person.id),
  });

  const allocMap = useMemo(() => {
    const m: Record<string, number> = {};
    for (const d of (defaults ?? [])) m[d.wbs_area_id] = d.percent;
    return m;
  }, [defaults]);

  const handleSave = useCallback(async (allocs: Record<string, number>) => {
    setSaving(true);
    try {
      const rows = Object.entries(allocs).map(([wbs_area_id, percent]) => ({ wbs_area_id, percent }));
      await api.personnel.setDefaultWBS(grantId, person.id, rows);
      queryClient.invalidateQueries({ queryKey: ['personnel-default-wbs', grantId, person.id] });
      setEditing(false);
    } finally {
      setSaving(false);
    }
  }, [grantId, person.id, queryClient]);

  const { colors, rootOf } = useRootColors(areas);

  if (!editing) {
    return (
      <div className="flex items-center gap-2 flex-wrap">
        {defaults && defaults.length > 0 ? (
          defaults.map((d) => {
            const area = areas.find((a) => a.id === d.wbs_area_id);
            const root = area ? rootOf(area.id) : null;
            const c = root ? colors.get(root.id) : null;
            const bgClass = c ? `${c.bg} ${c.text}` : 'bg-blue-50 text-blue-700';
            return (
              <span key={d.id} className={`text-xs px-2 py-0.5 rounded ${bgClass}`}>
                {area ? `${area.code} ${area.name}` : '?'}: {d.percent}%
              </span>
            );
          })
        ) : (
          <span className="text-xs text-gray-400 italic">No default WBS</span>
        )}
        <button onClick={() => setEditing(true)} className="text-xs text-blue-600 hover:underline ml-1">
          edit
        </button>
      </div>
    );
  }

  return (
    <div className="bg-gray-50 p-2 rounded">
      <WBSAllocEditor
        wbsAreas={areas}
        allocations={allocMap}
        onSave={handleSave}
        saving={saving}
        title="Default WBS Effort Breakdown (%)"
        onCancel={() => setEditing(false)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function WBSPage() {
  const { grant, grantId, isLoading: grantLoading } = useGrant();
  const { isAdmin, isGrantAdmin, isSubawardAdmin, permittedInstitutions } = useAuth();
  const canEditWBS = isAdmin || isGrantAdmin;
  const queryClient = useQueryClient();

  // Active tab: 'tree' | 'effort' | 'defaults'
  const [tab, setTab] = useState<'tree' | 'effort' | 'defaults'>('tree');

  // Institution filter for effort tab
  const [effortInstitutions, setEffortInstitutions] = useState<string[]>([]);

  // form state
  const [showForm, setShowForm] = useState(false);
  const [editingArea, setEditingArea] = useState<WBSArea | undefined>();
  const [defaultParentId, setDefaultParentId] = useState<string | undefined>();

  // Fetch subawards to build institution list
  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  // Build list of all institution names
  const allInstitutions = useMemo(() => {
    const names: string[] = [];
    if (grant?.institution) names.push(grant.institution);
    for (const s of subawards ?? []) {
      if (!names.includes(s.institution)) names.push(s.institution);
    }
    return names;
  }, [grant, subawards]);

  // Auto-set filter for subaward admins
  useEffect(() => {
    if (isSubawardAdmin && permittedInstitutions.length > 0 && effortInstitutions.length === 0) {
      setEffortInstitutions(permittedInstitutions);
    }
  }, [isSubawardAdmin, permittedInstitutions]); // eslint-disable-line react-hooks/exhaustive-deps

  // data
  const { data: areas, isLoading: areasLoading } = useQuery({
    queryKey: ['wbs', grantId],
    queryFn: () => api.wbs.list(grantId!),
    enabled: !!grantId,
  });

  const { data: effortSummary } = useQuery({
    queryKey: ['wbs-effort', grantId, effortInstitutions],
    queryFn: () => api.wbs.effortSummary(grantId!, effortInstitutions.length > 0 ? effortInstitutions : undefined),
    enabled: !!grantId && tab === 'effort',
  });

  const { data: personnel } = useQuery({
    queryKey: ['personnel', grantId],
    queryFn: () => api.personnel.list(grantId!),
    enabled: !!grantId && tab === 'defaults',
  });

  const childMap = useMemo(() => buildTree(areas ?? []), [areas]);
  const roots = useMemo(() => childMap.get(null) ?? [], [childMap]);

  // mutations
  const createMut = useMutation({
    mutationFn: (data: Partial<WBSArea>) => api.wbs.create(grantId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wbs', grantId] });
      setShowForm(false);
      setEditingArea(undefined);
      setDefaultParentId(undefined);
    },
  });

  const updateMut = useMutation({
    mutationFn: (data: Partial<WBSArea> & { id: string }) =>
      api.wbs.update(grantId!, data.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wbs', grantId] });
      setShowForm(false);
      setEditingArea(undefined);
    },
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.wbs.delete(grantId!, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['wbs', grantId] }),
  });

  function handleSave(data: Partial<WBSArea>) {
    if (editingArea) {
      updateMut.mutate({ ...data, id: editingArea.id });
    } else {
      createMut.mutate(data);
    }
  }

  function handleEdit(a: WBSArea) {
    setEditingArea(a);
    setDefaultParentId(undefined);
    setShowForm(true);
  }

  function handleAddChild(parent: WBSArea) {
    setEditingArea(undefined);
    setDefaultParentId(parent.id);
    setShowForm(true);
  }

  function handleDelete(a: WBSArea) {
    if (confirm(`Delete WBS "${a.code} — ${a.name}" and all its children?`)) {
      deleteMut.mutate(a.id);
    }
  }

  if (grantLoading || areasLoading) {
    return <p className="p-6 text-gray-500">Loading…</p>;
  }

  if (!grantId) {
    return <p className="p-6 text-gray-500">No grant found. Create a grant first.</p>;
  }

  return (
    <div className="p-6 max-w-6xl">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">WBS Areas</h1>
        {tab === 'tree' && canEditWBS && (
          <button
            onClick={() => {
              setEditingArea(undefined);
              setDefaultParentId(undefined);
              setShowForm(true);
            }}
            className="px-3 py-1.5 text-sm bg-nsf-blue text-white rounded hover:bg-blue-700"
          >
            + New WBS Area
          </button>
        )}
      </div>

      {/* tabs */}
      <div className="flex gap-1 mb-4 border-b">
        {(['tree', 'effort', 'defaults'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors -mb-px ${
              tab === t
                ? 'border-nsf-blue text-nsf-blue'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            {t === 'tree' ? 'Hierarchy' : t === 'effort' ? 'Effort by Year' : 'Default Allocations'}
          </button>
        ))}
      </div>

      {/* ---- Tree tab ---- */}
      {tab === 'tree' && (
        <div className="space-y-3">
          {showForm && (
            <WBSForm
              initial={editingArea}
              parentId={defaultParentId}
              allAreas={areas ?? []}
              onSave={handleSave}
              onCancel={() => {
                setShowForm(false);
                setEditingArea(undefined);
                setDefaultParentId(undefined);
              }}
            />
          )}

          {roots.length === 0 && !showForm ? (
            <p className="text-sm text-gray-500 italic">
              No WBS areas defined yet.{canEditWBS && ' Click "+ New WBS Area" to create the first one.'}
            </p>
          ) : (
            <div className="bg-white border rounded-lg divide-y">
              {roots.map((area) => (
                <WBSNode
                  key={area.id}
                  area={area}
                  childMap={childMap}
                  depth={0}
                  onEdit={canEditWBS ? handleEdit : undefined}
                  onDelete={canEditWBS ? handleDelete : undefined}
                  onAddChild={canEditWBS ? handleAddChild : undefined}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* ---- Effort tab ---- */}
      {tab === 'effort' && (
        <div className="space-y-3">
          {allInstitutions.length > 1 && (
            <div className="flex items-center gap-2">
              <span className="text-sm text-gray-600">Filter:</span>
              <InstitutionFilter
                allInstitutions={allInstitutions}
                selected={effortInstitutions}
                onChange={setEffortInstitutions}
              />
            </div>
          )}
          <EffortSummaryTable
            summaries={effortSummary ?? []}
            areas={areas ?? []}
          />
        </div>
      )}

      {/* ---- Person Default WBS tab ---- */}
      {tab === 'defaults' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Set each person&apos;s default WBS effort breakdown. These defaults are used to
            pre-populate budget entries.
          </p>
          {(!personnel || personnel.length === 0) ? (
            <p className="text-sm text-gray-500 italic">No personnel on this grant.</p>
          ) : (
            <div className="space-y-6">
              {allInstitutions
                .filter((inst) => !isSubawardAdmin || permittedInstitutions.includes(inst))
                .map((inst) => {
                const people = personnel.filter((p) => p.institution === inst);
                if (people.length === 0) return null;
                return (
                  <div key={inst}>
                    <h3 className="text-sm font-semibold text-gray-700 mb-2 px-1">{inst}</h3>
                    <div className="divide-y border rounded-lg bg-white">
                      {people.map((p) => (
                        <div key={p.id} className="px-4 py-3">
                          <div className="flex items-center gap-3 mb-1">
                            <span className="font-medium text-sm">{p.name}</span>
                            <span className="text-xs text-gray-500">{p.title || p.role}</span>
                          </div>
                          <DefaultWBSEditor
                            grantId={grantId!}
                            person={p}
                            areas={areas ?? []}
                          />
                        </div>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
