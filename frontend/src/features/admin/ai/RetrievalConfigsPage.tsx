import { useEffect, useMemo, useState } from 'react';
import {
  createAIRetrievalConfig,
  deleteAIRetrievalConfig,
  disableAIRetrievalConfig,
  enableAIRetrievalConfig,
  listAIRetrievalConfigs,
  search,
  unwrapError,
  updateAIRetrievalConfig
} from '@/core/api';
import type { AIRetrievalConfig } from '@/core/types';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import RetrievalConfigForm from './RetrievalConfigForm';

const emptyConfig: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'> = {
  name: '',
  enabled: true,
  default_top_k: 5,
  rerank_enabled: true,
  rerank_vector_weight: 0.55,
  rerank_keyword_weight: 0.25,
  rerank_metadata_weight: 0.15,
  rerank_article_weight: 0.05,
  adjacent_chunk_enabled: true,
  adjacent_chunk_window: 1,
  max_context_chunks: 12,
  max_context_chars: 12000,
  default_effective_status: 'active',
  preferred_doc_types_json: ['law', 'resolution', 'decree'],
  legal_domain_defaults_json: {
    marriage_family: { top_k: 6, preferred_doc_types: ['law', 'resolution'] },
    criminal_law: { top_k: 8 }
  }
};

const RetrievalConfigsPage = () => {
  const [items, setItems] = useState<AIRetrievalConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [editingId, setEditingId] = useState<string>();
  const [creating, setCreating] = useState(false);

  const [testQuery, setTestQuery] = useState('');
  const [testOutput, setTestOutput] = useState('');

  const fetchItems = async () => {
    setLoading(true);
    try {
      setItems(await listAIRetrievalConfigs());
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

  const handleCreate = async (payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await createAIRetrievalConfig(payload);
      setCreating(false);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleUpdate = async (id: string, payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'>) => {
    try {
      await updateAIRetrievalConfig(id, payload);
      setEditingId(undefined);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm('Delete this retrieval config?')) return;
    try {
      await deleteAIRetrievalConfig(id);
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleEnable = async (id: string) => {
    try {
      await enableAIRetrievalConfig(id);
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const handleDisable = async (id: string) => {
    try {
      await disableAIRetrievalConfig(id);
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    }
  };

  const runRetrievalTest = async () => {
    if (!testQuery.trim()) return;
    try {
      const data = await search(testQuery.trim());
      setTestOutput(JSON.stringify(data, null, 2));
    } catch (err) {
      setTestOutput(unwrapError(err));
    }
  };

  return (
    <>
      <Panel title="AI Retrieval Configs">
        <div className="grid">
          {loading && <div className="badge">Loading...</div>}
          {error && <div className="badge">{error}</div>}
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => setCreating((prev) => !prev)}>{creating ? 'Close Create' : 'Create'}</Button>
          </div>

          {creating && (
            <RetrievalConfigForm
              value={{ ...emptyConfig, id: 'new' } as AIRetrievalConfig}
              onSubmit={handleCreate}
              onCancel={() => setCreating(false)}
            />
          )}

          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Enabled</th>
                <th>Top K</th>
                <th>Rerank Enabled</th>
                <th>Adjacent Window</th>
                <th>Max Context Chunks</th>
                <th>Updated At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.enabled ? 'Yes' : 'No'}</td>
                  <td>{item.default_top_k}</td>
                  <td>{item.rerank_enabled ? 'Yes' : 'No'}</td>
                  <td>{item.adjacent_chunk_enabled ? item.adjacent_chunk_window : 'Disabled'}</td>
                  <td>{item.max_context_chunks}</td>
                  <td>{item.updated_at || '-'}</td>
                  <td>
                    <div style={{ display: 'flex', gap: 6 }}>
                      <Button variant="outline" onClick={() => setEditingId(item.id)}>
                        Edit
                      </Button>
                      <Button variant="outline" onClick={() => void (item.enabled ? handleDisable(item.id) : handleEnable(item.id))}>
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
      </Panel>

      {editingItem && (
        <Panel title={`Edit Retrieval Config: ${editingItem.name}`}>
          <RetrievalConfigForm
            value={editingItem}
            onSubmit={(payload) => handleUpdate(editingItem.id, payload)}
            onCancel={() => setEditingId(undefined)}
          />
        </Panel>
      )}

      <Panel title="Retrieval Test Tool (Optional)">
        <div className="grid">
          <label>
            <div className="label">Test Query</div>
            <textarea className="textarea" rows={3} value={testQuery} onChange={(e) => setTestQuery(e.target.value)} />
          </label>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={() => void runRetrievalTest()}>Run Test</Button>
          </div>
          {testOutput && (
            <pre className="source-item" style={{ whiteSpace: 'pre-wrap' }}>
              {testOutput}
            </pre>
          )}
        </div>
      </Panel>
    </>
  );
};

export default RetrievalConfigsPage;
