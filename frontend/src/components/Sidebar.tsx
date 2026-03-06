'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useAuth } from '@/lib/auth-context';

const navItems = [
  { label: 'Dashboard', href: '/', icon: '📊' },
  { label: 'WBS Areas', href: '/wbs', icon: '🗂️' },
  { label: 'Personnel', href: '/personnel', icon: '👥' },
  { label: 'Institutions', href: '/institutions', icon: '🏛️' },
  { label: 'Budget', href: '/budget', icon: '💰' },
  { label: 'Documents', href: '/documents', icon: '📄' },
  { label: 'Settings', href: '/settings', icon: '⚙️' },
  { label: 'Backup', href: '/backup', icon: '💾' },
];

const adminNavItems = [
  { label: 'Users', href: '/admin/users', icon: '🔑' },
  { label: 'API Keys', href: '/admin/api-keys', icon: '🔐' },
];

export function Sidebar() {
  const pathname = usePathname();
  const { session, isAuthenticated, isAdmin, logout } = useAuth();

  const roleLabel = session?.role?.replace('_', ' ') ?? '';
  const roleBadgeColor =
    session?.role === 'admin'
      ? 'bg-red-400/30 text-red-100'
      : session?.role === 'grant_admin'
      ? 'bg-blue-400/30 text-blue-100'
      : 'bg-gray-400/30 text-gray-200';

  return (
    <aside className="w-64 bg-nsf-blue text-white flex flex-col">
      <div className="p-6 border-b border-white/10">
        <h1 className="text-xl font-bold">Project Tracker</h1>
        <p className="text-xs text-blue-200 mt-1">FabAID</p>
      </div>
      <nav className="flex-1 p-4 space-y-1">
        {navItems.map((item) => {
          const isActive =
            pathname === item.href ||
            (item.href !== '/' && pathname.startsWith(item.href));
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                isActive
                  ? 'bg-white/20 text-white'
                  : 'text-blue-100 hover:bg-white/10 hover:text-white'
              }`}
            >
              <span>{item.icon}</span>
              <span>{item.label}</span>
            </Link>
          );
        })}

        {isAdmin && (
          <>
            <div className="pt-4 pb-1">
              <span className="text-xs font-semibold text-blue-300 uppercase tracking-wider px-3">
                Admin
              </span>
            </div>
            {adminNavItems.map((item) => {
              const isActive = pathname.startsWith(item.href);
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                    isActive
                      ? 'bg-white/20 text-white'
                      : 'text-blue-100 hover:bg-white/10 hover:text-white'
                  }`}
                >
                  <span>{item.icon}</span>
                  <span>{item.label}</span>
                </Link>
              );
            })}
          </>
        )}
      </nav>

      <div className="p-4 border-t border-white/10">
        {isAuthenticated ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <div className="truncate">
                <div className="text-sm font-medium truncate">
                  {session?.user?.display_name ?? 'User'}
                </div>
                <span className={`text-xs px-1.5 py-0.5 rounded ${roleBadgeColor}`}>
                  {roleLabel}
                </span>
              </div>
            </div>
            <button
              onClick={logout}
              className="w-full text-xs text-blue-200 hover:text-white hover:bg-white/10 rounded px-2 py-1 transition-colors text-left"
            >
              Sign Out
            </button>
          </div>
        ) : (
          <Link
            href="/login"
            className="block text-sm text-blue-200 hover:text-white transition-colors"
          >
            Sign In
          </Link>
        )}
        <div className="text-xs text-blue-300/50 mt-2">v0.1.0</div>
      </div>
    </aside>
  );
}
