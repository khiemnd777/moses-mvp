import { useEffect, useMemo, useState } from 'react';
import {
  createTelegramBot,
  deleteTelegramBot,
  listTelegramBots,
  startTelegramBot,
  stopTelegramBot,
  unwrapError,
  updateTelegramBot
} from '@/core/api';
import type { TelegramBot, TelegramBotPayload } from '@/core/types';
import Button from '@/shared/Button';
import Panel from '@/shared/Panel';
import { AddIcon, CloseIcon, DeleteIcon, EditIcon, PlayArrowIcon, RefreshIcon, StopIcon } from '@/shared/muiIcons';
import TelegramBotForm from './TelegramBotForm';

const formatDate = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

const statusClass = (status: TelegramBot['status']) => {
  if (status === 'running') return 'status-ok';
  if (status === 'error') return 'status-warn';
  return '';
};

const TelegramBotsPage = () => {
  const [items, setItems] = useState<TelegramBot[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [creating, setCreating] = useState(false);
  const [editingId, setEditingId] = useState<string>();
  const [actionId, setActionId] = useState<string>();

  const fetchItems = async () => {
    setLoading(true);
    try {
      setItems(await listTelegramBots());
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

  const editingItem = useMemo(() => items.find((item) => item.id === editingId), [editingId, items]);

  const handleCreate = async (payload: TelegramBotPayload) => {
    try {
      await createTelegramBot(payload);
      setCreating(false);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const handleUpdate = async (id: string, payload: TelegramBotPayload) => {
    try {
      await updateTelegramBot(id, payload);
      setEditingId(undefined);
      await fetchItems();
    } catch (err) {
      throw new Error(unwrapError(err));
    }
  };

  const runAction = async (id: string, action: () => Promise<unknown>) => {
    setActionId(id);
    try {
      await action();
      await fetchItems();
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setActionId(undefined);
    }
  };

  const handleDelete = async (item: TelegramBot) => {
    if (!window.confirm(`Delete Telegram bot "${item.name}"?`)) return;
    await runAction(item.id, () => deleteTelegramBot(item.id));
  };

  return (
    <>
      <Panel title="Telegram Bots">
        <div className="grid">
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button onClick={() => setCreating((prev) => !prev)}>
              {creating ? <CloseIcon aria-hidden="true" /> : <AddIcon aria-hidden="true" />}
              {creating ? 'Close Create' : 'Create'}
            </Button>
            <Button variant="outline" onClick={() => void fetchItems()} disabled={loading}>
              <RefreshIcon aria-hidden="true" />
              Refresh
            </Button>
          </div>
          {loading && <div className="badge">Loading...</div>}
          {error && <div className="badge status-warn">{error}</div>}
          {creating && <TelegramBotForm onSubmit={handleCreate} onCancel={() => setCreating(false)} />}
          <div className="grid">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Bot</th>
                  <th>Status</th>
                  <th>Default</th>
                  <th>Chats</th>
                  <th>Last error</th>
                  <th>Updated At</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{item.name}</td>
                    <td>{item.bot_username ? `@${item.bot_username}` : item.token_hint || '-'}</td>
                    <td>
                      <span className={`badge ${statusClass(item.status)}`.trim()}>{item.status}</span>
                    </td>
                    <td>
                      {item.default_tone} / top {item.default_top_k}
                    </td>
                    <td>{item.chat_count}</td>
                    <td>{item.last_error || '-'}</td>
                    <td>{formatDate(item.updated_at)}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                        <Button variant="outline" onClick={() => setEditingId(item.id)} disabled={actionId === item.id}>
                          <EditIcon aria-hidden="true" />
                          Edit
                        </Button>
                        {item.status === 'running' ? (
                          <Button
                            variant="outline"
                            onClick={() => void runAction(item.id, () => stopTelegramBot(item.id))}
                            disabled={actionId === item.id}
                          >
                            <StopIcon aria-hidden="true" />
                            Stop
                          </Button>
                        ) : (
                          <Button
                            variant="outline"
                            onClick={() => void runAction(item.id, () => startTelegramBot(item.id))}
                            disabled={actionId === item.id}
                          >
                            <PlayArrowIcon aria-hidden="true" />
                            Start
                          </Button>
                        )}
                        <Button variant="outline" onClick={() => void handleDelete(item)} disabled={actionId === item.id}>
                          <DeleteIcon aria-hidden="true" />
                          Delete
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
                {!loading && items.length === 0 && (
                  <tr>
                    <td colSpan={8}>No Telegram bots configured.</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      </Panel>
      {editingItem && (
        <Panel title={`Edit Telegram Bot: ${editingItem.name}`}>
          <TelegramBotForm
            value={editingItem}
            onSubmit={(payload) => handleUpdate(editingItem.id, payload)}
            onCancel={() => setEditingId(undefined)}
          />
        </Panel>
      )}
    </>
  );
};

export default TelegramBotsPage;
