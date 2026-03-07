'use client';

import { useState, useRef, useEffect } from 'react';

interface InstitutionFilterProps {
  /** All available institution names */
  allInstitutions: string[];
  /** Currently selected institution names (empty = show all) */
  selected: string[];
  /** Called when selection changes */
  onChange: (selected: string[]) => void;
}

export function InstitutionFilter({ allInstitutions, selected, onChange }: InstitutionFilterProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  const allSelected = selected.length === 0 || selected.length === allInstitutions.length;

  function toggle(name: string) {
    if (selected.includes(name)) {
      const next = selected.filter((n) => n !== name);
      onChange(next.length === allInstitutions.length ? [] : next);
    } else {
      const next = [...selected, name];
      onChange(next.length === allInstitutions.length ? [] : next);
    }
  }

  function selectAll() {
    onChange([]);
  }

  const label = allSelected
    ? 'All Institutions'
    : selected.length === 1
    ? selected[0]
    : `${selected.length} institutions`;

  return (
    <div ref={ref} className="relative inline-block text-sm">
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 px-3 py-1.5 border rounded-md bg-white hover:bg-gray-50 text-gray-700"
      >
        <span>🏛️</span>
        <span>{label}</span>
        <span className="text-gray-400 text-xs ml-1">{open ? '▴' : '▾'}</span>
      </button>
      {open && (
        <div className="absolute z-20 mt-1 w-64 bg-white border rounded-md shadow-lg py-1">
          <button
            onClick={selectAll}
            className={`w-full text-left px-3 py-1.5 hover:bg-gray-50 flex items-center gap-2 ${
              allSelected ? 'font-semibold text-nsf-blue' : ''
            }`}
          >
            <span className="w-4 text-center">{allSelected ? '✓' : ''}</span>
            All Institutions
          </button>
          <div className="border-t my-1" />
          {allInstitutions.map((name) => {
            const checked = allSelected || selected.includes(name);
            return (
              <button
                key={name}
                onClick={() => toggle(name)}
                className={`w-full text-left px-3 py-1.5 hover:bg-gray-50 flex items-center gap-2 ${
                  checked && !allSelected ? 'font-semibold text-nsf-blue' : ''
                }`}
              >
                <span className="w-4 text-center">{checked ? '✓' : ''}</span>
                {name}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
