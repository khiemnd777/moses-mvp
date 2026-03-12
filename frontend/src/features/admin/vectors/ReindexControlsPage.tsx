import { useState } from 'react';
import { reindexQdrantAll, reindexQdrantDocument } from '@/core/api';
import type { ReindexAcceptedResponse } from '@/core/types';
import Panel from '@/shared/Panel';
import Input from '@/shared/Input';
import Select from '@/shared/Select';
import Button from '@/shared/Button';
import { useMutationAction } from './useAdminApi';

const ReindexControlsPage = () => {
  const [documentId, setDocumentId] = useState('');
  const [documentVersionId, setDocumentVersionId] = useState('');
  const [singleReason, setSingleReason] = useState('');

  const [allReason, setAllReason] = useState('');
  const [allConfirm, setAllConfirm] = useState(false);
  const [allDocTypeCode, setAllDocTypeCode] = useState('');
  const [allCollection, setAllCollection] = useState('');
  const [allStatus, setAllStatus] = useState('');
  const [allLimit, setAllLimit] = useState('500');

  const singleMutation = useMutationAction(reindexQdrantDocument);
  const allMutation = useMutationAction(reindexQdrantAll);

  const runSingleReindex = async () => {
    await singleMutation.mutate({
      document_id: documentId.trim() || undefined,
      document_version_id: documentVersionId.trim() || undefined,
      reason: singleReason.trim() || undefined,
      force: false
    });
  };

  const runAllReindex = async () => {
    await allMutation.mutate({
      confirm: true,
      reason: allReason.trim(),
      doc_type_code: allDocTypeCode.trim() || undefined,
      collection: allCollection.trim() || undefined,
      status: allStatus || undefined,
      limit: Number(allLimit) || 500,
      force: false
    });
  };

  const singleScopeCount = Number(Boolean(documentId.trim())) + Number(Boolean(documentVersionId.trim()));
  const singleScopeValid = singleScopeCount === 1;

  const renderResult = (result: ReindexAcceptedResponse | null) => {
    if (!result) return null;
    return (
      <div className="source-item">
        <div className="label">Backend Response</div>
        <div className="vector-meta-grid">
          <div className="badge">status: {result.status}</div>
          <div className="badge">accepted: {result.accepted_count}</div>
          <div className="badge">created: {result.created_count}</div>
          <div className="badge">skipped: {result.skipped_count}</div>
        </div>
        <div className="badge">{result.summary}</div>
        {(result.items || []).length > 0 && (
          <pre className="vector-pre">
            {(result.items || [])
              .map((item) => `${item.document_version_id} | job=${item.job_id} | status=${item.job_status} | created=${item.created}`)
              .join('\n')}
          </pre>
        )}
      </div>
    );
  };

  return (
    <div className="grid">
      <Panel title="Reindex Single Document">
        <div className="grid">
          <Input label="document_id" value={documentId} onChange={(e) => setDocumentId(e.target.value)} />
          <Input
            label="document_version_id"
            value={documentVersionId}
            onChange={(e) => setDocumentVersionId(e.target.value)}
          />
          <Input label="Reason (optional)" value={singleReason} onChange={(e) => setSingleReason(e.target.value)} />
          {!singleScopeValid && (
            <div className="badge">Provide exactly one of document_id or document_version_id.</div>
          )}
          <Button onClick={() => void runSingleReindex()} disabled={!singleScopeValid || singleMutation.isLoading}>
            {singleMutation.isLoading ? 'Submitting...' : 'Trigger Reindex Document'}
          </Button>
          {singleMutation.error && <div className="badge">Error: {singleMutation.error}</div>}
          {renderResult(singleMutation.data)}
        </div>
      </Panel>

      <Panel title="Reindex All">
        <div className="grid">
          <Input
            label="Reason (required)"
            value={allReason}
            onChange={(e) => setAllReason(e.target.value)}
            placeholder="Required by backend for audit logging"
          />
          <div className="grid two">
            <Input
              label="doc_type_code (optional)"
              value={allDocTypeCode}
              onChange={(e) => setAllDocTypeCode(e.target.value)}
            />
            <Input
              label="collection (optional)"
              value={allCollection}
              onChange={(e) => setAllCollection(e.target.value)}
            />
            <Select label="status (optional)" value={allStatus} onChange={(e) => setAllStatus(e.target.value)}>
              <option value="">Any</option>
              <option value="queued">queued</option>
              <option value="pending">pending</option>
              <option value="processing">processing</option>
              <option value="done">done</option>
              <option value="failed">failed</option>
              <option value="never_ingested">never_ingested</option>
            </Select>
            <Input label="limit (max 2000)" type="number" value={allLimit} onChange={(e) => setAllLimit(e.target.value)} />
          </div>
          <label className="badge" style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
            <input type="checkbox" checked={allConfirm} onChange={(e) => setAllConfirm(e.target.checked)} />
            I confirm bulk reindex may enqueue many jobs.
          </label>
          <Button onClick={() => void runAllReindex()} disabled={!allConfirm || !allReason.trim() || allMutation.isLoading}>
            {allMutation.isLoading ? 'Submitting...' : 'Trigger Reindex All'}
          </Button>
          {allMutation.error && <div className="badge">Error: {allMutation.error}</div>}
          {renderResult(allMutation.data)}
        </div>
      </Panel>
    </div>
  );
};

export default ReindexControlsPage;
