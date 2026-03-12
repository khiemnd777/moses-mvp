import { useMemo, useState } from 'react';
import { searchDebugQdrant } from '@/core/api';
import type { SearchDebugRequest, SearchDebugResponse } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import { useMutationAction } from './useAdminApi';

const parseCsv = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

const formatPayload = (payload?: Record<string, unknown>) => {
  if (!payload) return '-';
  return JSON.stringify(payload, null, 2);
};

const SearchDebugPage = () => {
  const [queryText, setQueryText] = useState('');
  const [topK, setTopK] = useState('10');
  const [collection, setCollection] = useState('');
  const [includePayload, setIncludePayload] = useState(true);
  const [includeChunkPreview, setIncludeChunkPreview] = useState(true);
  const [legalDomain, setLegalDomain] = useState('');
  const [documentType, setDocumentType] = useState('');
  const [effectiveStatus, setEffectiveStatus] = useState('');
  const [documentNumber, setDocumentNumber] = useState('');
  const [articleNumber, setArticleNumber] = useState('');
  const [copiedChunk, setCopiedChunk] = useState<string | null>(null);
  const { mutate, data, error, isLoading } = useMutationAction<SearchDebugRequest, SearchDebugResponse>(searchDebugQdrant);

  const filterPayload = useMemo(() => {
    const filters = {
      legal_domain: parseCsv(legalDomain),
      document_type: parseCsv(documentType),
      effective_status: parseCsv(effectiveStatus),
      document_number: parseCsv(documentNumber),
      article_number: parseCsv(articleNumber)
    };
    const hasAnyFilter = Object.values(filters).some((value) => value.length > 0);
    return hasAnyFilter ? filters : undefined;
  }, [articleNumber, documentNumber, documentType, effectiveStatus, legalDomain]);

  const handleRun = async () => {
    await mutate({
      query_text: queryText.trim(),
      top_k: Number(topK) || 10,
      collection: collection.trim() || undefined,
      metadata_filters: filterPayload,
      include_payload: includePayload,
      include_chunk_preview: includeChunkPreview
    });
  };

  const copyChunkId = async (chunkId?: string) => {
    if (!chunkId) return;
    await navigator.clipboard.writeText(chunkId);
    setCopiedChunk(chunkId);
    setTimeout(() => setCopiedChunk(null), 1200);
  };

  return (
    <Panel title="Search Debug">
      <div className="grid">
        <div className="grid two">
          <Input label="Query Text" value={queryText} onChange={(e) => setQueryText(e.target.value)} />
          <Input label="Top K (1-50)" type="number" value={topK} onChange={(e) => setTopK(e.target.value)} min={1} max={50} />
        </div>
        <Input label="Collection (optional)" value={collection} onChange={(e) => setCollection(e.target.value)} />
        <div className="grid two">
          <Input label="legal_domain (csv)" value={legalDomain} onChange={(e) => setLegalDomain(e.target.value)} />
          <Input label="document_type (csv)" value={documentType} onChange={(e) => setDocumentType(e.target.value)} />
          <Input label="effective_status (csv)" value={effectiveStatus} onChange={(e) => setEffectiveStatus(e.target.value)} />
          <Input label="document_number (csv)" value={documentNumber} onChange={(e) => setDocumentNumber(e.target.value)} />
          <Input label="article_number (csv)" value={articleNumber} onChange={(e) => setArticleNumber(e.target.value)} />
        </div>
        <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
          <label className="badge">
            <input
              type="checkbox"
              checked={includePayload}
              onChange={(e) => setIncludePayload(e.target.checked)}
              style={{ marginRight: 6 }}
            />
            Include payload
          </label>
          <label className="badge">
            <input
              type="checkbox"
              checked={includeChunkPreview}
              onChange={(e) => setIncludeChunkPreview(e.target.checked)}
              style={{ marginRight: 6 }}
            />
            Include chunk preview
          </label>
        </div>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <Button onClick={() => void handleRun()} disabled={isLoading || !queryText.trim()}>
            {isLoading ? 'Searching...' : 'Run Search'}
          </Button>
          {error && <div className="badge">Error: {error}</div>}
          {data && <div className="badge">{data.summary}</div>}
        </div>
        {data && (
          <>
            <div className="vector-meta-grid">
              <div className="badge">Collection: {data.collection}</div>
              <div className="badge">Hits: {data.hit_count}</div>
              <div className="badge">Top K: {data.top_k}</div>
              <div className="badge">Duration: {data.duration_ms}ms</div>
              <div className="badge">Filter: {data.filter_summary}</div>
              <div className="badge">Query Hash: {data.query_hash}</div>
            </div>
            <div className="grid">
              {data.hits.map((hit) => (
                <div key={`${hit.point_id}-${hit.rank}`} className="source-item">
                  <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
                    <strong>#{hit.rank}</strong>
                    <div className="badge">Point: {hit.point_id}</div>
                    <div className="badge">Score: {hit.score.toFixed(6)}</div>
                    {hit.chunk?.chunk_id && (
                      <Button variant="outline" onClick={() => void copyChunkId(hit.chunk?.chunk_id)}>
                        {copiedChunk === hit.chunk.chunk_id ? 'Copied' : 'Copy chunk_id'}
                      </Button>
                    )}
                  </div>
                  <div className="vector-meta-grid">
                    <div className="badge">chunk_id: {hit.chunk?.chunk_id || '-'}</div>
                    <div className="badge">document_version_id: {hit.chunk?.document_version_id || '-'}</div>
                    <div className="badge">chunk_index: {hit.chunk?.chunk_index ?? '-'}</div>
                    <div className="badge">citation: {hit.chunk?.citation || '-'}</div>
                  </div>
                  <div className="label" style={{ marginTop: 10 }}>
                    Content Preview
                  </div>
                  <pre className="vector-pre">{hit.chunk?.preview || '(no preview)'}</pre>
                  <div className="label" style={{ marginTop: 10 }}>
                    Payload Metadata
                  </div>
                  <pre className="vector-pre">{formatPayload(hit.payload)}</pre>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </Panel>
  );
};

export default SearchDebugPage;
