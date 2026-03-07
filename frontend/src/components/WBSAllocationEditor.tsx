'use client';

import { useState, useMemo, useCallback } from 'react';
import { WBSArea } from '@/lib/api';

// ---------------------------------------------------------------------------
// Color palette for top-level WBS sections (matches Effort-by-Year table)
// ---------------------------------------------------------------------------
export const ROOT_PALETTE = [
  { bg: 'bg-blue-100', text: 'text-blue-900', ring: 'ring-blue-200' },
  { bg: 'bg-amber-100', text: 'text-amber-900', ring: 'ring-amber-200' },
  { bg: 'bg-emerald-100', text: 'text-emerald-900', ring: 'ring-emerald-200' },
  { bg: 'bg-purple-100', text: 'text-purple-900', ring: 'ring-purple-200' },
  { bg: 'bg-rose-100', text: 'text-rose-900', ring: 'ring-rose-200' },
];

// ---------------------------------------------------------------------------
// Helpers (duplicated cheaply to keep component self-contained)
// ---------------------------------------------------------------------------

function buildChildMap(areas: WBSArea[]): Map<string | null, WBSArea[]> {
  const map = new Map<string | null, WBSArea[]>();
  for (const a of areas) {
    const key = a.parent_id ?? null;
    if (!map.has(key)) map.set(key, []);
    map.get(key)!.push(a);
  }
  return map;
}

function isLeaf(id: string, childMap: Map<string | null, WBSArea[]>): boolean {
  return !(childMap.get(id)?.length);
}

interface RootColorMap {
  colors: Map<string, typeof ROOT_PALETTE[0]>;
  rootOf: (id: string) => WBSArea | null;
}

export function useRootColors(areas: WBSArea[]): RootColorMap {
  return useMemo(() => {
    const areaMap = new Map(areas.map((a) => [a.id, a]));
    const childMap = buildChildMap(areas);
    const roots = childMap.get(null) ?? [];

    const colors = new Map<string, typeof ROOT_PALETTE[0]>();
    roots.forEach((r, i) => colors.set(r.id, ROOT_PALETTE[i % ROOT_PALETTE.length]));

    const cache = new Map<string, WBSArea | null>();
    function findRoot(id: string): WBSArea | null {
      if (cache.has(id)) return cache.get(id)!;
      const a = areaMap.get(id);
      if (!a) { cache.set(id, null); return null; }
      if (!a.parent_id) { cache.set(id, a); return a; }
      const r = findRoot(a.parent_id);
      cache.set(id, r);
      return r;
    }

    return { colors, rootOf: findRoot };
  }, [areas]);
}

// ---------------------------------------------------------------------------
// Grouped leaf areas: group leaves by their top-level root, tree-ordered
// ---------------------------------------------------------------------------

interface LeafGroup {
  root: WBSArea;
  color: typeof ROOT_PALETTE[0];
  leaves: WBSArea[];
}

function useGroupedLeaves(areas: WBSArea[]): LeafGroup[] {
  const { colors, rootOf } = useRootColors(areas);

  return useMemo(() => {
    const childMap = buildChildMap(areas);

    // Walk tree depth-first to preserve natural ordering
    const orderedLeaves: WBSArea[] = [];
    function walk(parentId: string | null) {
      for (const c of (childMap.get(parentId) ?? [])) {
        if (isLeaf(c.id, childMap)) orderedLeaves.push(c);
        else walk(c.id);
      }
    }
    walk(null);

    // Group by root
    const groupMap = new Map<string, LeafGroup>();
    for (const leaf of orderedLeaves) {
      const root = rootOf(leaf.id);
      if (!root) continue;
      if (!groupMap.has(root.id)) {
        groupMap.set(root.id, {
          root,
          color: colors.get(root.id) ?? ROOT_PALETTE[0],
          leaves: [],
        });
      }
      groupMap.get(root.id)!.leaves.push(leaf);
    }

    // Return groups in tree order (roots sorted by code)
    const roots = childMap.get(null) ?? [];
    return roots.filter((r) => groupMap.has(r.id)).map((r) => groupMap.get(r.id)!);
  }, [areas, colors, rootOf]);
}

// ---------------------------------------------------------------------------
// Shared WBS Allocation Editor
// ---------------------------------------------------------------------------

export interface WBSAllocEditorProps {
  /** All WBS areas on the grant (the component filters to leaves internally). */
  wbsAreas: WBSArea[];
  /** Current allocations as { wbs_area_id → percent }. */
  allocations: Record<string, number>;
  /** Called with new allocations map when the user clicks Save. */
  onSave: (allocs: Record<string, number>) => void | Promise<void>;
  /** Whether a save is currently in progress. */
  saving?: boolean;
  /** If true, inputs are disabled. */
  readOnly?: boolean;
  /** Label for the title line. Defaults to "WBS Cost Allocation (%)". */
  title?: string;
  /** Optional: a function to copy allocations from (e.g. copy salary WBS to fringe). */
  onCopyFrom?: () => Promise<Record<string, number>>;
  /** Label for the copy button. */
  copyLabel?: string;
  /** Called when the user clicks Cancel. If omitted, no cancel button shown. */
  onCancel?: () => void;
}

export function WBSAllocEditor({
  wbsAreas,
  allocations,
  onSave,
  saving = false,
  readOnly = false,
  title = 'WBS Cost Allocation (%)',
  onCopyFrom,
  copyLabel = 'Copy from Salary',
  onCancel,
}: WBSAllocEditorProps) {
  const groups = useGroupedLeaves(wbsAreas);

  const [localAllocs, setLocalAllocs] = useState<Record<string, number>>(allocations);
  const [dirty, setDirty] = useState(false);
  useMemo(() => { if (!dirty) setLocalAllocs(allocations); }, [allocations, dirty]);

  const total = Object.values(localAllocs).reduce((s, v) => s + v, 0);
  const isComplete = Math.abs(total - 100) < 0.01;

  const handleCopy = useCallback(async () => {
    if (!onCopyFrom) return;
    const copied = await onCopyFrom();
    setLocalAllocs(copied);
    setDirty(true);
  }, [onCopyFrom]);

  const handleSave = useCallback(async () => {
    const cleaned: Record<string, number> = {};
    for (const [id, pct] of Object.entries(localAllocs)) {
      if (pct > 0) cleaned[id] = pct;
    }
    await onSave(cleaned);
    setDirty(false);
  }, [localAllocs, onSave]);

  if (groups.length === 0) {
    return <p className="text-xs text-gray-400">No WBS areas defined. Create them on the WBS Areas page.</p>;
  }

  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-gray-600">{title}</div>
      <table className="text-xs w-full">
        <tbody>
          {groups.map((g) => {
            // Check if any leaf in this group has a nonzero allocation
            const groupHasAlloc = g.leaves.some((w) => (localAllocs[w.id] ?? 0) > 0);
            // Always show header if not readOnly; in readOnly mode, only if group has data
            if (readOnly && !groupHasAlloc) return null;

            return (
              <React.Fragment key={g.root.id}>
                <tr>
                  <td
                    colSpan={2}
                    className={`px-2 py-1 font-semibold text-[10px] tracking-wide uppercase ${g.color.bg} ${g.color.text} rounded-t`}
                  >
                    {g.root.code} — {g.root.name}
                  </td>
                </tr>
                {g.leaves.map((w) => {
                  if (readOnly && !(localAllocs[w.id] > 0)) return null;
                  const otherTotal = Object.entries(localAllocs)
                    .filter(([id]) => id !== w.id)
                    .reduce((s, [, v]) => s + v, 0);
                  const remaining = Math.round((100 - otherTotal) * 10) / 10;
                  return (
                    <tr key={w.id}>
                      <td className="pr-2 py-0.5 pl-3 text-gray-600 whitespace-nowrap">{w.code} {w.name}</td>
                      <td className="py-0.5">
                        <div className="flex items-center gap-1">
                          <input
                            type="number" min={0} max={100} step={1}
                            value={localAllocs[w.id] ?? 0} disabled={readOnly}
                            onChange={(e) => {
                              setLocalAllocs({ ...localAllocs, [w.id]: parseFloat(e.target.value) || 0 });
                              setDirty(true);
                            }}
                            className="w-16 border rounded px-1.5 py-0.5 text-xs text-right disabled:bg-gray-100"
                          />
                          <span className="text-gray-400">%</span>
                          {!readOnly && remaining > 0 && (localAllocs[w.id] ?? 0) !== remaining && (
                            <button
                              onClick={() => { setLocalAllocs({ ...localAllocs, [w.id]: remaining }); setDirty(true); }}
                              className="text-[10px] text-nsf-light hover:underline whitespace-nowrap"
                              title={`Assign remaining ${remaining}%`}
                            >
                              ← {remaining}%
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </React.Fragment>
            );
          })}
        </tbody>
      </table>
      <div className="flex items-center gap-3">
        <span className={`text-xs font-medium ${isComplete ? 'text-green-600' : total > 100 ? 'text-red-600' : 'text-amber-600'}`}>
          Total: {total.toFixed(1)}% {isComplete ? '✓' : '(should be 100%)'}
        </span>
        {!readOnly && (
          <button onClick={handleSave} disabled={saving || !dirty}
            className="px-2 py-1 bg-nsf-light text-white rounded text-xs hover:bg-nsf-blue disabled:opacity-50">
            {saving ? 'Saving...' : 'Save Allocations'}
          </button>
        )}
        {!readOnly && onCopyFrom && (
          <button onClick={handleCopy}
            className="px-2 py-1 bg-gray-100 text-gray-700 rounded text-xs hover:bg-gray-200">
            {copyLabel}
          </button>
        )}
        {onCancel && (
          <button onClick={onCancel}
            className="px-2 py-1 text-xs border rounded hover:bg-gray-100">
            Cancel
          </button>
        )}
      </div>
    </div>
  );
}

// Need React for Fragment usage in JSX
import React from 'react';
