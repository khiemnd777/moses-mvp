import { useEffect, useMemo, useState } from 'react';
import {
  createAIGuardPolicy,
  deleteAIGuardPolicy,
  listAIGuardPolicies,
  unwrapError,
  updateAIGuardPolicy
} from '@/core/api';
import type { AIGuardPolicy } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import GuardPolicyForm from './GuardPolicyForm';

const emptyPolicy: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'> = {
  name: '',
  enabled: true,
  min_retrieved_chunks: 1,
  min_similarity_score: 0.7,
  on_empty_retrieval: 'refuse',
  on_low_confidence: 'ask_clarification'
};

const GuardPoliciesPage = () => {
  const [items, setItems] = useState<AIGuardPolicy[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [editingId, setEditingId] = useState<string>();
  const [creating, setCreating] = useState(false);

  const fetchItems = async () => {
    setLoading(true);
    try {
      setItems(await listAIGuardPolicies());
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

  const handleCreate = async (payload: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await createAIGuardPolicy(payload);
      setCreating(false);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleUpdate = async (id: string, payload: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await updateAIGuardPolicy(id, payload);
      setEditingId(undefined);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Delete this guard policy?')) return;
    try {
      await deleteAIGuardPolicy(id);
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleToggle = async (item: AIGuardPolicy, enabled: boolean) => {
    try {
      await updateAIGuardPolicy(item.id, {
        name: item.name,
        enabled,
        min_retrieved_chunks: item.min_retrieved_chunks,
        min_similarity_score: item.min_similarity_score,
        on_empty_retrieval: item.on_empty_retrieval,
        on_low_confidence: item.on_low_confidence
      });
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  return (
    <>
      <Panel title="AI Guard Policies">
        <div className="grid">
          {loading && <div className="badge">Loading...</div>}
          {error && <div className="badge">{error}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => setCreating((prev) => !prev)}>{creating ? 'Close Create' : 'Create'}</Button>
          </div>
          {creating && (
            <GuardPolicyForm
              value={{ ...emptyPolicy, id: 'new' } as AIGuardPolicy}
              onSubmit={handleCreate}
              onCancel={() => setCreating(false)}
            />
          )}
          <div className="grid">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Enabled</th>
                  <th>Min Chunks</th>
                  <th>Min Similarity</th>
                  <th>Empty Action</th>
                  <th>Low Confidence</th>
                  <th>Updated At</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>{item.enabled ? 'Yes' : 'No'}</td>
                    <td>{item.min_retrieved_chunks}</td>
                    <td>{item.min_similarity_score}</td>
                    <td>{item.on_empty_retrieval}</td>
                    <td>{item.on_low_confidence}</td>
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
        <Panel title={`Edit Guard Policy: ${editingItem.name}`}>
          <GuardPolicyForm
            value={editingItem}
            onSubmit={(payload) => handleUpdate(editingItem.id, payload)}
            onCancel={() => setEditingId(undefined)}
          />
        </Panel>
      )}
    </>
  );
};

export default GuardPoliciesPage;
