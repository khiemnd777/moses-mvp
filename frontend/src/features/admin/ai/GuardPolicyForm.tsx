import { useMemo, useState } from 'react';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import Select from '@/shared/Select';
import type { AIGuardPolicy } from '@/core/types';

const actions = ['refuse', 'fallback_llm', 'ask_clarification'] as const;

type Props = {
  value?: AIGuardPolicy;
  onSubmit: (payload: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  onCancel?: () => void;
};

const GuardPolicyForm = ({ value, onSubmit, onCancel }: Props) => {
  const [name, setName] = useState(value?.name || '');
  const [enabled, setEnabled] = useState(value?.enabled ?? true);
  const [minRetrievedChunks, setMinRetrievedChunks] = useState(value?.min_retrieved_chunks ?? 1);
  const [minSimilarityScore, setMinSimilarityScore] = useState(value?.min_similarity_score ?? 0.7);
  const [onEmptyRetrieval, setOnEmptyRetrieval] =
    useState<AIGuardPolicy['on_empty_retrieval']>(value?.on_empty_retrieval || 'refuse');
  const [onLowConfidence, setOnLowConfidence] =
    useState<AIGuardPolicy['on_low_confidence']>(value?.on_low_confidence || 'ask_clarification');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  const validationError = useMemo(() => {
    if (!name.trim()) return 'Name is required';
    if (minRetrievedChunks < 0) return 'Min retrieved chunks must be >= 0';
    if (minSimilarityScore < 0 || minSimilarityScore > 1) return 'Min similarity score must be between 0 and 1';
    return undefined;
  }, [name, minRetrievedChunks, minSimilarityScore]);

  const handleSubmit = async () => {
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        enabled,
        min_retrieved_chunks: minRetrievedChunks,
        min_similarity_score: minSimilarityScore,
        on_empty_retrieval: onEmptyRetrieval,
        on_low_confidence: onLowConfidence
      });
      setError(undefined);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="grid">
      {error && <div className="badge">{error}</div>}
      <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} />
      <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        <span>Enabled</span>
      </label>
      <Input
        label="Min Retrieved Chunks"
        type="number"
        min={0}
        value={minRetrievedChunks}
        onChange={(e) => setMinRetrievedChunks(Number(e.target.value || 0))}
      />
      <Input
        label="Min Similarity Score"
        type="number"
        min={0}
        max={1}
        step={0.01}
        value={minSimilarityScore}
        onChange={(e) => setMinSimilarityScore(Number(e.target.value || 0))}
      />
      <Select
        label="Empty Retrieval Action"
        value={onEmptyRetrieval}
        onChange={(e) => setOnEmptyRetrieval(e.target.value as AIGuardPolicy['on_empty_retrieval'])}
      >
        {actions.map((action) => (
          <option key={action} value={action}>
            {action}
          </option>
        ))}
      </Select>
      <Select
        label="Low Confidence Action"
        value={onLowConfidence}
        onChange={(e) => setOnLowConfidence(e.target.value as AIGuardPolicy['on_low_confidence'])}
      >
        {actions.map((action) => (
          <option key={action} value={action}>
            {action}
          </option>
        ))}
      </Select>
      <div style={{ display: 'flex', gap: 8 }}>
        <Button onClick={() => void handleSubmit()} disabled={saving}>
          {saving ? 'Saving...' : 'Save'}
        </Button>
        {onCancel && (
          <Button variant="outline" onClick={onCancel} disabled={saving}>
            Cancel
          </Button>
        )}
      </div>
    </div>
  );
};

export default GuardPolicyForm;
