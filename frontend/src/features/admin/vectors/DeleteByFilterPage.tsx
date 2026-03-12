import { useMemo, useState } from 'react';
import { deleteByFilterQdrant } from '@/core/api';
import type { DeleteByFilterFilter, DeleteByFilterResponse } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import { useMutationAction } from './useAdminApi';

const DEFAULT_FILTER = JSON.stringify(
  {
    must: [
      {
        key: 'document_version_id',
        match: { value: '' }
      }
    ]
  },
  null,
  2
);

const DeleteByFilterPage = () => {
  const [collection, setCollection] = useState('');
  const [reason, setReason] = useState('');
  const [filterText, setFilterText] = useState(DEFAULT_FILTER);
  const [parsedFilter, setParsedFilter] = useState<DeleteByFilterFilter | null>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [dryRunResult, setDryRunResult] = useState<DeleteByFilterResponse | null>(null);
  const [confirmChecked, setConfirmChecked] = useState(false);
  const dryRunMutation = useMutationAction(deleteByFilterQdrant);
  const deleteMutation = useMutationAction(deleteByFilterQdrant);

  const step = useMemo(() => {
    if (!parsedFilter) return 1;
    if (!dryRunResult) return 2;
    return 3;
  }, [dryRunResult, parsedFilter]);

  const handleParse = () => {
    try {
      const parsed = JSON.parse(filterText) as DeleteByFilterFilter;
      if (!parsed || !Array.isArray(parsed.must) || parsed.must.length === 0) {
        throw new Error('filter.must must be a non-empty array');
      }
      setParsedFilter(parsed);
      setParseError(null);
      setDryRunResult(null);
      setConfirmChecked(false);
      dryRunMutation.reset();
      deleteMutation.reset();
    } catch (error) {
      setParsedFilter(null);
      setParseError(error instanceof Error ? error.message : 'Invalid filter JSON');
    }
  };

  const runDryRun = async () => {
    if (!parsedFilter) return;
    const result = await dryRunMutation.mutate({
      collection: collection.trim() || undefined,
      reason: reason.trim() || undefined,
      filter: parsedFilter,
      dry_run: true,
      confirm: false
    });
    setDryRunResult(result);
    setConfirmChecked(false);
  };

  const runDelete = async () => {
    if (!parsedFilter || !dryRunResult || !confirmChecked) return;
    await deleteMutation.mutate({
      collection: collection.trim() || undefined,
      reason: reason.trim() || undefined,
      filter: parsedFilter,
      dry_run: false,
      confirm: true
    });
  };

  return (
    <Panel title="Delete by Filter">
      <div className="grid">
        <div className="badge">Step {step} of 3</div>
        <Input label="Collection (optional)" value={collection} onChange={(e) => setCollection(e.target.value)} />
        <Input label="Reason (recommended)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <label>
          <div className="label">Filter JSON</div>
          <textarea
            className="textarea vector-textarea"
            value={filterText}
            onChange={(e) => setFilterText(e.target.value)}
            spellCheck={false}
          />
        </label>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <Button variant="secondary" onClick={handleParse}>
            Validate Filter (Step 1)
          </Button>
          <Button onClick={() => void runDryRun()} disabled={!parsedFilter || dryRunMutation.isLoading}>
            {dryRunMutation.isLoading ? 'Running dry run...' : 'Dry Run Preview (Step 2)'}
          </Button>
        </div>
        {parseError && <div className="badge">Invalid filter: {parseError}</div>}
        {dryRunMutation.error && <div className="badge">Dry run error: {dryRunMutation.error}</div>}
        {dryRunResult && (
          <div className="source-item">
            <div className="label">Dry Run Preview</div>
            <div className="vector-meta-grid">
              <div className="badge">Collection: {dryRunResult.collection}</div>
              <div className="badge">Scope estimated: {dryRunResult.scope_estimated ? 'yes' : 'no'}</div>
              <div className="badge">Matched vectors: {dryRunResult.estimated_scope ?? 0}</div>
              <div className="badge">Filter: {dryRunResult.filter_summary}</div>
            </div>
            <div className="badge status-warn">Warning: delete is irreversible.</div>
          </div>
        )}
        <label className="badge" style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
          <input
            type="checkbox"
            checked={confirmChecked}
            onChange={(e) => setConfirmChecked(e.target.checked)}
            disabled={!dryRunResult}
          />
          I confirm this delete operation is irreversible.
        </label>
        <Button onClick={() => void runDelete()} disabled={!dryRunResult || !confirmChecked || deleteMutation.isLoading}>
          {deleteMutation.isLoading ? 'Deleting...' : 'Confirm Delete (Step 3)'}
        </Button>
        {deleteMutation.error && <div className="badge">Delete error: {deleteMutation.error}</div>}
        {deleteMutation.data && (
          <div className="badge status-ok">
            {deleteMutation.data.summary} | matched scope: {deleteMutation.data.estimated_scope ?? '-'}
          </div>
        )}
      </div>
    </Panel>
  );
};

export default DeleteByFilterPage;
