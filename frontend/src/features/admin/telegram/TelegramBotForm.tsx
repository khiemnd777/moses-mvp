import { useMemo, useState } from 'react';
import type { TelegramBot, TelegramBotPayload } from '@/core/types';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import Select from '@/shared/Select';
import { CloseIcon, SaveIcon } from '@/shared/muiIcons';

type Props = {
  value?: TelegramBot;
  onSubmit: (payload: TelegramBotPayload) => Promise<void>;
  onCancel?: () => void;
};

const parseAllowedChatIds = (value: string) => {
  const parts = value
    .split(/[\s,]+/)
    .map((part) => part.trim())
    .filter(Boolean);
  const parsed = parts.map((part) => Number(part));
  if (parsed.some((item) => !Number.isSafeInteger(item) || item === 0)) {
    throw new Error('Allowed chat IDs must be comma-separated integers.');
  }
  return Array.from(new Set(parsed));
};

const formatAllowedChatIds = (value?: number[]) => (value || []).join(', ');

const TelegramBotForm = ({ value, onSubmit, onCancel }: Props) => {
  const isEditing = Boolean(value?.id);
  const [name, setName] = useState(value?.name || '');
  const [token, setToken] = useState('');
  const [defaultTone, setDefaultTone] = useState(value?.default_tone || 'default');
  const [defaultTopK, setDefaultTopK] = useState(value?.default_top_k || 5);
  const [defaultEffectiveStatus, setDefaultEffectiveStatus] = useState(value?.default_effective_status || 'active');
  const [defaultDomain, setDefaultDomain] = useState(value?.default_domain || '');
  const [defaultDocType, setDefaultDocType] = useState(value?.default_doc_type || '');
  const [allowedChatIds, setAllowedChatIds] = useState(formatAllowedChatIds(value?.allowed_chat_ids));
  const [welcomeMessage, setWelcomeMessage] = useState(value?.welcome_message || '');
  const [startAfterSave, setStartAfterSave] = useState(!isEditing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();

  const validationError = useMemo(() => {
    if (!name.trim()) return 'Name is required.';
    if (!isEditing && !token.trim()) return 'Telegram bot token is required.';
    if (defaultTopK < 1 || defaultTopK > 20) return 'Top K must be between 1 and 20.';
    if (welcomeMessage.length > 1000) return 'Welcome message must be 1000 characters or fewer.';
    return undefined;
  }, [defaultTopK, isEditing, name, token, welcomeMessage]);

  const handleSubmit = async () => {
    if (validationError) {
      setError(validationError);
      return;
    }
    let parsedChatIds: number[];
    try {
      parsedChatIds = parseAllowedChatIds(allowedChatIds);
    } catch (err) {
      setError((err as Error).message);
      return;
    }
    setSaving(true);
    try {
      await onSubmit({
        name: name.trim(),
        token: token.trim() || undefined,
        default_tone: defaultTone,
        default_top_k: defaultTopK,
        default_effective_status: defaultEffectiveStatus,
        default_domain: defaultDomain.trim(),
        default_doc_type: defaultDocType.trim(),
        allowed_chat_ids: parsedChatIds,
        welcome_message: welcomeMessage.trim(),
        start_after_save: startAfterSave
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
      <Input
        label={isEditing ? `Bot token (${value?.token_hint || 'unchanged'})` : 'Bot token'}
        type="password"
        value={token}
        onChange={(e) => setToken(e.target.value)}
        autoComplete="off"
      />
      <div className="grid two">
        <Select label="Default tone" value={defaultTone} onChange={(e) => setDefaultTone(e.target.value)}>
          <option value="default">default</option>
          <option value="academic">academic</option>
          <option value="procedure">procedure</option>
        </Select>
        <Input
          label="Top K"
          type="number"
          min={1}
          max={20}
          value={defaultTopK}
          onChange={(e) => setDefaultTopK(Number(e.target.value || 5))}
        />
      </div>
      <div className="grid two">
        <Select
          label="Effective status"
          value={defaultEffectiveStatus}
          onChange={(e) => setDefaultEffectiveStatus(e.target.value)}
        >
          <option value="active">active</option>
          <option value="archived">archived</option>
        </Select>
        <Input label="Domain" value={defaultDomain} onChange={(e) => setDefaultDomain(e.target.value)} />
      </div>
      <Input label="Doc type" value={defaultDocType} onChange={(e) => setDefaultDocType(e.target.value)} />
      <Input
        label="Allowed chat IDs"
        value={allowedChatIds}
        onChange={(e) => setAllowedChatIds(e.target.value)}
      />
      <label>
        <div className="label">Welcome message</div>
        <textarea className="input" rows={4} value={welcomeMessage} onChange={(e) => setWelcomeMessage(e.target.value)} />
      </label>
      <label style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
        <input type="checkbox" checked={startAfterSave} onChange={(e) => setStartAfterSave(e.target.checked)} />
        <span>Start after save</span>
      </label>
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <Button onClick={() => void handleSubmit()} disabled={saving}>
          <SaveIcon aria-hidden="true" />
          {saving ? 'Saving...' : 'Save'}
        </Button>
        {onCancel && (
          <Button variant="outline" onClick={onCancel} disabled={saving}>
            <CloseIcon aria-hidden="true" />
            Cancel
          </Button>
        )}
      </div>
    </div>
  );
};

export default TelegramBotForm;
