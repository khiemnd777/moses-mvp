import { useMemo, useState } from 'react';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import type { AIPrompt, Citation } from '@/core/types';
import { testAIPrompt } from '@/core/api';

type Props = {
  value?: AIPrompt;
  onSubmit: (payload: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  onCancel?: () => void;
};

const PromptForm = ({ value, onSubmit, onCancel }: Props) => {
  const [name, setName] = useState(value?.name || '');
  const [promptType, setPromptType] = useState(value?.prompt_type || 'legal_guard');
  const [systemPrompt, setSystemPrompt] = useState(value?.system_prompt || '');
  const [temperature, setTemperature] = useState(value?.temperature ?? 0.2);
  const [maxTokens, setMaxTokens] = useState(value?.max_tokens ?? 1200);
  const [retry, setRetry] = useState(value?.retry ?? 2);
  const [enabled, setEnabled] = useState(value?.enabled ?? true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  const [testQuery, setTestQuery] = useState('');
  const [testLoading, setTestLoading] = useState(false);
  const [testAnswer, setTestAnswer] = useState('');
  const [testCitations, setTestCitations] = useState<Citation[]>([]);

  const validationError = useMemo(() => {
    if (!name.trim()) return 'Name is required';
    if (!promptType.trim()) return 'Prompt type is required';
    if (!systemPrompt.trim()) return 'System prompt is required';
    if (temperature < 0 || temperature > 1) return 'Temperature must be between 0 and 1';
    if (maxTokens < 1 || maxTokens > 8000) return 'Max tokens must be between 1 and 8000';
    if (retry < 0 || retry > 5) return 'Retry must be between 0 and 5';
    return undefined;
  }, [name, promptType, systemPrompt, temperature, maxTokens, retry]);

  const handleSubmit = async () => {
    if (validationError) {
      setError(validationError);
      return;
    }
    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        prompt_type: promptType.trim(),
        system_prompt: systemPrompt,
        temperature,
        max_tokens: maxTokens,
        retry,
        enabled
      });
      setError(undefined);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const handleTestPrompt = async () => {
    if (!value?.id || !testQuery.trim()) return;
    setTestLoading(true);
    try {
      const result = await testAIPrompt({ prompt_id: value.id, query: testQuery.trim(), top_k: 5 });
      setTestAnswer(result.answer || '');
      setTestCitations((result.citations || []) as Citation[]);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setTestLoading(false);
    }
  };

  return (
    <div className="grid">
      {error && <div className="badge">{error}</div>}
      <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} />
      <Input label="Prompt Type" value={promptType} onChange={(e) => setPromptType(e.target.value)} />
      <label>
        <div className="label">System Prompt</div>
        <textarea className="textarea" rows={10} value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} />
      </label>
      <Input
        label="Temperature"
        type="number"
        min={0}
        max={1}
        step={0.01}
        value={temperature}
        onChange={(e) => setTemperature(Number(e.target.value || 0))}
      />
      <Input
        label="Max Tokens"
        type="number"
        min={1}
        max={8000}
        value={maxTokens}
        onChange={(e) => setMaxTokens(Number(e.target.value || 1))}
      />
      <Input
        label="Retry"
        type="number"
        min={0}
        max={5}
        value={retry}
        onChange={(e) => setRetry(Number(e.target.value || 0))}
      />
      <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
        <span>Enabled</span>
      </label>
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

      {value?.id && (
        <div className="grid">
          <div className="label">Prompt Test</div>
          <textarea
            className="textarea"
            rows={3}
            value={testQuery}
            onChange={(e) => setTestQuery(e.target.value)}
            placeholder="Enter test query"
          />
          <Button variant="outline" onClick={() => void handleTestPrompt()} disabled={testLoading || !testQuery.trim()}>
            {testLoading ? 'Testing...' : 'Run Test'}
          </Button>
          {testAnswer && (
            <pre className="source-item" style={{ whiteSpace: 'pre-wrap' }}>
              {testAnswer}
            </pre>
          )}
          {testCitations.length > 0 && (
            <pre className="source-item" style={{ whiteSpace: 'pre-wrap' }}>
              {JSON.stringify(testCitations, null, 2)}
            </pre>
          )}
        </div>
      )}
    </div>
  );
};

export default PromptForm;
