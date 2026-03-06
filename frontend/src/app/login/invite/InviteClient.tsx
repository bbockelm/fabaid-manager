'use client';

import { useEffect, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { api, AuthMode } from '@/lib/api';

export default function InviteClient() {
  const searchParams = useSearchParams();
  const token = searchParams.get('token');
  const [mode, setMode] = useState<AuthMode | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.auth.mode().then(setMode).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const handleAcceptOIDC = () => {
    if (token) {
      window.location.href = api.auth.oidcLoginUrl(token);
    }
  };

  if (!token) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50">
        <div className="w-full max-w-md p-8 bg-white rounded-lg shadow-md text-center">
          <h1 className="text-xl font-bold text-red-600">Invalid Invite</h1>
          <p className="text-sm text-gray-500 mt-2">No invite token provided.</p>
        </div>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-gray-50">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50">
      <div className="w-full max-w-md p-8 space-y-6 bg-white rounded-lg shadow-md">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-gray-900">You&apos;re Invited!</h1>
          <p className="text-sm text-gray-500 mt-1">
            You&apos;ve been invited to join the FabAID Project Tracker.
          </p>
        </div>

        {error && (
          <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">{error}</div>
        )}

        {mode?.oidc_configured ? (
          <button
            onClick={handleAcceptOIDC}
            className="w-full py-2 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 font-medium"
          >
            Sign In with OIDC to Accept Invite
          </button>
        ) : mode?.mode === 'dev' ? (
          <div className="text-center">
            <p className="text-sm text-amber-700 bg-amber-50 rounded-md p-3">
              Development mode — invite links redirect through OIDC.
              Configure OIDC or use dev login instead.
            </p>
            <a
              href="/login"
              className="mt-4 inline-block text-blue-600 hover:underline text-sm"
            >
              Go to Login
            </a>
          </div>
        ) : (
          <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">
            OIDC is not configured. Contact an administrator.
          </div>
        )}
      </div>
    </div>
  );
}
