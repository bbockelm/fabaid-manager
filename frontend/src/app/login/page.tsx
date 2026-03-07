'use client';

import { Suspense, useState, useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { api, AuthMode } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';

const ERROR_MESSAGES: Record<string, string> = {
  no_account: 'No account found for this identity. Please contact an administrator for an invite link.',
  invalid_invite: 'This invite link is invalid or has expired. Please request a new one from an administrator.',
  identity_already_linked: 'This identity is already linked to another account. If you believe this is an error, contact an administrator.',
};

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="flex items-center justify-center min-h-screen bg-gray-50"><div className="text-gray-500">Loading...</div></div>}>
      <LoginContent />
    </Suspense>
  );
}

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { session, isAuthenticated, isLoading: authLoading, refetch, logout } = useAuth();
  const isDisabled = !!session?.user && session.user.status !== 'active';
  const [mode, setMode] = useState<AuthMode | null>(null);
  const [loading, setLoading] = useState(true);
  const [devName, setDevName] = useState('Developer');
  const [devRole, setDevRole] = useState('admin');
  const [error, setError] = useState('');

  useEffect(() => {
    const errCode = searchParams.get('error');
    if (errCode) {
      setError(ERROR_MESSAGES[errCode] || `Login error: ${errCode}`);
    }
  }, [searchParams]);

  useEffect(() => {
    api.auth.mode().then(setMode).catch(() => setMode({ mode: 'dev', oidc_configured: false, callback_url: '' })).finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!authLoading && isAuthenticated) {
      router.push('/');
    }
  }, [authLoading, isAuthenticated, router]);

  const handleDevLogin = async () => {
    setError('');
    try {
      await api.auth.devLogin(devName, devRole);
      refetch();
      router.push('/');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Login failed');
    }
  };

  const handleOIDCLogin = () => {
    window.location.href = api.auth.oidcLoginUrl();
  };

  if (loading || authLoading) {
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
          <h1 className="text-2xl font-bold text-gray-900">Project Tracker</h1>
          <p className="text-sm text-gray-500 mt-1">FabAID Manager — Sign In</p>
        </div>

        {error && (
          <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">{error}</div>
        )}

        {isDisabled ? (
          <div className="space-y-4">
            <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">
              Your account has been disabled. Please contact an administrator.
            </div>
            <button
              onClick={() => logout()}
              className="w-full py-2 px-4 bg-gray-600 text-white rounded-md hover:bg-gray-700 font-medium"
            >
              Sign Out
            </button>
          </div>
        ) : mode?.mode === 'dev' ? (
          <div className="space-y-4">
            <div className="p-3 text-sm text-amber-700 bg-amber-50 rounded-md">
              ⚠️ Development mode — no OIDC required
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Display Name
              </label>
              <input
                type="text"
                value={devName}
                onChange={(e) => setDevName(e.target.value)}
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Role
              </label>
              <div className="grid grid-cols-3 gap-2">
                {['admin', 'grant_admin', 'read_only'].map((role) => (
                  <button
                    key={role}
                    onClick={() => setDevRole(role)}
                    className={`px-3 py-2 text-sm rounded-md border transition-colors ${
                      devRole === role
                        ? 'bg-blue-600 text-white border-blue-600'
                        : 'bg-white text-gray-700 border-gray-300 hover:bg-gray-50'
                    }`}
                  >
                    {role.replace('_', ' ')}
                  </button>
                ))}
              </div>
            </div>
            <button
              onClick={handleDevLogin}
              className="w-full py-2 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 font-medium"
            >
              Sign In as {devRole.replace('_', ' ')}
            </button>

            {mode.oidc_configured && (
              <div className="pt-4 border-t">
                <button
                  onClick={handleOIDCLogin}
                  className="w-full py-2 px-4 bg-gray-600 text-white rounded-md hover:bg-gray-700 font-medium"
                >
                  Sign In with OIDC
                </button>
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            {mode?.oidc_configured ? (
              <button
                onClick={handleOIDCLogin}
                className="w-full py-2 px-4 bg-blue-600 text-white rounded-md hover:bg-blue-700 font-medium"
              >
                Sign In with OIDC
              </button>
            ) : (
              <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">
                OIDC is not configured. An administrator must configure OIDC settings before you can sign in.
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
