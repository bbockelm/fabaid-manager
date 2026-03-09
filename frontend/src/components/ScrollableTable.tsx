'use client';

import { useRef, useState, useEffect, useCallback } from 'react';

/**
 * Wraps a table in a horizontally scrollable container with
 * edge shadow indicators that signal more content is available.
 *
 * Usage:
 *   <ScrollableTable>
 *     <table className="w-full"> ... </table>
 *   </ScrollableTable>
 */
export function ScrollableTable({
  children,
  className = '',
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const check = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 1);
    setCanScrollRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
  }, []);

  useEffect(() => {
    check();
    const el = scrollRef.current;
    if (!el) return;
    el.addEventListener('scroll', check, { passive: true });
    const ro = new ResizeObserver(check);
    ro.observe(el);
    return () => {
      el.removeEventListener('scroll', check);
      ro.disconnect();
    };
  }, [check]);

  return (
    <div className={`relative ${className}`}>
      {/* Left shadow */}
      <div
        className={`pointer-events-none absolute inset-y-0 left-0 w-4 bg-gradient-to-r from-black/[0.06] to-transparent z-10 transition-opacity duration-150 ${
          canScrollLeft ? 'opacity-100' : 'opacity-0'
        }`}
      />
      {/* Right shadow + scroll hint */}
      <div
        className={`pointer-events-none absolute inset-y-0 right-0 w-6 bg-gradient-to-l from-black/[0.08] to-transparent z-10 transition-opacity duration-150 flex items-center justify-end pr-1 ${
          canScrollRight ? 'opacity-100' : 'opacity-0'
        }`}
      >
        <span className="text-gray-400 text-[10px] font-bold">›</span>
      </div>
      <div ref={scrollRef} className="overflow-x-auto">
        {children}
      </div>
    </div>
  );
}
