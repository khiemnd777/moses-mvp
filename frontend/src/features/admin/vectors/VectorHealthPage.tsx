import { useState } from 'react';
import { getQdrantVectorHealth } from '@/core/api';
import type { VectorHealthResponse } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import { useMutationAction } from './useAdminApi';

const HealthCard = ({ label, value, tone = 'neutral' }: { label: string; value: string | number; tone?: 'neutral' | 'ok' | 'warn' }) => (
  <div className={`source-item health-card ${tone}`}>
    <div className="label">{label}</div>
    <div style={{ fontWeight: 700, fontSize: 20 }}>{value}</div>
  </div>
);

const VectorHealthPage = () => {
  const [maxVectors, setMaxVectors] = useState('1000');
  const [maxDurationMs, setMaxDurationMs] = useState('8000');
  const { mutate, data, error, isLoading } = useMutationAction<
    { mode: 'quick' | 'full'; max_vectors_scanned?: number; max_scan_duration_ms?: number },
    VectorHealthResponse
  >(getQdrantVectorHealth);

  const runQuickScan = async () => {
    await mutate({
      mode: 'quick',
      max_vectors_scanned: Number(maxVectors) || undefined,
      max_scan_duration_ms: Number(maxDurationMs) || undefined
    });
  };

  const runFullScan = async () => {
    const confirmed = window.confirm('Full scan is heavier and slower. Continue?');
    if (!confirmed) return;
    await mutate({
      mode: 'full',
      max_vectors_scanned: Number(maxVectors) || undefined,
      max_scan_duration_ms: Number(maxDurationMs) || undefined
    });
  };

  const hasIssues = Boolean(
    data &&
      (data.missing_vectors_count > 0 ||
        data.orphan_vectors_count > 0 ||
        data.dimension_mismatch_detected ||
        data.chunk_vector_count_mismatch)
  );

  return (
    <Panel title="Vector Health">
      <div className="grid">
        <div className="grid two">
          <Input
            label="max_vectors_scanned"
            type="number"
            min={1}
            value={maxVectors}
            onChange={(e) => setMaxVectors(e.target.value)}
          />
          <Input
            label="max_scan_duration_ms"
            type="number"
            min={100}
            value={maxDurationMs}
            onChange={(e) => setMaxDurationMs(e.target.value)}
          />
        </div>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <Button onClick={() => void runQuickScan()} disabled={isLoading}>
            {isLoading ? 'Scanning...' : 'Quick Scan'}
          </Button>
          <Button variant="outline" onClick={() => void runFullScan()} disabled={isLoading}>
            Full Scan (Warning)
          </Button>
        </div>
        {error && <div className="badge">Error: {error}</div>}
        {data && (
          <>
            <div className={`badge ${hasIssues ? 'status-warn' : 'status-ok'}`}>
              Status: {hasIssues ? 'Needs attention' : 'Healthy'} | Mode: {data.scan_mode}
            </div>
            <div className="grid two">
              <HealthCard label="Total Scanned Vectors" value={data.scanned_vectors} />
              <HealthCard label="Total Scanned Chunks" value={data.scanned_chunks} />
              <HealthCard
                label="Missing Vectors"
                value={data.missing_vectors_count}
                tone={data.missing_vectors_count > 0 ? 'warn' : 'ok'}
              />
              <HealthCard
                label="Orphan Vectors"
                value={data.orphan_vectors_count}
                tone={data.orphan_vectors_count > 0 ? 'warn' : 'ok'}
              />
              <HealthCard label="Scanned Batches" value={data.scanned_batches} />
              <HealthCard label="Duration (ms)" value={data.duration_ms} />
            </div>
            <div className="vector-meta-grid">
              <div className={`badge ${data.chunk_vector_count_mismatch ? 'status-warn' : 'status-ok'}`}>
                chunk/vector mismatch: {data.chunk_vector_count_mismatch ? 'yes' : 'no'}
              </div>
              <div className={`badge ${data.dimension_mismatch_detected ? 'status-warn' : 'status-ok'}`}>
                dimension mismatch: {data.dimension_mismatch_detected ? 'yes' : 'no'}
              </div>
              <div className={`badge ${data.repairable_issues_detected ? 'status-warn' : 'status-ok'}`}>
                repairable issues: {data.repairable_issues_detected ? 'yes' : 'no'}
              </div>
              <div className="badge">bounded: {data.bounded ? 'yes' : 'no'}</div>
            </div>
            <div className="badge">Recommendation: {data.repair_recommendation}</div>
            {(data.samples || []).length > 0 && (
              <div className="source-item">
                <div className="label">Sample IDs</div>
                <pre className="vector-pre">{(data.samples || []).join('\n')}</pre>
              </div>
            )}
          </>
        )}
      </div>
    </Panel>
  );
};

export default VectorHealthPage;
