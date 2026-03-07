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

/** Modal overlay to display a key with copy button */
function KeyModal({ title, keyHex, onClose }: { title: string; keyHex: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async () => {
    await navigator.clipboard.writeText(keyHex);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div className="bg-white rounded-lg shadow-xl max-w-lg w-full mx-4 p-6" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
        <p className="text-xs text-gray-500 mb-3">
          This key can be shared to allow decryption. Keep it safe.
        </p>
        <div className="bg-gray-50 border rounded-md p-3 font-mono text-sm break-all select-all text-gray-700">
          {keyHex}
        </div>
        <div className="flex justify-end gap-2 mt-4">
          <button
            onClick={handleCopy}
            className="px-4 py-2 text-sm bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors"
          >
            {copied ? '✓ Copied!' : '📋 Copy to Clipboard'}
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

/** Upload & Restore modal with optional decryption key input */
function UploadRestoreModal({ onClose }: { onClose: () => void }) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [decryptKey, setDecryptKey] = useState('');
  const [encrypted, setEncrypted] = useState(true);
  const [status, setStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!file) return;
    if (!confirm(`Restore from "${file.name}"?\n\nThis will overwrite the current database and documents.`)) return;
    setStatus('Uploading and restoring...');
    setError(null);
    try {
      await api.backup.uploadRestore(file, encrypted, decryptKey || undefined);
      setStatus('Restore completed! Reloading...');
      setTimeout(() => window.location.reload(), 1500);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      setStatus(null);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={onClose}>
      <div className="bg-white rounded-lg shadow-xl max-w-lg w-full mx-4 p-6" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-semibold text-gray-900 mb-4">Upload &amp; Restore Backup</h3>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Backup File</label>
            <input
              ref={fileInputRef}
              type="file"
              accept=".tar.gz,.tar.gz.enc,.enc"
              onChange={e => setFile(e.target.files?.[0] || null)}
              className="block w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:text-sm file:font-medium file:bg-nsf-light file:text-white hover:file:bg-nsf-blue"
            />
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="encrypted-cb"
              checked={encrypted}
              onChange={e => setEncrypted(e.target.checked)}
              className="rounded border-gray-300"
            />
            <label htmlFor="encrypted-cb" className="text-sm text-gray-700">File is encrypted</label>
          </div>

          {encrypted && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Decryption Key <span className="text-gray-400 font-normal">(per-backup or general backup key, hex)</span>
              </label>
              <input
                type="text"
                value={decryptKey}
                onChange={e => setDecryptKey(e.target.value.trim())}
                placeholder="Leave empty if this server has the master key"
                className="w-full px-3 py-2 border border-gray-300 rounded-md text-sm font-mono placeholder:font-sans focus:outline-none focus:ring-2 focus:ring-nsf-light"
              />
              <p className="text-xs text-gray-400 mt-1">
                If restoring on the same server that created the backup, you can leave this empty.
                If restoring on a different server, provide the per-backup key or general backup key.
              </p>
            </div>
          )}

          {error && (
            <div className="bg-red-50 border border-red-200 text-red-700 p-3 rounded-md text-sm">{error}</div>
          )}
          {status && (
            <div className="bg-blue-50 border border-blue-200 text-blue-700 p-3 rounded-md text-sm">{status}</div>
          )}
        </div>

        <div className="flex justify-end gap-2 mt-6">
          <button
            onClick={handleSubmit}
            disabled={!file || !!status}
            className="px-4 py-2 text-sm bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50"
          >
            🔄 Restore
          </button>
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}

export default function BackupPage() {
  const { isAdmin } = useAuth();
  const queryClient = useQueryClient();
  const [restoring, setRestoring] = useState<string | null>(null);
  const [justTriggered, setJustTriggered] = useState(false);
  const [keyModal, setKeyModal] = useState<{ title: string; keyHex: string } | null>(null);
  const [showUploadModal, setShowUploadModal] = useState(false);

  const { data: backups, isLoading, error } = useQuery({
    queryKey: ['backups'],
    queryFn: api.backup.list,
    retry: false,
    refetchInterval: (query) => {
      if (query.state.error) return false;
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
      setTimeout(() => setJustTriggered(false), 60000);
    },
  });

  const hasRunning = backups?.some(b => b.status === 'running');
  if (justTriggered && backups && !hasRunning) {
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

  const handleShowPerBackupKey = async (backup: BackupRecord) => {
    try {
      const resp = await api.backup.getPerBackupKey(backup.id);
      setKeyModal({ title: `Decryption Key — ${backup.filename}`, keyHex: resp.key });
    } catch (err: unknown) {
      alert(`Failed to get key: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }
  };

  const handleShowGeneralKey = async () => {
    try {
      const resp = await api.backup.getGeneralBackupKey();
      setKeyModal({ title: 'General Backup Decryption Key', keyHex: resp.key });
    } catch (err: unknown) {
      alert(`Failed to get key: ${err instanceof Error ? err.message : 'Unknown error'}`);
    }
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
      {keyModal && <KeyModal title={keyModal.title} keyHex={keyModal.keyHex} onClose={() => setKeyModal(null)} />}
      {showUploadModal && <UploadRestoreModal onClose={() => setShowUploadModal(false)} />}

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
              onClick={() => setShowUploadModal(true)}
              className="px-4 py-2 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors text-sm font-medium"
            >
              📤 Upload &amp; Restore
            </button>
            <button
              onClick={handleShowGeneralKey}
              className="px-4 py-2 border border-amber-300 bg-amber-50 rounded-md hover:bg-amber-100 transition-colors text-sm font-medium text-amber-800"
              title="Show the general backup decryption key (can decrypt any backup)"
            >
              🔑 General Key
            </button>
          </div>
        )}
      </div>

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
                    <div className="flex items-center justify-end gap-1">
                      {b.status === 'completed' && b.encrypted && isAdmin && (
                        <button
                          onClick={() => handleShowPerBackupKey(b)}
                          className="px-2 py-1 text-xs bg-amber-50 hover:bg-amber-100 border border-amber-200 rounded transition-colors"
                          title="Show per-backup decryption key"
                        >
                          🔑
                        </button>
                      )}
                      {b.status === 'completed' && (
                        <a
                          href={api.backup.downloadUrl(b.id)}
                          className="px-2 py-1 text-xs bg-gray-100 hover:bg-gray-200 rounded transition-colors"
                          title="Download backup"
                        >
                          ⬇️
                        </a>
                      )}
                      {b.status === 'completed' && isAdmin && (
                        <button
                          onClick={() => handleRestore(b)}
                          disabled={restoring === b.id}
                          className="px-2 py-1 text-xs bg-amber-100 hover:bg-amber-200 rounded transition-colors disabled:opacity-50"
                          title="Restore from this backup"
                        >
                          {restoring === b.id ? '...' : '🔄'}
                        </button>
                      )}
                      {(b.status === 'completed' || b.status === 'failed') && isAdmin && (
                        <button
                          onClick={() => {
                            if (confirm('Delete this backup?')) deleteMutation.mutate(b.id);
                          }}
                          className="px-2 py-1 text-xs bg-red-100 hover:bg-red-200 rounded transition-colors"
                          title="Delete backup"
                        >
                          🗑️
                        </button>
                      )}
                    </div>
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
