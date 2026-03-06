'use client';

import { usePathname } from 'next/navigation';
import { Sidebar } from '@/components/Sidebar';

/** Wraps pages with sidebar, except on /login routes which are full-screen. */
export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isLoginPage = pathname.startsWith('/login');

  if (isLoginPage) {
    return <>{children}</>;
  }

  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto p-6">{children}</main>
    </div>
  );
}
