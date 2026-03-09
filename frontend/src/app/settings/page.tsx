'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, Grant, OIDCConfig, BackupSettings } from '@/lib/api';
import { useGrant } from '@/lib/grant-context';
import { useAuth } from '@/lib/auth-context';
import { useState, useEffect } from 'react';
import CurrencyInput from '@/components/CurrencyInput';

export default function SettingsPage() {
  const { grant, grantId, isLoading } = useGrant();
  const { isAdmin } = useAuth();
  const queryClient = useQueryClient();

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="max-w-3xl space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-nsf-blue">Project Settings</h1>
        <p className="text-sm text-gray-500">Configure the project grant details</p>
      </div>

      {grant ? (
        <EditGrantForm grant={grant} />
      ) : (
        <CreateGrantForm />
      )}

      {isAdmin && <OIDCConfigSection />}
      {isAdmin && <BackupConfigSection />}
    </div>
  );
}

function EditGrantForm({ grant }: { grant: Grant }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState({
    award_number: grant.award_number,
    title: grant.title,
    pi_name: grant.pi_name,
    institution: grant.institution ?? '',
    agency: grant.agency,
    start_date: grant.start_date,
    end_date: grant.end_date,
    total_budget: grant.total_budget,
    salary_escalation_rate: grant.salary_escalation_rate ?? 0,
    status: grant.status,
  });

  useEffect(() => {
    setForm({
      award_number: grant.award_number,
      title: grant.title,
      pi_name: grant.pi_name,
      institution: grant.institution ?? '',
      agency: grant.agency,
      start_date: grant.start_date,
      end_date: grant.end_date,
      total_budget: grant.total_budget,
      salary_escalation_rate: grant.salary_escalation_rate ?? 0,
      status: grant.status,
    });
  }, [grant]);

  const updateMutation = useMutation({
    mutationFn: (data: Partial<Grant>) => api.grants.update(grant.id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['grants'] });
      queryClient.invalidateQueries({ queryKey: ['grant', grant.id] });
    },
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(form); }}
      className="bg-white p-6 rounded-lg border space-y-4"
    >
      <h2 className="font-semibold text-lg">Edit Project Grant</h2>
      <GrantFields form={form} setForm={setForm} />
      <div className="flex items-center gap-4">
        <button type="submit" disabled={updateMutation.isPending}
          className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50">
          {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
        </button>
        {updateMutation.isSuccess && (
          <span className="text-green-600 text-sm">Saved!</span>
        )}
        {updateMutation.isError && (
          <span className="text-red-600 text-sm">Error: {updateMutation.error?.message}</span>
        )}
      </div>
    </form>
  );
}

function CreateGrantForm() {
  const queryClient = useQueryClient();
  const [form, setForm] = useState({
    award_number: '',
    title: '',
    pi_name: '',
    institution: '',
    agency: 'NSF',
    start_date: '2026-05-01',
    end_date: '2031-04-30',
    total_budget: 0,
    salary_escalation_rate: 0.03,
    status: 'active',
  });

  const createMutation = useMutation({
    mutationFn: (data: Partial<Grant>) => api.grants.create(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['grants'] }),
  });

  return (
    <form
      onSubmit={(e) => { e.preventDefault(); createMutation.mutate(form); }}
      className="bg-white p-6 rounded-lg border space-y-4"
    >
      <h2 className="font-semibold text-lg">Set Up Project Grant</h2>
      <p className="text-sm text-gray-500">
        This tracker is designed for a single project. Create the grant record to get started.
      </p>
      <GrantFields form={form} setForm={setForm} />
      <button type="submit" disabled={createMutation.isPending}
        className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50">
        {createMutation.isPending ? 'Creating...' : 'Create Project'}
      </button>
    </form>
  );
}

function GrantFields({
  form,
  setForm,
}: {
  form: {
    award_number: string;
    title: string;
    pi_name: string;
    institution: string;
    agency: string;
    start_date: string;
    end_date: string;
    total_budget: number;
    salary_escalation_rate: number;
    status: string;
  };
  setForm: (f: typeof form) => void;
}) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Award Number</label>
        <input type="text" required value={form.award_number}
          onChange={(e) => setForm({ ...form, award_number: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" placeholder="e.g. 2345678" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">PI Name</label>
        <input type="text" required value={form.pi_name}
          onChange={(e) => setForm({ ...form, pi_name: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Lead Institution</label>
        <input type="text" required value={form.institution}
          onChange={(e) => setForm({ ...form, institution: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm"
          placeholder="e.g. University of Wisconsin–Madison" />
      </div>
      <div className="col-span-2">
        <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
        <input type="text" required value={form.title}
          onChange={(e) => setForm({ ...form, title: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Agency</label>
        <input type="text" value={form.agency}
          onChange={(e) => setForm({ ...form, agency: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
        <select value={form.status}
          onChange={(e) => setForm({ ...form, status: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm">
          <option value="active">Active</option>
          <option value="no-cost-extension">No-Cost Extension</option>
          <option value="closed">Closed</option>
        </select>
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Start Date</label>
        <input type="date" required value={form.start_date}
          onChange={(e) => setForm({ ...form, start_date: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">End Date</label>
        <input type="date" required value={form.end_date}
          onChange={(e) => setForm({ ...form, end_date: e.target.value })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Total Budget ($)</label>
        <CurrencyInput value={form.total_budget} required
          onChange={(val) => setForm({ ...form, total_budget: val })}
          className="w-full border rounded-md px-3 py-2 text-sm" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Salary Escalation Rate (%)</label>
        <input type="number" step="0.1" min="0" max="20"
          value={form.salary_escalation_rate === 0 ? '' : (form.salary_escalation_rate * 100).toFixed(1)}
          onChange={(e) => setForm({ ...form, salary_escalation_rate: (parseFloat(e.target.value) || 0) / 100 })}
          className="w-full border rounded-md px-3 py-2 text-sm"
          placeholder="e.g. 3.0" />
        <p className="text-xs text-gray-400 mt-0.5">Annual salary increase applied per year (e.g. 3% = 0.03)</p>
      </div>
    </div>
  );
}

function OIDCConfigSection() {
  const queryClient = useQueryClient();
  const { data: config, isLoading } = useQuery({
    queryKey: ['admin', 'oidc-config'],
    queryFn: api.admin.getOIDCConfig,
  });

  const [form, setForm] = useState({
    oidc_issuer: '',
    oidc_client_id: '',
    oidc_client_secret: '',
  });
  const [showSecret, setShowSecret] = useState(false);

  useEffect(() => {
    if (config) {
      setForm({
        oidc_issuer: config.oidc_issuer ?? '',
        oidc_client_id: config.oidc_client_id ?? '',
        oidc_client_secret: '', // Don't prefill secret
      });
    }
  }, [config]);

  const updateMut = useMutation({
    mutationFn: (data: { oidc_issuer?: string; oidc_client_id?: string; oidc_client_secret?: string }) => {
      // Only send non-empty fields
      const payload: Record<string, string> = {};
      if (data.oidc_issuer) payload.oidc_issuer = data.oidc_issuer;
      if (data.oidc_client_id) payload.oidc_client_id = data.oidc_client_id;
      if (data.oidc_client_secret) payload.oidc_client_secret = data.oidc_client_secret;
      return api.admin.updateOIDCConfig(payload);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'oidc-config'] });
    },
  });

  const callbackUrl = typeof window !== 'undefined'
    ? `${window.location.origin}/api/v1/auth/oidc/callback`
    : '';

  if (isLoading) return <div className="text-gray-400 text-sm">Loading OIDC config...</div>;

  return (
    <div className="bg-white p-6 rounded-lg border space-y-4">
      <h2 className="font-semibold text-lg">OIDC Authentication</h2>
      <p className="text-sm text-gray-500">
        Configure OpenID Connect for production authentication.
      </p>

      {callbackUrl && (
        <div className="p-3 bg-blue-50 rounded-md">
          <div className="text-xs font-medium text-blue-700 mb-1">Callback URL</div>
          <code className="text-xs text-blue-900 break-all">{callbackUrl}</code>
          <p className="text-xs text-blue-600 mt-1">
            Register this URL with your OIDC provider as an allowed redirect URI.
          </p>
        </div>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          updateMut.mutate(form);
        }}
        className="space-y-4"
      >
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Issuer URL</label>
          <input
            type="url"
            value={form.oidc_issuer}
            onChange={(e) => setForm({ ...form, oidc_issuer: e.target.value })}
            placeholder="https://accounts.google.com"
            className="w-full border rounded-md px-3 py-2 text-sm"
          />
          {config?.oidc_issuer && (
            <p className="text-xs text-green-600 mt-0.5">Currently set: {config.oidc_issuer}</p>
          )}
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Client ID</label>
          <input
            type="text"
            value={form.oidc_client_id}
            onChange={(e) => setForm({ ...form, oidc_client_id: e.target.value })}
            placeholder="your-client-id"
            className="w-full border rounded-md px-3 py-2 text-sm"
          />
          {config?.oidc_client_id && (
            <p className="text-xs text-green-600 mt-0.5">Currently set: {config.oidc_client_id}</p>
          )}
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Client Secret</label>
          <div className="relative">
            <input
              type={showSecret ? 'text' : 'password'}
              value={form.oidc_client_secret}
              onChange={(e) => setForm({ ...form, oidc_client_secret: e.target.value })}
              placeholder={config?.secret_set ? '••••• (already set, leave blank to keep)' : 'your-client-secret'}
              className="w-full border rounded-md px-3 py-2 text-sm pr-16"
            />
            <button
              type="button"
              onClick={() => setShowSecret(!showSecret)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-xs text-gray-500 hover:text-gray-700"
            >
              {showSecret ? 'Hide' : 'Show'}
            </button>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <button
            type="submit"
            disabled={updateMut.isPending}
            className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50"
          >
            {updateMut.isPending ? 'Saving...' : 'Save OIDC Settings'}
          </button>
          {updateMut.isSuccess && <span className="text-green-600 text-sm">Saved!</span>}
          {updateMut.isError && (
            <span className="text-red-600 text-sm">Error: {updateMut.error?.message}</span>
          )}
        </div>
      </form>
    </div>
  );
}

function BackupConfigSection() {
  const queryClient = useQueryClient();
  const { data: settings, isLoading } = useQuery({
    queryKey: ['admin', 'backup-settings'],
    queryFn: api.backup.getSettings,
  });

  const [form, setForm] = useState<BackupSettings>({
    backup_frequency_hours: 0,
    backup_bucket: '',
    backup_endpoint: '',
    backup_access_key: '',
    backup_secret_key: '',
    backup_use_ssl: false,
  });

  useEffect(() => {
    if (settings) {
      setForm({
        backup_frequency_hours: settings.backup_frequency_hours ?? 0,
        backup_bucket: settings.backup_bucket ?? '',
        backup_endpoint: settings.backup_endpoint ?? '',
        backup_access_key: settings.backup_access_key ?? '',
        backup_secret_key: settings.backup_secret_key ?? '',
        backup_use_ssl: settings.backup_use_ssl ?? false,
      });
    }
  }, [settings]);

  const updateMut = useMutation({
    mutationFn: (data: BackupSettings) => api.backup.updateSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'backup-settings'] });
    },
  });

  if (isLoading) return <div className="text-gray-400 text-sm">Loading backup config...</div>;

  return (
    <div className="bg-white p-6 rounded-lg border space-y-4">
      <h2 className="font-semibold text-lg">Automated Backups</h2>
      <p className="text-sm text-gray-500">
        Configure automatic backup schedule and optional alternate S3 storage.
        Backups are encrypted with a key derived from the master key.
      </p>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          updateMut.mutate(form);
        }}
        className="space-y-4"
      >
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Backup Frequency (hours)</label>
          <input
            type="number"
            min={0}
            step={1}
            value={form.backup_frequency_hours}
            onChange={(e) => setForm({ ...form, backup_frequency_hours: parseInt(e.target.value) || 0 })}
            className="w-full border rounded-md px-3 py-2 text-sm"
          />
          <p className="text-xs text-gray-400 mt-0.5">
            Set to 0 to disable automatic backups. Recommended: 24 (daily).
          </p>
        </div>

        <div className="border-t pt-4">
          <h3 className="text-sm font-medium text-gray-700 mb-2">Alternate Backup S3 (optional)</h3>
          <p className="text-xs text-gray-400 mb-3">
            Leave empty to store backups in the default S3 bucket.
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Endpoint</label>
              <input
                type="text"
                value={form.backup_endpoint}
                onChange={(e) => setForm({ ...form, backup_endpoint: e.target.value })}
                placeholder="s3.amazonaws.com"
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Bucket</label>
              <input
                type="text"
                value={form.backup_bucket}
                onChange={(e) => setForm({ ...form, backup_bucket: e.target.value })}
                placeholder="my-backup-bucket"
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Access Key</label>
              <input
                type="text"
                value={form.backup_access_key}
                onChange={(e) => setForm({ ...form, backup_access_key: e.target.value })}
                placeholder="Leave empty to use default"
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Secret Key</label>
              <input
                type="password"
                value={form.backup_secret_key}
                onChange={(e) => setForm({ ...form, backup_secret_key: e.target.value })}
                placeholder="Leave empty to use default"
                className="w-full border rounded-md px-3 py-2 text-sm"
              />
            </div>
          </div>
          <div className="mt-3">
            <label className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={form.backup_use_ssl}
                onChange={(e) => setForm({ ...form, backup_use_ssl: e.target.checked })}
                className="rounded border-gray-300"
              />
              Use SSL/TLS
            </label>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <button
            type="submit"
            disabled={updateMut.isPending}
            className="px-4 py-2 bg-nsf-light text-white rounded-md hover:bg-nsf-blue transition-colors disabled:opacity-50"
          >
            {updateMut.isPending ? 'Saving...' : 'Save Backup Settings'}
          </button>
          {updateMut.isSuccess && <span className="text-green-600 text-sm">Saved!</span>}
          {updateMut.isError && (
            <span className="text-red-600 text-sm">Error: {updateMut.error?.message}</span>
          )}
        </div>
      </form>
    </div>
  );
}
