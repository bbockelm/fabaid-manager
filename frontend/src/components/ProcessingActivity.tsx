'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import { api, DocumentProcessingRun } from '@/lib/api';
import { useQueryClient } from '@tanstack/react-query';
import { MemoizedMarkdown } from '@/components/MemoizedMarkdown';

// Status badge colors
const statusColors: Record<string, string> = {
  pending: 'bg-gray-100 text-gray-700',
  extracting: 'bg-yellow-100 text-yellow-800',
  processing: 'bg-blue-100 text-blue-800',
  applying: 'bg-indigo-100 text-indigo-800',
  completed: 'bg-green-100 text-green-800',
  failed: 'bg-red-100 text-red-800',
};

// Tool call icons
const toolIcons: Record<string, string> = {
  create_budget: '📊',
  create_line_item: '➕',
  update_line_item: '✏️',
  delete_line_item: '🗑️',
  create_personnel: '👤',
  update_personnel: '✏️',
  upsert_fringe_rate: '📈',
  create_overhead_rate: '🏷️',
  update_overhead_rate: '✏️',
  update_subaward: '📋',
  report_summary: '📝',
};

interface ActivityEntry {
  id: number;
  type: 'status' | 'error';
  text: string;
  timestamp: Date;
}

/**
 * ProcessingActivityStream starts AI processing of a document and polls for
 * status updates. Replaces the old SSE-based implementation.
 */
export function ProcessingActivityStream({
  entityType,
  entityId,
  docId,
  userPrompt,
  onComplete,
}: {
  entityType: string;
  entityId: string;
  docId: string;
  userPrompt?: string;
  onComplete?: (runId: string) => void;
}) {
  const [status, setStatus] = useState('pending');
  const [statusDetail, setStatusDetail] = useState('Starting...');
  const [activities, setActivities] = useState<ActivityEntry[]>([]);
  const [run, setRun] = useState<DocumentProcessingRun | null>(null);
  const [error, setError] = useState('');
  const [runId, setRunId] = useState('');
  const [isActive, setIsActive] = useState(true);
  const [startTime] = useState(() => Date.now());
  const [elapsed, setElapsed] = useState(0);
  const idRef = useRef(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const lastDetailRef = useRef('');
  const queryClient = useQueryClient();

  const addActivity = useCallback((entry: Omit<ActivityEntry, 'id' | 'timestamp'>) => {
    setActivities((prev) => [
      ...prev,
      { ...entry, id: ++idRef.current, timestamp: new Date() },
    ]);
  }, []);

  // Elapsed timer — ticks every second while active
  useEffect(() => {
    if (!isActive) return;
    const interval = setInterval(() => setElapsed(Date.now() - startTime), 1000);
    return () => clearInterval(interval);
  }, [isActive, startTime]);

  // Start processing and poll for updates
  useEffect(() => {
    let pollTimer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;

    async function startAndPoll() {
      // Step 1: Start processing
      let activeRunId = '';
      try {
        addActivity({ type: 'status', text: 'Starting AI processing...' });
        const result = await api.budgetDocuments.processDocument(entityType, entityId, docId, userPrompt);
        if (cancelled) return;

        activeRunId = result.run_id;
        setRunId(result.run_id);
        setStatus(result.status);
        addActivity({ type: 'status', text: `Processing started (run ${result.run_id.slice(0, 8)})` });
      } catch (err: unknown) {
        if (cancelled) return;
        const msg = err instanceof Error ? err.message : 'Failed to start processing';
        setError(msg);
        setStatus('failed');
        setStatusDetail(msg);
        addActivity({ type: 'error', text: msg });
        setIsActive(false);
        return;
      }

      // Step 2: Poll for status updates every 2 seconds
      async function poll() {
        if (cancelled) return;
        try {
          const latestRun = await api.budgetDocuments.getProcessingRun(entityType, entityId, activeRunId);
          if (cancelled) return;

          setRun(latestRun);
          setStatus(latestRun.status);
          setStatusDetail(latestRun.status_detail);

          // Add activity entry when status_detail changes
          if (latestRun.status_detail && latestRun.status_detail !== lastDetailRef.current) {
            lastDetailRef.current = latestRun.status_detail;
            addActivity({ type: 'status', text: latestRun.status_detail });
          }

          // Check if terminal
          if (latestRun.status === 'completed' || latestRun.status === 'failed') {
            setIsActive(false);
            if (latestRun.error_msg) {
              setError(latestRun.error_msg);
            }
            // Invalidate queries so lists refresh
            queryClient.invalidateQueries({ queryKey: ['budget-documents'] });
            queryClient.invalidateQueries({ queryKey: ['institution-budgets'] });
            queryClient.invalidateQueries({ queryKey: ['personnel'] });
            queryClient.invalidateQueries({ queryKey: ['processing-runs'] });
            if (onComplete) onComplete(latestRun.id);
            return;
          }

          // Schedule next poll
          pollTimer = setTimeout(poll, 2000);
        } catch {
          if (cancelled) return;
          // Network error — retry polling
          pollTimer = setTimeout(poll, 3000);
        }
      }

      // Small delay before first poll to give the server time to start
      pollTimer = setTimeout(poll, 1500);
    }

    startAndPoll();

    return () => {
      cancelled = true;
      if (pollTimer) clearTimeout(pollTimer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entityType, entityId, docId]);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [activities]);

  // Parse actions from completed run
  const actions = run ? parseJSON<Array<{
    tool_name: string;
    arguments: string;
    result: string;
    error?: string;
    timestamp: string;
  }>>(run.actions_taken, []) : [];

  return (
    <div className="bg-white rounded-lg border shadow-sm overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-gray-50 border-b">
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusColors[status] || statusColors.pending}`}>
            {isActive && <span className="mr-1.5 h-2 w-2 rounded-full bg-current animate-pulse" />}
            {status}
          </span>
          <span className="text-sm text-gray-600">{statusDetail}</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-xs text-gray-500 font-mono tabular-nums">
            {formatElapsed(elapsed)}
          </span>
          {runId && (
            <span className="text-xs text-gray-400 font-mono">Run: {runId.slice(0, 8)}</span>
          )}
        </div>
      </div>

      {/* Activity log */}
      <div
        ref={scrollRef}
        className="max-h-[400px] overflow-y-auto divide-y divide-gray-100"
      >
        {activities.map((entry) => (
          <div key={entry.id} className={`px-4 py-2 text-sm ${entry.type === 'error' ? 'bg-red-50' : ''}`}>
            <div className="flex items-start gap-2">
              <span className="text-xs text-gray-400 font-mono mt-0.5 shrink-0">
                {entry.timestamp.toLocaleTimeString()}
              </span>
              <span className="shrink-0">
                {entry.type === 'status' && 'ℹ️'}
                {entry.type === 'error' && '❌'}
              </span>
              <span className={`whitespace-pre-wrap break-words ${entry.type === 'error' ? 'text-red-700' : ''}`}>
                {entry.text}
              </span>
            </div>
          </div>
        ))}
        {activities.length === 0 && isActive && (
          <div className="px-4 py-8 text-center text-gray-400 text-sm">
            Waiting for processing to begin...
          </div>
        )}
      </div>

      {/* Error banner */}
      {error && !isActive && status === 'failed' && (
        <div className="px-4 py-3 bg-red-50 border-t border-red-200 text-sm text-red-700">
          <strong>Error:</strong> {error}
        </div>
      )}

      {/* Summary */}
      {run?.summary_md && (
        <div className="border-t">
          <details className="group" open={!isActive}>
            <summary className="px-4 py-3 cursor-pointer text-sm font-medium text-gray-700 hover:bg-gray-50 flex items-center gap-2">
              <span className="transform transition-transform group-open:rotate-90">▶</span>
              AI Summary
            </summary>
            <div className="px-4 py-3 border-t bg-gray-50">
              <div className="prose prose-sm max-w-none text-gray-700">
                <MemoizedMarkdown content={run.summary_md} id="stream-summary" />
              </div>
            </div>
          </details>
        </div>
      )}

      {/* Actions taken */}
      {actions.length > 0 && (
        <details className="border-t">
          <summary className="px-4 py-2 cursor-pointer text-xs font-medium text-gray-500 hover:bg-gray-50">
            {actions.length} tool call{actions.length !== 1 ? 's' : ''} executed
          </summary>
          <div className="divide-y divide-gray-100 max-h-[300px] overflow-y-auto">
            {actions.map((a, i) => (
              <div key={i} className="px-4 py-2 text-xs">
                <div className="flex items-center gap-2">
                  <span>{toolIcons[a.tool_name] || '🔧'}</span>
                  <span className="font-medium text-gray-700">{a.tool_name}</span>
                  {a.error && <span className="text-red-500">(error)</span>}
                  <span className="text-gray-400 ml-auto">{a.timestamp ? new Date(a.timestamp).toLocaleTimeString() : ''}</span>
                </div>
                <pre className="mt-1 text-gray-500 overflow-x-auto">{formatJsonCompact(a.arguments)}</pre>
                {a.error ? (
                  <pre className="mt-1 text-red-500">{a.error}</pre>
                ) : (
                  <pre className="mt-1 text-green-700 overflow-x-auto">{formatJsonCompact(a.result)}</pre>
                )}
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}

/**
 * ProcessingRunCard displays a completed processing run (non-streaming).
 */
export function ProcessingRunCard({ run }: { run: DocumentProcessingRun }) {
  const [showConvo, setShowConvo] = useState(false);

  const actions = parseJSON<Array<{
    tool_name: string;
    arguments: string;
    result: string;
    error?: string;
    timestamp: string;
  }>>(run.actions_taken, []);

  return (
    <div className="bg-white rounded-lg border shadow-sm overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 bg-gray-50 border-b">
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusColors[run.status] || statusColors.pending}`}>
            {run.status}
          </span>
          <span className="text-sm text-gray-600">{run.status_detail}</span>
        </div>
        <div className="flex items-center gap-3 text-xs text-gray-400">
          {run.llm_model && <span>{run.llm_model}</span>}
          {(run.prompt_tokens > 0 || run.completion_tokens > 0) && (
            <span>{run.prompt_tokens + run.completion_tokens} tokens</span>
          )}
          {run.completed_at && (
            <span>{new Date(run.completed_at).toLocaleString()}</span>
          )}
        </div>
      </div>

      {/* Error */}
      {run.error_msg && (
        <div className="px-4 py-3 bg-red-50 border-b text-sm text-red-700">
          <strong>Error:</strong> {run.error_msg}
        </div>
      )}

      {/* Summary */}
      {run.summary_md && (
        <div className="px-4 py-3">
          <div className="prose prose-sm max-w-none text-gray-700">
            <MemoizedMarkdown content={run.summary_md} id={`run-${run.id}`} />
          </div>
        </div>
      )}

      {/* Actions taken */}
      {actions.length > 0 && (
        <details className="border-t">
          <summary className="px-4 py-2 cursor-pointer text-xs font-medium text-gray-500 hover:bg-gray-50">
            {actions.length} tool call{actions.length !== 1 ? 's' : ''} executed
          </summary>
          <div className="divide-y divide-gray-100 max-h-[300px] overflow-y-auto">
            {actions.map((a, i) => (
              <div key={i} className="px-4 py-2 text-xs">
                <div className="flex items-center gap-2">
                  <span>{toolIcons[a.tool_name] || '🔧'}</span>
                  <span className="font-medium text-gray-700">{a.tool_name}</span>
                  {a.error && <span className="text-red-500">(error)</span>}
                  <span className="text-gray-400 ml-auto">{a.timestamp ? new Date(a.timestamp).toLocaleTimeString() : ''}</span>
                </div>
                <pre className="mt-1 text-gray-500 overflow-x-auto">{formatJsonCompact(a.arguments)}</pre>
                {a.error ? (
                  <pre className="mt-1 text-red-500">{a.error}</pre>
                ) : (
                  <pre className="mt-1 text-green-700 overflow-x-auto">{formatJsonCompact(a.result)}</pre>
                )}
              </div>
            ))}
          </div>
        </details>
      )}

      {/* Conversation (collapsible, for debugging) */}
      {run.conversation && run.conversation !== '[]' && (
        <div className="border-t">
          <button
            className="w-full px-4 py-2 text-left text-xs font-medium text-gray-400 hover:bg-gray-50"
            onClick={() => setShowConvo(!showConvo)}
          >
            {showConvo ? '▼' : '▶'} Full conversation ({run.prompt_tokens + run.completion_tokens} tokens)
          </button>
          {showConvo && (
            <div className="px-4 py-2 border-t bg-gray-50 max-h-[400px] overflow-y-auto">
              <pre className="text-xs text-gray-600 whitespace-pre-wrap">
                {formatJsonCompact(run.conversation)}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// --- Helpers ---

function formatElapsed(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  return min > 0 ? `${min}m ${sec.toString().padStart(2, '0')}s` : `${sec}s`;
}

function formatJsonCompact(jsonStr: string): string {
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2);
  } catch {
    return jsonStr;
  }
}

function parseJSON<T>(str: string, fallback: T): T {
  if (!str) return fallback;
  try {
    return JSON.parse(str) as T;
  } catch {
    return fallback;
  }
}


