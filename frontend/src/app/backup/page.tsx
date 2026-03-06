'use client';

import { useState, useRef } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, BackupRecord } from '@/lib/api';
import { useAuth } from '@/lib/auth-context';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

function timeAgo(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    completed: 'bg-green-100 text-green-800',
    running: 'bg-blue-100 text-blue-800',
    failed: 'bg-red-100 text-red-800',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${colors[status] || 'bg-gray-100 text-gray-800'}`}>
      {status === 'running' && <span className="animate-pulse mr-1">●</span>}
      {status}
    </span>
  );
}

export default function BackupPage() {
  const { isAdmin } = useAuth();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [restoring, setRestoring] = useState<string | null>(null);
  const [uploadMsg, setUploadMsg] = useState<string | null>(null);
  const [justTriggered, setJustTriggered] = useState(false);

  const { data: backups, isLoading, error } = useQuery({
    queryKey: ['backups'],
    queryFn: api.backup.list,
    retry: false,
    // Poll every 2s when a backup is running or was just triggered; 10s otherwise.
    refetchInterval: (query) => {
      if (query.state.error) return false; // stop polling on error
      const data = query.state.data as BackupRecord[] | undefined;
      const hasRunning = data?.some(b => b.status === 'running');
      return (hasRunning || justTriggered) ? 2000 : 10000;
    },
  });

  const triggerMutation = useMutation({
    mutationFn: api.backup.trigger,
    onSuccess: () => {
      setJustTriggered(true);
      queryClient.invalidateQueries({ queryKey: ['backups'] });
      // Stop fast-polling after 60s even if something is stuck.
      setTimeout(() => setJustTriggered(false), 60000);
    },
  });

  // Clear justTriggered once the running backup finishes.
  const hasRunning = backups?.some(b => b.status === 'running');
  if (justTriggered && backups && !hasRunning) {
    // The backup finished (or failed) — stop fast-polling on next render.
    setTimeout(() => setJustTriggered(false), 0);
  }

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.backup.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] });
    },
  });

  const restoreMutation = useMutation({
    mutationFn: (id: string) => api.backup.restore(id),
    onSuccess: () => {
      setRestoring(null);
      alert('Restore completed successfully. The page will reload.');
      window.location.reload();
    },
    onError: (err: Error) => {
      setRestoring(null);
      alert(`Restore failed: ${err.message}`);
    },
  });

  const handleRestore = (backup: BackupRecord) => {
    if (!confirm(`Are you sure you want to restore from "${backup.filename}"?\n\nThis will overwrite the current database and documents.`)) return;
    setRestoring(backup.id);
    restoreMutation.mutate(backup.id);
  };

  const handleUploadRestore = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!confirm(`Restore from uploaded file "${file.name}"?\n\nThis will overwrite the current database and documents.`)) {
      e.target.value = '';
      return;
    }
    setUploadMsg('Uploading and restoring...');
    try {
      await api.backup.uploadRestore(file);
      setUploadMsg('Restore completed! Reloading...');
      setTimeout(() => window.location.reload(), 1500);
    } catch (err: unknown) {
      setUploadMsg(`Restore failed: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }
    e.target.value = '';
  };

  const lastCompleted = backups?.find(b => b.status === 'completed');

  if (isLoading) return <div className="p-4">Loading backups...</div>;
  if (error) return (
    <div className="p-4">
      <h1 className="text-2xl font-bold text-nsf-blue mb-4">System Backups</h1>
      <div className="bg-red-50 border border-red-200 text-red-700 p-4 rounded-md text-sm">
        Failed to load backups: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    </div>
  );

  return (
    <div className="max-w-5xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-nsf-blue">System Backups</h1>
          <p className="text-sm text-gray-500 mt-1">
            Encrypted backups of the database and all documents.
          </p>
        </div>
        {isAdmin && (
          <div className="flex gap-2">
            <button
              onClick={() => triggerMutation.mutate()}
              disabled={triggerMutation.isPending || hasRunning}
              className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50 text-sm font-medium"
            >
              {triggerMutation.isPending ? 'Starting...' : hasRunning ? 'Backup Running...' : '📦 Create Backup'}
            </button>
            <button
              onClick={() => fileInputRef.current?.click()}
              className="px-4 py-2 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors text-sm font-medium"
            >
              📤 Upload &amp; Restore
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept=".tar.gz,.tar.gz.enc,.enc"
              className="hidden"
              onChange={handleUploadRestore}
            />
          </div>
        )}
      </div>

      {uploadMsg && (
        <div className={`p-3 rounded-md text-sm ${uploadMsg.includes('failed') ? 'bg-red-50 text-red-700' : 'bg-blue-50 text-blue-700'}`}>
          {uploadMsg}
        </div>
      )}

      {/* Time since last backup */}
      <div className="bg-white p-4 rounded-lg border border-gray-200">
        <div className="flex items-center gap-4">
          <div className="text-sm text-gray-600">
            <span className="font-medium">Last successful backup:</span>{' '}
            {lastCompleted ? (
              <>
                <span className="text-green-700 font-medium">{timeAgo(lastCompleted.completed_at!)}</span>
                <span className="text-gray-400 ml-2">
                  ({new Date(lastCompleted.completed_at!).toLocaleString()})
                </span>
              </>
            ) : (
              <span className="text-amber-600">No backups yet</span>
            )}
          </div>
          {hasRunning && (
            <span className="inline-flex items-center text-sm text-blue-600">
              <span className="animate-spin mr-1">⏳</span> Backup in progress...
            </span>
          )}
        </div>
      </div>

      {/* Backup table */}
      <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Filename</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Size</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Type</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Started</th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duration</th>
              <th className="px-4 py-3 text-right text-xs font-medium text-gray-500 uppercase">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-200">
            {(!backups || backups.length === 0) && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400 text-sm">
                  No backups yet. Click &quot;Create Backup&quot; to get started.
                </td>
              </tr>
            )}
            {backups?.map((b) => {
              const duration =
                b.completed_at && b.started_at
                  ? `${Math.round((new Date(b.completed_at).getTime() - new Date(b.started_at).getTime()) / 1000)}s`
                  : b.status === 'running' ? '...' : '—';

              return (
                <tr key={b.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm">
                    <div className="font-medium text-gray-900">{b.filename}</div>
                    {b.checksum && (
                      <div className="text-xs text-gray-400 font-mono truncate max-w-[200px]" title={b.checksum}>
                        SHA-256: {b.checksum.slice(0, 16)}...
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={b.status} />
                    {b.status === 'running' && b.status_detail && (
                      <div className="text-xs text-blue-600 mt-1 flex items-center gap-1">
                        <span className="animate-pulse">●</span> {b.status_detail}
                      </div>
                    )}
                    {b.error_msg && (
                      <div className="text-xs text-red-500 mt-1 max-w-[200px] truncate" title={b.error_msg}>
                        {b.error_msg}
                      </div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">
                    {b.size_bytes > 0 ? formatBytes(b.size_bytes) : '—'}
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">
                    <span className="inline-flex items-center gap-1">
                      {b.encrypted ? '🔒' : '🔓'}
                      <span className="text-xs">{b.initiated_by}</span>
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">
                    <div>{timeAgo(b.started_at)}</div>
                    <div className="text-xs text-gray-400">{new Date(b.started_at).toLocaleString()}</div>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-600">{duration}</td>
                  <td className="px-4 py-3 text-right">
                    {b.status === 'completed' && (
                      <div className="flex items-center justify-end gap-1">
                        <a
                          href={api.backup.downloadUrl(b.id)}
                          className="px-2 py-1 text-xs bg-gray-100 hover:bg-gray-200 rounded transition-colors"
                          title="Download backup"
                        >
                          ⬇️
                        </a>
                        {isAdmin && (
                          <>
                            <button
                              onClick={() => handleRestore(b)}
                              disabled={restoring === b.id}
                              className="px-2 py-1 text-xs bg-amber-100 hover:bg-amber-200 rounded transition-colors disabled:opacity-50"
                              title="Restore from this backup"
                            >
                              {restoring === b.id ? '...' : '🔄'}
                            </button>
                            <button
                              onClick={() => {
                                if (confirm('Delete this backup?')) deleteMutation.mutate(b.id);
                              }}
                              className="px-2 py-1 text-xs bg-red-100 hover:bg-red-200 rounded transition-colors"
                              title="Delete backup"
                            >
                              🗑️
                            </button>
                          </>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
