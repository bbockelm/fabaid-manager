'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, UserInfo, InviteInfo } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';
import { useRouter } from 'next/navigation';

export default function AdminUsersPage() {
  const { isAdmin, isLoading: authLoading } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();

  const [newUserName, setNewUserName] = useState('');
  const [newUserRole, setNewUserRole] = useState('read_only');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [error, setError] = useState('');
  const [expandedUser, setExpandedUser] = useState<string | null>(null);

  const { data: users, isLoading } = useQuery({
    queryKey: ['admin', 'users'],
    queryFn: api.admin.listUsers,
    enabled: isAdmin,
  });

  const createUserMut = useMutation({
    mutationFn: () => api.admin.createUser(newUserName, newUserRole),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'users'] });
      setNewUserName('');
      setNewUserRole('read_only');
      setShowCreateForm(false);
      setError('');
    },
    onError: (e: Error) => setError(e.message),
  });

  const deleteUserMut = useMutation({
    mutationFn: (userId: string) => api.admin.deleteUser(userId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (e: Error) => setError(e.message),
  });

  const addRoleMut = useMutation({
    mutationFn: (vars: { userId: string; role: string }) => api.admin.addRole(vars.userId, vars.role),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (e: Error) => setError(e.message),
  });

  const removeRoleMut = useMutation({
    mutationFn: (vars: { userId: string; role: string }) => api.admin.removeRole(vars.userId, vars.role),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (e: Error) => setError(e.message),
  });

  const removeIdentityMut = useMutation({
    mutationFn: (vars: { userId: string; identityId: string }) =>
      api.admin.removeIdentity(vars.userId, vars.identityId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (e: Error) => setError(e.message),
  });

  const updateStatusMut = useMutation({
    mutationFn: (vars: { userId: string; status: string }) =>
      api.admin.updateUser(vars.userId, { status: vars.status }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['admin', 'users'] }),
    onError: (e: Error) => setError(e.message),
  });

  if (authLoading || isLoading) {
    return <div className="p-6 text-gray-500">Loading...</div>;
  }

  if (!isAdmin) {
    return (
      <div className="p-6">
        <div className="p-4 text-red-700 bg-red-50 rounded-md">
          Access denied. Admin role required.
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">User Management</h1>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          className="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 text-sm font-medium"
        >
          {showCreateForm ? 'Cancel' : '+ New User'}
        </button>
      </div>

      {error && (
        <div className="p-3 text-sm text-red-700 bg-red-50 rounded-md">
          {error}
          <button onClick={() => setError('')} className="ml-2 underline">dismiss</button>
        </div>
      )}

      {showCreateForm && (
        <div className="p-4 bg-gray-50 rounded-md space-y-3 border">
          <h3 className="text-sm font-semibold text-gray-700">Create New User</h3>
          <div className="flex gap-3">
            <input
              type="text"
              value={newUserName}
              onChange={(e) => setNewUserName(e.target.value)}
              placeholder="Display name"
              className="flex-1 border rounded-md px-3 py-2 text-sm"
            />
            <select
              value={newUserRole}
              onChange={(e) => setNewUserRole(e.target.value)}
              className="border rounded-md px-3 py-2 text-sm"
            >
              <option value="admin">Admin</option>
              <option value="grant_admin">Grant Admin</option>
              <option value="read_only">Read Only</option>
            </select>
            <button
              onClick={() => createUserMut.mutate()}
              disabled={!newUserName.trim() || createUserMut.isPending}
              className="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 text-sm font-medium disabled:opacity-50"
            >
              Create
            </button>
          </div>
        </div>
      )}

      <div className="space-y-3">
        {users?.map((user) => (
          <UserCard
            key={user.id}
            user={user}
            isExpanded={expandedUser === user.id}
            onToggle={() => setExpandedUser(expandedUser === user.id ? null : user.id)}
            onDeleteUser={() => {
              if (confirm(`Delete user "${user.display_name}"?`)) {
                deleteUserMut.mutate(user.id);
              }
            }}
            onAddRole={(role) => addRoleMut.mutate({ userId: user.id, role })}
            onRemoveRole={(role) => removeRoleMut.mutate({ userId: user.id, role })}
            onRemoveIdentity={(identityId) =>
              removeIdentityMut.mutate({ userId: user.id, identityId })
            }
            onToggleStatus={() =>
              updateStatusMut.mutate({
                userId: user.id,
                status: user.status === 'active' ? 'disabled' : 'active',
              })
            }
          />
        ))}
        {users?.length === 0 && (
          <div className="text-center py-8 text-gray-400">No users yet.</div>
        )}
      </div>
    </div>
  );
}

function UserCard({
  user,
  isExpanded,
  onToggle,
  onDeleteUser,
  onAddRole,
  onRemoveRole,
  onRemoveIdentity,
  onToggleStatus,
}: {
  user: UserInfo;
  isExpanded: boolean;
  onToggle: () => void;
  onDeleteUser: () => void;
  onAddRole: (role: string) => void;
  onRemoveRole: (role: string) => void;
  onRemoveIdentity: (identityId: string) => void;
  onToggleStatus: () => void;
}) {
  const allRoles = ['admin', 'grant_admin', 'read_only'];
  const userRoles = user.roles ?? [];
  const missingRoles = allRoles.filter((r) => !userRoles.includes(r));

  return (
    <div className="border rounded-lg bg-white">
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-50"
        onClick={onToggle}
      >
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 bg-blue-100 text-blue-600 rounded-full flex items-center justify-center font-semibold text-sm">
            {user.display_name.charAt(0).toUpperCase()}
          </div>
          <div>
            <div className="font-medium text-gray-900">{user.display_name}</div>
            <div className="flex items-center gap-2 mt-0.5">
              {userRoles.map((role) => (
                <span
                  key={role}
                  className={`text-xs px-2 py-0.5 rounded-full ${
                    role === 'admin'
                      ? 'bg-red-100 text-red-700'
                      : role === 'grant_admin'
                      ? 'bg-blue-100 text-blue-700'
                      : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  {role.replace('_', ' ')}
                </span>
              ))}
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <span
            className={`text-xs px-2 py-1 rounded ${
              user.status === 'active'
                ? 'bg-green-100 text-green-700'
                : 'bg-gray-100 text-gray-500'
            }`}
          >
            {user.status}
          </span>
          <span className="text-gray-400">{isExpanded ? '▼' : '▶'}</span>
        </div>
      </div>

      {isExpanded && (
        <div className="border-t px-4 py-3 space-y-4 bg-gray-50">
          {/* Roles */}
          <div>
            <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">Roles</h4>
            <div className="flex flex-wrap gap-2">
              {userRoles.map((role) => (
                <span
                  key={role}
                  className="inline-flex items-center gap-1 text-xs bg-white border rounded-full px-2.5 py-1"
                >
                  {role.replace('_', ' ')}
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onRemoveRole(role);
                    }}
                    className="text-red-400 hover:text-red-600 ml-1"
                    title="Remove role"
                  >
                    ×
                  </button>
                </span>
              ))}
              {missingRoles.length > 0 && (
                <select
                  onChange={(e) => {
                    if (e.target.value) {
                      onAddRole(e.target.value);
                      e.target.value = '';
                    }
                  }}
                  defaultValue=""
                  className="text-xs border rounded-full px-2.5 py-1 bg-white"
                >
                  <option value="">+ Add role...</option>
                  {missingRoles.map((r) => (
                    <option key={r} value={r}>{r.replace('_', ' ')}</option>
                  ))}
                </select>
              )}
            </div>
          </div>

          {/* Identities */}
          <div>
            <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">
              Linked Identities
            </h4>
            {user.identities && user.identities.length > 0 ? (
              <div className="space-y-1">
                {user.identities.map((id) => (
                  <div
                    key={id.id}
                    className="flex items-center justify-between text-xs bg-white border rounded px-3 py-2"
                  >
                    <div>
                      <span className="text-gray-500">{id.issuer}</span>
                      <span className="mx-1 text-gray-300">|</span>
                      <span className="text-gray-700">{id.subject}</span>
                    </div>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        if (confirm('Remove this identity?')) onRemoveIdentity(id.id);
                      }}
                      className="text-red-400 hover:text-red-600"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-xs text-gray-400">No linked identities</div>
            )}
          </div>

          {/* Invites */}
          <InviteSection userId={user.id} />

          {/* Info & Actions */}
          <div className="flex items-center justify-between pt-2 border-t">
            <div className="text-xs text-gray-400">
              Last login: {user.last_login ? new Date(user.last_login).toLocaleString() : 'Never'}
              {' · '}Created: {new Date(user.created_at).toLocaleDateString()}
            </div>
            <div className="flex gap-2">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onToggleStatus();
                }}
                className="text-xs px-3 py-1 rounded border hover:bg-gray-100"
              >
                {user.status === 'active' ? 'Disable' : 'Enable'}
              </button>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onDeleteUser();
                }}
                className="text-xs px-3 py-1 rounded border border-red-200 text-red-600 hover:bg-red-50"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function InviteSection({ userId }: { userId: string }) {
  const queryClient = useQueryClient();
  const [generatedUrl, setGeneratedUrl] = useState('');

  const { data: invites } = useQuery({
    queryKey: ['admin', 'invites', userId],
    queryFn: () => api.admin.listInvites(userId),
  });

  const createInviteMut = useMutation({
    mutationFn: () => api.admin.createInvite(userId),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'invites', userId] });
      setGeneratedUrl(data.invite_url);
    },
  });

  const deleteInviteMut = useMutation({
    mutationFn: (inviteId: string) => api.admin.deleteInvite(userId, inviteId),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ['admin', 'invites', userId] }),
  });

  return (
    <div>
      <h4 className="text-xs font-semibold text-gray-500 uppercase mb-2">Invite Links</h4>

      {invites && invites.length > 0 && (
        <div className="space-y-1 mb-2">
          {invites.map((inv) => (
            <div
              key={inv.id}
              className="flex items-center justify-between text-xs bg-white border rounded px-3 py-2"
            >
              <div>
                <span className={`${inv.used ? 'text-gray-400 line-through' : 'text-gray-700'}`}>
                  {inv.role.replace('_', ' ')}
                </span>
                <span className="text-gray-400 ml-2">
                  expires {new Date(inv.expires_at).toLocaleDateString()}
                </span>
                {inv.used && <span className="ml-2 text-green-600">✓ used</span>}
              </div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  deleteInviteMut.mutate(inv.id);
                }}
                className="text-red-400 hover:text-red-600"
              >
                ×
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-2 items-center">
        <button
          onClick={() => createInviteMut.mutate()}
          disabled={createInviteMut.isPending}
          className="text-xs px-3 py-1 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
        >
          Generate Invite
        </button>
      </div>

      {generatedUrl && (
        <div className="mt-2 p-2 bg-green-50 border border-green-200 rounded text-xs">
          <div className="font-medium text-green-800 mb-1">Invite link generated:</div>
          <div className="flex items-center gap-2">
            <input
              type="text"
              readOnly
              value={generatedUrl}
              className="flex-1 bg-white border rounded px-2 py-1 text-xs"
            />
            <button
              onClick={() => {
                navigator.clipboard.writeText(generatedUrl);
              }}
              className="px-2 py-1 bg-white border rounded hover:bg-gray-50 text-xs"
            >
              Copy
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
