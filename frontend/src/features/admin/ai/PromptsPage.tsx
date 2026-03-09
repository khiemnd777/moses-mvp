import { useEffect, useMemo, useState } from 'react';
import {
  createAIPrompt,
  deleteAIPrompt,
  listAIPrompts,
  unwrapError,
  updateAIPrompt
} from '@/core/api';
import type { AIPrompt } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import PromptForm from './PromptForm';

const emptyPrompt: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'> = {
  name: '',
  prompt_type: 'legal_guard',
  system_prompt: '',
  temperature: 0.2,
  max_tokens: 1200,
  retry: 2,
  enabled: true
};

const PromptsPage = () => {
  const [items, setItems] = useState<AIPrompt[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [editingId, setEditingId] = useState<string>();
  const [creating, setCreating] = useState(false);

  const fetchItems = async () => {
    setLoading(true);
    try {
      setItems(await listAIPrompts());
      setError(undefined);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchItems();
  }, []);

  const editingItem = useMemo(() => items.find((item) => item.id === editingId), [items, editingId]);

  const handleCreate = async (payload: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await createAIPrompt(payload);
      setCreating(false);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleUpdate = async (id: string, payload: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await updateAIPrompt(id, payload);
      setEditingId(undefined);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Delete this prompt?')) return;
    try {
      await deleteAIPrompt(id);
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleToggle = async (item: AIPrompt, enabled: boolean) => {
    try {
      await updateAIPrompt(item.id, {
        name: item.name,
        prompt_type: item.prompt_type,
        system_prompt: item.system_prompt,
        temperature: item.temperature,
        max_tokens: item.max_tokens,
        retry: item.retry,
        enabled
      });
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  return (
    <>
      <Panel title="AI Prompts">
        <div className="grid">
          {loading && <div className="badge">Loading...</div>}
          {error && <div className="badge">{error}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => setCreating((prev) => !prev)}>{creating ? 'Close Create' : 'Create'}</Button>
          </div>
          {creating && (
            <PromptForm value={{ ...emptyPrompt, id: 'new' } as AIPrompt} onSubmit={handleCreate} onCancel={() => setCreating(false)} />
          )}
          <div className="grid">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Prompt Type</th>
                  <th>Temperature</th>
                  <th>Max Tokens</th>
                  <th>Retry</th>
                  <th>Enabled</th>
                  <th>Updated At</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>{item.prompt_type}</td>
                    <td>{item.temperature}</td>
                    <td>{item.max_tokens}</td>
                    <td>{item.retry}</td>
                    <td>{item.enabled ? 'Yes' : 'No'}</td>
                    <td>{item.updated_at || '-'}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 6 }}>
                        <Button variant="outline" onClick={() => setEditingId(item.id)}>
                          Edit
                        </Button>
                        <Button variant="outline" onClick={() => void handleToggle(item, !item.enabled)}>
                          {item.enabled ? 'Disable' : 'Enable'}
                        </Button>
                        <Button variant="outline" onClick={() => void handleDelete(item.id)}>
                          Delete
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </Panel>
      {editingItem && (
        <Panel title={`Edit Prompt: ${editingItem.name}`}>
          <PromptForm
            value={editingItem}
            onSubmit={(payload) => handleUpdate(editingItem.id, payload)}
            onCancel={() => setEditingId(undefined)}
          />
        </Panel>
      )}
    </>
  );
};

export default PromptsPage;
