'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  WBSArea,
  WBSEffortSummary,
  Personnel,
  PersonnelDefaultWBS,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useState, useMemo } from 'react';

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
  onEdit: (a: WBSArea) => void;
  onDelete: (a: WBSArea) => void;
  onAddChild: (parent: WBSArea) => void;
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
        <div className="hidden group-hover:flex items-center gap-1 flex-shrink-0">
          <button
            onClick={() => onAddChild(area)}
            className="text-xs text-blue-600 hover:underline"
            title="Add child WBS"
          >
            + child
          </button>
          <button
            onClick={() => onEdit(area)}
            className="text-xs text-gray-600 hover:underline"
          >
            edit
          </button>
          <button
            onClick={() => onDelete(area)}
            className="text-xs text-red-500 hover:underline"
          >
            delete
          </button>
        </div>
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
              <>
                <th key={`${y}-m`} className="text-right px-3 py-1 border-b">Months</th>
                <th key={`${y}-$`} className="text-right px-3 py-1 border-b">Amount</th>
              </>
            ))}
            <th className="text-right px-3 py-1 border-b">Months</th>
            <th className="text-right px-3 py-1 border-b">Amount</th>
          </tr>
        </thead>
        <tbody>
          {orderedAreaIds.map((areaId) => {
            const area = areas.find((a) => a.id === areaId);
            if (!area) return null;
            const yearMap = byWBS.get(areaId);
            let totalMonths = 0;
            let totalAmount = 0;

            return (
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
                    <>
                      <td key={`${y}-m`} className="text-right px-3 py-1.5 border-b tabular-nums">
                        {months > 0 ? months.toFixed(1) : '—'}
                      </td>
                      <td key={`${y}-$`} className="text-right px-3 py-1.5 border-b tabular-nums">
                        {amount > 0 ? fmt$(amount) : '—'}
                      </td>
                    </>
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
          })}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Default WBS Editor (per person)
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
  const childMap = useMemo(() => buildTree(areas), [areas]);
  const leafAreas = useMemo(
    () => areas.filter((a) => isLeaf(a.id, childMap)),
    [areas, childMap]
  );

  const { data: defaults } = useQuery({
    queryKey: ['personnel-default-wbs', grantId, person.id],
    queryFn: () => api.personnel.defaultWBS(grantId, person.id),
  });

  const [rows, setRows] = useState<{ wbs_area_id: string; percent: number }[]>([]);
  const [editing, setEditing] = useState(false);

  function startEdit() {
    setRows(
      (defaults ?? []).map((d) => ({
        wbs_area_id: d.wbs_area_id,
        percent: d.percent,
      }))
    );
    setEditing(true);
  }

  const saveMut = useMutation({
    mutationFn: () =>
      api.personnel.setDefaultWBS(grantId, person.id, rows),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['personnel-default-wbs', grantId, person.id] });
      setEditing(false);
    },
  });

  const totalPct = rows.reduce((s, r) => s + r.percent, 0);

  if (!editing) {
    return (
      <div className="flex items-center gap-2 flex-wrap">
        {defaults && defaults.length > 0 ? (
          defaults.map((d) => {
            const area = areas.find((a) => a.id === d.wbs_area_id);
            return (
              <span key={d.id} className="text-xs bg-blue-50 text-blue-700 px-2 py-0.5 rounded">
                {area?.code ?? '?'}: {d.percent}%
              </span>
            );
          })
        ) : (
          <span className="text-xs text-gray-400 italic">No default WBS</span>
        )}
        <button onClick={startEdit} className="text-xs text-blue-600 hover:underline ml-1">
          edit
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-2 bg-gray-50 p-2 rounded">
      {rows.map((row, idx) => (
        <div key={idx} className="flex items-center gap-2">
          <select
            className="border rounded px-1 py-0.5 text-xs flex-1"
            value={row.wbs_area_id}
            onChange={(e) => {
              const next = [...rows];
              next[idx] = { ...next[idx], wbs_area_id: e.target.value };
              setRows(next);
            }}
          >
            <option value="">Select WBS...</option>
            {leafAreas.map((a) => (
              <option key={a.id} value={a.id}>
                {a.code} — {a.name}
              </option>
            ))}
          </select>
          <input
            type="number"
            min={0}
            max={100}
            step={0.5}
            className="w-20 border rounded px-1 py-0.5 text-xs text-right"
            value={row.percent}
            onChange={(e) => {
              const next = [...rows];
              next[idx] = { ...next[idx], percent: parseFloat(e.target.value) || 0 };
              setRows(next);
            }}
          />
          <span className="text-xs text-gray-500">%</span>
          <button
            onClick={() => setRows(rows.filter((_, i) => i !== idx))}
            className="text-red-400 hover:text-red-600 text-xs"
          >
            ✕
          </button>
        </div>
      ))}
      <button
        onClick={() => setRows([...rows, { wbs_area_id: '', percent: 0 }])}
        className="text-xs text-blue-600 hover:underline"
      >
        + Add allocation
      </button>
      <div className="flex items-center gap-3 pt-1">
        <span className={`text-xs font-medium ${Math.abs(totalPct - 100) < 0.01 ? 'text-green-600' : totalPct > 100 ? 'text-red-600' : 'text-yellow-600'}`}>
          Total: {totalPct.toFixed(1)}%
        </span>
        <button
          onClick={() => saveMut.mutate()}
          disabled={saveMut.isPending}
          className="px-2 py-0.5 text-xs bg-nsf-blue text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          Save
        </button>
        <button
          onClick={() => setEditing(false)}
          className="px-2 py-0.5 text-xs border rounded hover:bg-gray-100"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function WBSPage() {
  const { grantId, isLoading: grantLoading } = useGrant();
  const queryClient = useQueryClient();

  // Active tab: 'tree' | 'effort' | 'defaults'
  const [tab, setTab] = useState<'tree' | 'effort' | 'defaults'>('tree');

  // form state
  const [showForm, setShowForm] = useState(false);
  const [editingArea, setEditingArea] = useState<WBSArea | undefined>();
  const [defaultParentId, setDefaultParentId] = useState<string | undefined>();

  // data
  const { data: areas, isLoading: areasLoading } = useQuery({
    queryKey: ['wbs', grantId],
    queryFn: () => api.wbs.list(grantId!),
    enabled: !!grantId,
  });

  const { data: effortSummary } = useQuery({
    queryKey: ['wbs-effort', grantId],
    queryFn: () => api.wbs.effortSummary(grantId!),
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
        {tab === 'tree' && (
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
              No WBS areas defined yet. Click &quot;+ New WBS Area&quot; to create the first one.
            </p>
          ) : (
            <div className="bg-white border rounded-lg divide-y">
              {roots.map((area) => (
                <WBSNode
                  key={area.id}
                  area={area}
                  childMap={childMap}
                  depth={0}
                  onEdit={handleEdit}
                  onDelete={handleDelete}
                  onAddChild={handleAddChild}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {/* ---- Effort tab ---- */}
      {tab === 'effort' && (
        <EffortSummaryTable
          summaries={effortSummary ?? []}
          areas={areas ?? []}
        />
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
            <div className="divide-y border rounded-lg bg-white">
              {personnel.map((p) => (
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
          )}
        </div>
      )}
    </div>
  );
}
