'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, APIKeyInfo, APIKeyCreateResponse } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';

const ROLES = [
  { value: 'admin', label: 'Admin', color: 'bg-red-100 text-red-800' },
  { value: 'grant_admin', label: 'Grant Admin', color: 'bg-blue-100 text-blue-800' },
  { value: 'read_only', label: 'Read Only', color: 'bg-gray-100 text-gray-800' },
];

const IDLE_OPTIONS = [
  { value: '', label: 'No idle timeout' },
  { value: '3600', label: '1 hour' },
  { value: '86400', label: '1 day' },
  { value: '604800', label: '7 days' },
  { value: '2592000', label: '30 days' },
  { value: '7776000', label: '90 days' },
];

export default function AdminAPIKeysPage() {
  const { isAdmin, isLoading: authLoading } = useAuth();
  const queryClient = useQueryClient();

  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newRoles, setNewRoles] = useState<string[]>(['read_only']);
  const [idleTimeout, setIdleTimeout] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [error, setError] = useState('');
  const [createdKey, setCreatedKey] = useState<APIKeyCreateResponse | null>(null);
  const [copied, setCopied] = useState(false);

  const { data: keys, isLoading } = useQuery({
    queryKey: ['admin', 'api-keys'],
    queryFn: api.admin.listAPIKeys,
    enabled: isAdmin,
  });

  const createMut = useMutation({
    mutationFn: () =>
      api.admin.createAPIKey({
        name: newName,
        roles: newRoles,
        idle_timeout_s: idleTimeout ? parseInt(idleTimeout) : undefined,
        expires_at: expiresAt ? new Date(expiresAt).toISOString() : undefined,
      }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'api-keys'] });
      setCreatedKey(data);
      setNewName('');
      setNewRoles(['read_only']);
      setIdleTimeout('');
      setExpiresAt('');
      setShowCreate(false);
      setError('');
    },
    onError: (e: Error) => setError(e.message),
  });

  const revokeMut = useMutation({
    mutationFn: (id: string) => api.admin.revokeAPIKey(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'api-keys'] }),
    onError: (e: Error) => setError(e.message),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => api.admin.deleteAPIKey(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'api-keys'] }),
    onError: (e: Error) => setError(e.message),
  });

  const toggleRole = (role: string) => {
    setNewRoles((prev) =>
      prev.includes(role) ? prev.filter((r) => r !== role) : [...prev, role]
    );
  };

  const copyKey = async (key: string) => {
    await navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (authLoading || isLoading) return <div className="p-6 text-gray-500">Loading...</div>;
  if (!isAdmin) {
    return (
      <div className="p-6">
        <div className="p-4 text-red-700 bg-red-50 rounded-md">Access denied. Admin role required.</div>
      </div>
    );
  }

  const activeKeys = (keys ?? []).filter((k) => !k.revoked_at);
  const revokedKeys = (keys ?? []).filter((k) => k.revoked_at);

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">API Keys</h1>
          <p className="text-sm text-gray-500 mt-1">
            Manage programmatic access keys. Keys use the <code className="text-xs bg-gray-100 px-1 rounded">fabaid_</code> prefix for easy detection.
          </p>
        </div>
        <button
          onClick={() => { setShowCreate(!showCreate); setCreatedKey(null); }}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 text-sm font-medium"
        >
          {showCreate ? 'Cancel' : '+ New API Key'}
        </button>
      </div>

      {error && (
        <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">
          {error}
          <button onClick={() => setError('')} className="ml-2 underline">dismiss</button>
        </div>
      )}

      {/* --- Newly created key banner (shown only once) --- */}
      {createdKey && (
        <div className="p-4 bg-green-50 border border-green-200 rounded-lg space-y-2">
          <p className="font-semibold text-green-800">
            API key &ldquo;{createdKey.name}&rdquo; created successfully!
          </p>
          <p className="text-sm text-green-700">
            Copy the key now &mdash; it will <strong>not</strong> be shown again.
          </p>
          <div className="flex items-center gap-2">
            <code className="flex-1 p-2 bg-white border rounded text-sm font-mono break-all select-all">
              {createdKey.raw_key}
            </code>
            <button
              onClick={() => copyKey(createdKey.raw_key)}
              className="px-3 py-2 bg-green-600 text-white rounded text-sm hover:bg-green-700 shrink-0"
            >
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
          <button
            onClick={() => setCreatedKey(null)}
            className="text-xs text-green-600 hover:underline mt-1"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* --- Create form --- */}
      {showCreate && (
        <div className="bg-white p-6 rounded-lg border space-y-4">
          <h2 className="font-semibold text-gray-800">Create API Key</h2>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name *</label>
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="e.g. CI/CD Pipeline"
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Idle Timeout</label>
              <select
                value={idleTimeout}
                onChange={(e) => setIdleTimeout(e.target.value)}
                className="w-full border rounded-md px-3 py-2 text-sm"
              >
                {IDLE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Roles *</label>
            <div className="flex gap-3">
              {ROLES.map((role) => (
                <label key={role.value} className="flex items-center gap-1.5 text-sm">
                  <input
                    type="checkbox"
                    checked={newRoles.includes(role.value)}
                    onChange={() => toggleRole(role.value)}
                    className="rounded"
                  />
                  {role.label}
                </label>
              ))}
            </div>
          </div>

          <div className="max-w-xs">
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Expiration Date <span className="text-gray-400">(optional)</span>
            </label>
            <input
              type="date"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              className="w-full border rounded-md px-3 py-2 text-sm"
            />
          </div>

          <button
            onClick={() => createMut.mutate()}
            disabled={!newName || newRoles.length === 0 || createMut.isPending}
            className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 text-sm font-medium"
          >
            {createMut.isPending ? 'Creating...' : 'Create Key'}
          </button>
        </div>
      )}

      {/* --- Active keys table --- */}
      {activeKeys.length > 0 && (
        <KeyTable
          title="Active Keys"
          keys={activeKeys}
          onRevoke={(id) => {
            if (confirm('Revoke this API key? It will immediately stop working.'))
              revokeMut.mutate(id);
          }}
        />
      )}
      {activeKeys.length === 0 && !showCreate && !createdKey && (
        <div className="bg-white rounded-lg border p-8 text-center text-gray-400 text-sm">
          No API keys yet. Click &ldquo;+ New API Key&rdquo; to create one.
        </div>
      )}

      {/* --- Revoked keys --- */}
      {revokedKeys.length > 0 && (
        <div>
          <KeyTable
            title="Revoked Keys"
            keys={revokedKeys}
            isRevoked
            onDelete={(id) => {
              if (confirm('Permanently delete this revoked key? This cannot be undone.'))
                deleteMut.mutate(id);
            }}
          />
        </div>
      )}
    </div>
  );
}

function KeyTable({
  title,
  keys,
  isRevoked = false,
  onRevoke,
  onDelete,
}: {
  title: string;
  keys: APIKeyInfo[];
  isRevoked?: boolean;
  onRevoke?: (id: string) => void;
  onDelete?: (id: string) => void;
}) {
  const formatDate = (d?: string) => (d ? new Date(d).toLocaleDateString() : '\u2014');
  const formatDateTime = (d?: string) => (d ? new Date(d).toLocaleString() : 'Never');
  const formatIdle = (s?: number) => {
    if (!s) return '\u2014';
    if (s < 3600) return `${Math.round(s / 60)}m`;
    if (s < 86400) return `${Math.round(s / 3600)}h`;
    return `${Math.round(s / 86400)}d`;
  };

  const isExpired = (k: APIKeyInfo) => {
    if (k.expires_at && new Date(k.expires_at) < new Date()) return true;
    if (k.idle_timeout_s && k.last_used_at) {
      const deadline = new Date(k.last_used_at).getTime() + k.idle_timeout_s * 1000;
      if (Date.now() > deadline) return true;
    }
    return false;
  };

  return (
    <div className={`bg-white rounded-lg border overflow-hidden ${isRevoked ? 'opacity-60' : ''}`}>
      <div className="px-4 py-3 bg-gray-50 border-b">
        <h3 className="text-sm font-semibold text-gray-700">{title}</h3>
      </div>
      <table className="w-full">
        <thead className="border-b">
          <tr>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Name</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Prefix</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Roles</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Created</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Last Used</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Idle</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Expires</th>
            <th className="text-left px-4 py-2 text-xs font-medium text-gray-500 uppercase">Status</th>
            <th className="px-2 py-2 w-20"></th>
          </tr>
        </thead>
        <tbody className="divide-y">
          {keys.map((k) => {
            const expired = isExpired(k);
            return (
              <tr key={k.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 text-sm font-medium">{k.name}</td>
                <td className="px-4 py-3 text-sm">
                  <code className="text-xs bg-gray-100 px-1.5 py-0.5 rounded font-mono">{k.key_prefix}...</code>
                </td>
                <td className="px-4 py-3 text-sm">
                  <div className="flex flex-wrap gap-1">
                    {k.roles.map((role) => {
                      const rd = ROLES.find((r) => r.value === role);
                      return (
                        <span key={role} className={`inline-block px-1.5 py-0.5 rounded text-xs font-medium ${rd?.color ?? 'bg-gray-100 text-gray-800'}`}>
                          {rd?.label ?? role}
                        </span>
                      );
                    })}
                  </div>
                </td>
                <td className="px-4 py-3 text-sm text-gray-500">
                  <div>{formatDate(k.created_at)}</div>
                  {k.created_by_name && <div className="text-xs text-gray-400">{k.created_by_name}</div>}
                </td>
                <td className="px-4 py-3 text-sm text-gray-500">{formatDateTime(k.last_used_at)}</td>
                <td className="px-4 py-3 text-sm text-gray-500">{formatIdle(k.idle_timeout_s)}</td>
                <td className="px-4 py-3 text-sm text-gray-500">{formatDate(k.expires_at)}</td>
                <td className="px-4 py-3 text-sm">
                  {k.revoked_at ? (
                    <span className="text-xs font-medium text-red-600">Revoked</span>
                  ) : expired ? (
                    <span className="text-xs font-medium text-orange-600">Expired</span>
                  ) : (
                    <span className="text-xs font-medium text-green-600">Active</span>
                  )}
                </td>
                <td className="px-2 py-3">
                  {!isRevoked && onRevoke && (
                    <button
                      onClick={() => onRevoke(k.id)}
                      className="text-red-500 hover:text-red-700 text-xs"
                    >
                      Revoke
                    </button>
                  )}
                  {isRevoked && onDelete && (
                    <button
                      onClick={() => onDelete(k.id)}
                      className="text-red-500 hover:text-red-700 text-xs"
                    >
                      Delete
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
