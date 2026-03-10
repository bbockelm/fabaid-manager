'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  BudgetDocument,
  InstitutionBudget,
} from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { useState, useMemo, useRef, useEffect, Suspense } from 'react';
import { useSearchParams, useRouter, usePathname } from 'next/navigation';
import Link from 'next/link';
import { ScrollableTable } from '@/components/ScrollableTable';
import { ProcessingActivityStream, ProcessingRunCard } from '@/components/ProcessingActivity';

const YEAR_LABELS: Record<number, string> = {
  1: 'Y1 (2026-27)',
  2: 'Y2 (2027-28)',
  3: 'Y3 (2028-29)',
  4: 'Y4 (2029-30)',
  5: 'Y5 (2030-31)',
};

const DOC_TYPES = [
  { value: 'budget', label: 'Official Budget' },
  { value: 'budget_justification', label: 'Budget Justification' },
];

interface InstitutionOption {
  entityType: string;
  entityId: string;
  label: string;
}

export default function DocumentsPage() {
  return (
    <Suspense fallback={<div className="p-4">Loading documents...</div>}>
      <DocumentsPageInner />
    </Suspense>
  );
}

function DocumentsPageInner() {
  const { grantId, isLoading: grantLoading } = useGrant();
  const { isSubawardAdmin, permittedInstitutions } = useAuth();
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const initialEntity = searchParams.get('entity');
  const [selectedInst, setSelectedInst] = useState<string>(initialEntity ?? '');
  const [showDeleted, setShowDeleted] = useState(false);

  const { data: grant } = useQuery({
    queryKey: ['grant', grantId],
    queryFn: () => api.grants.get(grantId!),
    enabled: !!grantId,
  });
  const { data: subawards } = useQuery({
    queryKey: ['subawards', grantId],
    queryFn: () => api.subawards.list(grantId!),
    enabled: !!grantId,
  });

  const allInstitutions = useMemo<InstitutionOption[]>(() => {
    const opts: InstitutionOption[] = [];
    if (grant) opts.push({ entityType: 'grant', entityId: grant.id, label: `${grant.institution} (Lead)` });
    for (const s of subawards ?? []) opts.push({ entityType: 'subaward', entityId: s.id, label: s.institution });
    return opts;
  }, [grant, subawards]);

  // Subaward admins can only see their own institutions' documents
  const institutions = useMemo(() => {
    if (!isSubawardAdmin || permittedInstitutions.length === 0) return allInstitutions;
    return allInstitutions.filter((i) =>
      permittedInstitutions.some((p) => i.label.startsWith(p))
    );
  }, [allInstitutions, isSubawardAdmin, permittedInstitutions]);

  const activeInst = institutions.find((i) => `${i.entityType}:${i.entityId}` === selectedInst) || institutions[0];

  // Sync selected institution to URL so page reloads preserve the choice
  useEffect(() => {
    const key = activeInst ? `${activeInst.entityType}:${activeInst.entityId}` : '';
    const params = new URLSearchParams();
    if (key) params.set('entity', key);
    const qs = params.toString();
    const target = qs ? `${pathname}?${qs}` : pathname;
    const current = `${pathname}${searchParams.toString() ? '?' + searchParams.toString() : ''}`;
    if (target !== current) {
      router.replace(target, { scroll: false });
    }
  }, [activeInst, pathname, router, searchParams]);

  if (grantLoading) return <div className="p-4">Loading...</div>;
  if (!grantId) return <div className="p-4">No project configured. <Link href="/settings" className="text-nsf-light hover:underline">Set up project</Link></div>;

  return (
    <div className="max-w-6xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">Budget Documents</h1>
        <p className="text-sm text-gray-500">
          Upload and manage official budget PDFs and budget justification documents.
        </p>
      </div>

      <div className="flex flex-wrap items-end gap-4">
        <div>
          <label className="block text-xs font-medium text-gray-500 mb-1">Institution</label>
          <select
            value={activeInst ? `${activeInst.entityType}:${activeInst.entityId}` : ''}
            onChange={(e) => setSelectedInst(e.target.value)}
            className="border rounded-md px-3 py-2 text-sm min-w-[200px]"
          >
            {institutions.map((i) => (
              <option key={`${i.entityType}:${i.entityId}`} value={`${i.entityType}:${i.entityId}`}>
                {i.label}
              </option>
            ))}
          </select>
        </div>
        <label className="flex items-center gap-2 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={showDeleted}
            onChange={(e) => setShowDeleted(e.target.checked)}
            className="rounded"
          />
          Show deleted
        </label>
      </div>

      {activeInst && (
        <InstitutionDocumentsPanel
          entityType={activeInst.entityType}
          entityId={activeInst.entityId}
          institutionLabel={activeInst.label}
          showDeleted={showDeleted}
        />
      )}
    </div>
  );
}

function InstitutionDocumentsPanel({
  entityType,
  entityId,
  institutionLabel,
  showDeleted,
}: {
  entityType: string;
  entityId: string;
  institutionLabel: string;
  showDeleted: boolean;
}) {
  const queryClient = useQueryClient();
  const [showUpload, setShowUpload] = useState(false);
  const [processingDocId, setProcessingDocId] = useState<string | null>(null);
  const [pendingProcessDocId, setPendingProcessDocId] = useState<string | null>(null);
  const [userPrompt, setUserPrompt] = useState('');

  const docsKey = ['budget-documents', entityType, entityId, showDeleted];
  const { data: documents, isLoading } = useQuery({
    queryKey: docsKey,
    queryFn: () => api.budgetDocuments.list(entityType, entityId, showDeleted),
  });

  const { data: budgets } = useQuery({
    queryKey: ['institution-budgets', entityType, entityId],
    queryFn: () => api.institutionBudgets.list(entityType, entityId, false),
  });

  const runsKey = ['processing-runs', entityType, entityId];
  const { data: processingRuns } = useQuery({
    queryKey: runsKey,
    queryFn: () => api.budgetDocuments.listEntityProcessingRuns(entityType, entityId),
  });

  const deleteMutation = useMutation({
    mutationFn: (docId: string) => api.budgetDocuments.delete(entityType, entityId, docId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: docsKey }),
  });

  const activeDocuments = (documents ?? []).filter((d) => !d.deleted_at);
  const deletedDocuments = (documents ?? []).filter((d) => d.deleted_at);

  const processingDocFilename = processingDocId
    ? activeDocuments.find((d) => d.id === processingDocId)?.filename
    : null;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-800">{institutionLabel}</h2>
        <button
          onClick={() => setShowUpload(!showUpload)}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors"
        >
          {showUpload ? 'Cancel' : '+ Upload Document'}
        </button>
      </div>

      {showUpload && (
        <UploadForm
          entityType={entityType}
          entityId={entityId}
          budgets={budgets ?? []}
          onSuccess={(docId, processWithAI) => {
            queryClient.invalidateQueries({ queryKey: docsKey });
            setShowUpload(false);
            if (processWithAI && docId) {
              setProcessingDocId(docId);
            }
          }}
        />
      )}

      {isLoading && <p className="text-sm text-gray-400">Loading documents...</p>}

      {!isLoading && activeDocuments.length === 0 && (
        <div className="bg-white rounded-lg border p-8 text-center text-gray-400 text-sm">
          No documents uploaded yet. Click &quot;+ Upload Document&quot; to add a budget PDF or spreadsheet.
        </div>
      )}

      {activeDocuments.length > 0 && (
        <DocumentTable
          documents={activeDocuments}
          entityType={entityType}
          entityId={entityId}
          budgets={budgets ?? []}
          onDelete={(id) => {
            if (confirm('Mark this document as deleted? (The file is preserved for audit.)'))
              deleteMutation.mutate(id);
          }}
          onProcess={(docId) => setPendingProcessDocId(docId)}
          processingDocId={processingDocId || pendingProcessDocId}
        />
      )}

      {/* Process confirmation — user can add instructions before starting */}
      {pendingProcessDocId && !processingDocId && (
        <div className="bg-white rounded-lg border shadow-sm p-4 space-y-3">
          <h3 className="text-sm font-medium text-gray-700">
            🤖 Process: {activeDocuments.find((d) => d.id === pendingProcessDocId)?.filename || 'document'}
          </h3>
          <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">
              Additional instructions for the AI (optional)
            </label>
            <textarea
              value={userPrompt}
              onChange={(e) => setUserPrompt(e.target.value)}
              placeholder={'e.g., "The staff member\'s salary was updated to $95,000 this year" or "Ignore the travel line items"'}
              className="w-full border rounded-md px-3 py-2 text-sm resize-none h-16"
            />
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => {
                setProcessingDocId(pendingProcessDocId);
                setPendingProcessDocId(null);
              }}
              className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 text-sm transition-colors"
            >
              Start Processing
            </button>
            <button
              onClick={() => { setPendingProcessDocId(null); setUserPrompt(''); }}
              className="px-4 py-2 bg-gray-100 text-gray-700 rounded-md hover:bg-gray-200 text-sm transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Active AI processing stream */}
      {processingDocId && (
        <div className="space-y-2">
          <h3 className="text-sm font-medium text-gray-700">
            🤖 Processing: {processingDocFilename || 'document'}
          </h3>
          <ProcessingActivityStream
            entityType={entityType}
            entityId={entityId}
            docId={processingDocId}
            userPrompt={userPrompt}
            onComplete={() => {
              setProcessingDocId(null);
              setUserPrompt('');
              queryClient.invalidateQueries({ queryKey: runsKey });
            }}
          />
        </div>
      )}

      {/* Previous processing runs */}
      {(processingRuns ?? []).length > 0 && (
        <div className="space-y-3">
          <h3 className="text-sm font-medium text-gray-700">Processing History</h3>
          {(processingRuns ?? []).map((run) => (
            <ProcessingRunCard key={run.id} run={run} />
          ))}
        </div>
      )}

      {showDeleted && deletedDocuments.length > 0 && (
        <div>
          <h3 className="text-sm font-medium text-gray-500 mt-6 mb-2">Deleted Documents</h3>
          <DocumentTable
            documents={deletedDocuments}
            entityType={entityType}
            entityId={entityId}
            budgets={budgets ?? []}
            isDeletedView
          />
        </div>
      )}
    </div>
  );
}

function DocumentTable({
  documents,
  entityType,
  entityId,
  budgets,
  onDelete,
  onProcess,
  processingDocId,
  isDeletedView = false,
}: {
  documents: BudgetDocument[];
  entityType: string;
  entityId: string;
  budgets: InstitutionBudget[];
  onDelete?: (id: string) => void;
  onProcess?: (docId: string) => void;
  processingDocId?: string | null;
  isDeletedView?: boolean;
}) {
  const budgetLabel = (budgetId?: string) => {
    if (!budgetId) return '\u2014';
    const b = budgets.find((x) => x.id === budgetId);
    return b ? `${YEAR_LABELS[b.fiscal_year] ?? `Y${b.fiscal_year}`} v${b.version}` : budgetId.slice(0, 8);
  };

  const docTypeLabel = (dt: string) =>
    DOC_TYPES.find((t) => t.value === dt)?.label ?? dt;

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  return (
    <ScrollableTable className={`bg-white rounded-lg border ${isDeletedView ? 'opacity-60' : ''}`}>
      <table className="w-full">
        <thead className="bg-gray-50 border-b">
          <tr>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Type</th>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Filename</th>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Budget</th>
            <th className="text-right px-4 py-3 text-xs font-medium text-gray-600 uppercase">Size</th>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Uploaded By</th>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Date</th>
            <th className="text-left px-4 py-3 text-xs font-medium text-gray-600 uppercase">Notes</th>
            {!isDeletedView && <th className="px-2 py-3 w-24"></th>}
          </tr>
        </thead>
        <tbody className="divide-y">
          {documents.map((doc) => (
            <tr key={doc.id} className="hover:bg-gray-50">
              <td className="px-4 py-3 text-sm">
                <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
                  doc.doc_type === 'budget'
                    ? 'bg-blue-100 text-blue-800'
                    : 'bg-purple-100 text-purple-800'
                }`}>
                  {docTypeLabel(doc.doc_type)}
                </span>
              </td>
              <td className="px-4 py-3 text-sm font-medium">{doc.filename}</td>
              <td className="px-4 py-3 text-sm text-gray-600">{budgetLabel(doc.budget_id)}</td>
              <td className="px-4 py-3 text-sm text-right text-gray-500">{formatBytes(doc.file_size)}</td>
              <td className="px-4 py-3 text-sm text-gray-600">{doc.uploaded_by_name || '\u2014'}</td>
              <td className="px-4 py-3 text-sm text-gray-500">
                {new Date(doc.created_at).toLocaleDateString()}
              </td>
              <td className="px-4 py-3 text-sm text-gray-500 max-w-[200px] truncate" title={doc.notes}>
                {doc.notes || '\u2014'}
              </td>
              {!isDeletedView && (
                <td className="px-2 py-3">
                  <div className="flex gap-2">
                    <a
                      href={api.budgetDocuments.downloadUrl(entityType, entityId, doc.id)}
                      className="text-nsf-light hover:underline text-xs"
                    >
                      Download
                    </a>
                    {onProcess && (
                      <button
                        onClick={() => onProcess(doc.id)}
                        disabled={!!processingDocId}
                        className="text-indigo-500 hover:text-indigo-700 text-xs disabled:opacity-40 disabled:cursor-not-allowed"
                        title="Process this document with AI to extract budget data"
                      >
                        🤖 Process
                      </button>
                    )}
                    {onDelete && (
                      <button
                        onClick={() => onDelete(doc.id)}
                        className="text-red-400 hover:text-red-600 text-xs"
                      >
                        Delete
                      </button>
                    )}
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </ScrollableTable>
  );
}

function UploadForm({
  entityType,
  entityId,
  budgets,
  onSuccess,
}: {
  entityType: string;
  entityId: string;
  budgets: InstitutionBudget[];
  onSuccess: (docId: string | null, processWithAI: boolean) => void;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [docType, setDocType] = useState('budget');
  const [budgetId, setBudgetId] = useState('');
  const [notes, setNotes] = useState('');
  const [processWithAI, setProcessWithAI] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');

  const latestBudgets = useMemo(
    () => budgets.filter((b) => b.is_latest).sort((a, b) => a.fiscal_year - b.fiscal_year),
    [budgets]
  );

  const handleUpload = async () => {
    const file = fileRef.current?.files?.[0];
    if (!file) {
      setError('Please select a file.');
      return;
    }
    setError('');
    setUploading(true);
    try {
      const doc = await api.budgetDocuments.upload(
        entityType,
        entityId,
        file,
        docType,
        budgetId || undefined,
        notes || undefined
      );
      onSuccess(doc.id, processWithAI);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="bg-white p-6 rounded-lg border space-y-4">
      <h3 className="font-semibold text-gray-800">Upload Budget Document</h3>
      <p className="text-sm text-gray-500">
        Documents are encrypted before storage. Only users with the master key can decrypt them.
      </p>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">File *</label>
          <input
            ref={fileRef}
            type="file"
            accept=".pdf,.xlsx,.xls,.csv,application/pdf,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,application/vnd.ms-excel,text/csv"
            className="w-full border rounded-md px-3 py-2 text-sm file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:text-sm file:bg-nsf-light file:text-white hover:file:bg-nsf-blue"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Document Type</label>
          <select
            value={docType}
            onChange={(e) => setDocType(e.target.value)}
            className="w-full border rounded-md px-3 py-2 text-sm"
          >
            {DOC_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Associated Budget</label>
          <select
            value={budgetId}
            onChange={(e) => setBudgetId(e.target.value)}
            className="w-full border rounded-md px-3 py-2 text-sm"
          >
            <option value="">{'\u2014'} None (general) {'\u2014'}</option>
            {latestBudgets.map((b) => (
              <option key={b.id} value={b.id}>
                {YEAR_LABELS[b.fiscal_year] ?? `Year ${b.fiscal_year}`} v{b.version} ({b.status})
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Notes</label>
          <input
            type="text"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Optional notes"
            className="w-full border rounded-md px-3 py-2 text-sm"
          />
        </div>
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <div className="flex items-center gap-4">
        <button
          onClick={handleUpload}
          disabled={uploading}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue disabled:opacity-50 transition-colors"
        >
          {uploading ? 'Encrypting & Uploading...' : 'Upload Document'}
        </button>
        <label className="flex items-center gap-2 text-sm text-gray-600">
          <input
            type="checkbox"
            checked={processWithAI}
            onChange={(e) => setProcessWithAI(e.target.checked)}
            className="rounded"
          />
          🤖 Process with AI after upload
        </label>
      </div>
    </div>
  );
}
