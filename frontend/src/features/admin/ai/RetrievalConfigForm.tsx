import { useMemo, useState } from 'react';
import type { AIRetrievalConfig } from '@/core/types';
import Button from '@/shared/Button';

type Props = {
  value: AIRetrievalConfig;
  onSubmit: (payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  onCancel: () => void;
};

const parsePreferredDocTypes = (input: string): string[] => {
  return input
    .split(',')
    .map((v) => v.trim().toLowerCase())
    .filter(Boolean);
};

const RetrievalConfigForm = ({ value, onSubmit, onCancel }: Props) => {
  const [name, setName] = useState(value.name || '');
  const [enabled, setEnabled] = useState(Boolean(value.enabled));
  const [defaultTopK, setDefaultTopK] = useState(value.default_top_k || 5);
  const [maxContextChunks, setMaxContextChunks] = useState(value.max_context_chunks || 12);
  const [maxContextChars, setMaxContextChars] = useState(value.max_context_chars || 12000);
  const [rerankEnabled, setRerankEnabled] = useState(Boolean(value.rerank_enabled));
  const [vectorWeight, setVectorWeight] = useState(value.rerank_vector_weight ?? 0.55);
  const [keywordWeight, setKeywordWeight] = useState(value.rerank_keyword_weight ?? 0.25);
  const [metadataWeight, setMetadataWeight] = useState(value.rerank_metadata_weight ?? 0.15);
  const [articleWeight, setArticleWeight] = useState(value.rerank_article_weight ?? 0.05);
  const [adjacentEnabled, setAdjacentEnabled] = useState(Boolean(value.adjacent_chunk_enabled));
  const [adjacentWindow, setAdjacentWindow] = useState(value.adjacent_chunk_window ?? 1);
  const [defaultEffectiveStatus, setDefaultEffectiveStatus] = useState(value.default_effective_status || 'active');
  const [preferredDocTypesInput, setPreferredDocTypesInput] = useState((value.preferred_doc_types_json || []).join(', '));
  const [legalDefaultsInput, setLegalDefaultsInput] = useState(
    JSON.stringify(value.legal_domain_defaults_json || {}, null, 2)
  );
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);

  const weightSum = useMemo(() => vectorWeight + keywordWeight + metadataWeight + articleWeight, [
    vectorWeight,
    keywordWeight,
    metadataWeight,
    articleWeight
  ]);

  const handleSubmit = async () => {
    setError(undefined);
    let legalDefaultsJson: Record<string, unknown> = {};
    try {
      legalDefaultsJson = JSON.parse(legalDefaultsInput || '{}') as Record<string, unknown>;
    } catch {
      setError('legal_domain_defaults_json must be valid JSON');
      return;
    }

    const payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'> = {
      name,
      enabled,
      default_top_k: Number(defaultTopK),
      rerank_enabled: rerankEnabled,
      rerank_vector_weight: Number(vectorWeight),
      rerank_keyword_weight: Number(keywordWeight),
      rerank_metadata_weight: Number(metadataWeight),
      rerank_article_weight: Number(articleWeight),
      adjacent_chunk_enabled: adjacentEnabled,
      adjacent_chunk_window: Number(adjacentWindow),
      max_context_chunks: Number(maxContextChunks),
      max_context_chars: Number(maxContextChars),
      default_effective_status: defaultEffectiveStatus,
      preferred_doc_types_json: parsePreferredDocTypes(preferredDocTypesInput),
      legal_domain_defaults_json: legalDefaultsJson
    };

    try {
      setSaving(true);
      await onSubmit(payload);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save retrieval config');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="grid" style={{ gap: 14 }}>
      {error && <div className="badge">{error}</div>}

      <h3>Basic Retrieval</h3>
      <label>
        <div className="label">Name</div>
        <input className="input" value={name} onChange={(e) => setName(e.target.value)} />
      </label>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} /> Enabled
      </label>
      <label>
        <div className="label">Default Top K (1-20)</div>
        <input className="input" type="number" min={1} max={20} value={defaultTopK} onChange={(e) => setDefaultTopK(Number(e.target.value))} />
      </label>
      <label>
        <div className="label">Max Context Chunks (1-20)</div>
        <input className="input" type="number" min={1} max={20} value={maxContextChunks} onChange={(e) => setMaxContextChunks(Number(e.target.value))} />
      </label>
      <label>
        <div className="label">Max Context Characters (1000-200000)</div>
        <input
          className="input"
          type="number"
          min={1000}
          max={200000}
          value={maxContextChars}
          onChange={(e) => setMaxContextChars(Number(e.target.value))}
        />
      </label>

      <h3>Reranking</h3>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <input type="checkbox" checked={rerankEnabled} onChange={(e) => setRerankEnabled(e.target.checked)} /> Enable reranking
      </label>
      <label>
        <div className="label">Vector Weight (0-1)</div>
        <input className="input" type="number" min={0} max={1} step="0.01" value={vectorWeight} onChange={(e) => setVectorWeight(Number(e.target.value))} />
      </label>
      <label>
        <div className="label">Keyword Weight (0-1)</div>
        <input className="input" type="number" min={0} max={1} step="0.01" value={keywordWeight} onChange={(e) => setKeywordWeight(Number(e.target.value))} />
      </label>
      <label>
        <div className="label">Metadata Weight (0-1)</div>
        <input className="input" type="number" min={0} max={1} step="0.01" value={metadataWeight} onChange={(e) => setMetadataWeight(Number(e.target.value))} />
      </label>
      <label>
        <div className="label">Article Weight (0-1)</div>
        <input className="input" type="number" min={0} max={1} step="0.01" value={articleWeight} onChange={(e) => setArticleWeight(Number(e.target.value))} />
      </label>
      <div className="label">Weight Sum: {weightSum.toFixed(2)}</div>

      <h3>Adjacent Chunk Expansion</h3>
      <label style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
        <input type="checkbox" checked={adjacentEnabled} onChange={(e) => setAdjacentEnabled(e.target.checked)} /> Enable adjacent expansion
      </label>
      <label>
        <div className="label">Adjacent Window (0-3)</div>
        <input className="input" type="number" min={0} max={3} value={adjacentWindow} onChange={(e) => setAdjacentWindow(Number(e.target.value))} />
      </label>

      <h3>Domain Defaults</h3>
      <label>
        <div className="label">Default Effective Status</div>
        <input className="input" value={defaultEffectiveStatus} onChange={(e) => setDefaultEffectiveStatus(e.target.value)} />
      </label>
      <label>
        <div className="label">Preferred Document Types (comma separated)</div>
        <input
          className="input"
          placeholder="law, resolution, circular, decree"
          value={preferredDocTypesInput}
          onChange={(e) => setPreferredDocTypesInput(e.target.value)}
        />
      </label>

      <h3>Legal Domain Defaults</h3>
      <label>
        <div className="label">legal_domain_defaults_json</div>
        <textarea
          className="textarea"
          rows={10}
          value={legalDefaultsInput}
          onChange={(e) => setLegalDefaultsInput(e.target.value)}
        />
      </label>

      <div style={{ display: 'flex', gap: 8 }}>
        <Button onClick={() => void handleSubmit()} disabled={saving}>
          {saving ? 'Saving...' : 'Save'}
        </Button>
        <Button variant="outline" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  );
};

export default RetrievalConfigForm;
