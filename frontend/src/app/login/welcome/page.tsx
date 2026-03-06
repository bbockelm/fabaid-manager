'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { api } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';

function getCookie(name: string): string | null {
  const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'));
  return match ? decodeURIComponent(match[2]) : null;
}

function clearCookie(name: string) {
  document.cookie = name + '=; path=/; max-age=0';
}

export default function WelcomePage() {
  const router = useRouter();
  const { session, refetch } = useAuth();
  const [displayName, setDisplayName] = useState(session?.user?.display_name ?? '');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Auto-populate from OIDC-provided name cookie (set during OIDC callback)
  useEffect(() => {
    const oidcName = getCookie('fabaid_oidc_name');
    if (oidcName && !displayName) {
      setDisplayName(oidcName);
      clearCookie('fabaid_oidc_name');
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSave = async () => {
    if (!displayName.trim()) return;
    setSaving(true);
    setError('');
    try {
      await api.auth.updateProfile(displayName.trim());
      refetch();
      router.push('/');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to update profile');
    } finally {
      setSaving(false);
    }
  };

  const handleSkip = () => {
    router.push('/');
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50">
      <div className="w-full max-w-md p-8 space-y-6 bg-white rounded-lg shadow-md">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-gray-900">Welcome!</h1>
          <p className="text-sm text-gray-500 mt-1">
            Set your display name for the Project Tracker
          </p>
        </div>

        {error && (
          <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">{error}</div>
        )}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Display Name
          </label>
          <input
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Your name"
            className="w-full border rounded-md px-3 py-2 text-sm"
            autoFocus
          />
        </div>

        <div className="flex gap-3">
          <button
            onClick={handleSkip}
            className="flex-1 py-2 px-4 border rounded-md text-gray-700 hover:bg-gray-50 font-medium"
          >
            Skip
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !displayName.trim()}
            className="flex-1 py-2 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 font-medium disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Continue'}
          </button>
        </div>
      </div>
    </div>
  );
}
