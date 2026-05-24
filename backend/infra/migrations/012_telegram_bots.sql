CREATE TABLE IF NOT EXISTS telegram_bots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  bot_token TEXT NOT NULL,
  token_hint TEXT NOT NULL DEFAULT '',
  bot_username TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'stopped' CHECK (status IN ('running', 'stopped', 'error')),
  default_tone TEXT NOT NULL DEFAULT 'default',
  default_top_k INTEGER NOT NULL DEFAULT 5,
  default_effective_status TEXT NOT NULL DEFAULT 'active',
  default_domain TEXT NOT NULL DEFAULT '',
  default_doc_type TEXT NOT NULL DEFAULT '',
  allowed_chat_ids BIGINT[] NOT NULL DEFAULT '{}'::BIGINT[],
  welcome_message TEXT NOT NULL DEFAULT '',
  last_update_id BIGINT NOT NULL DEFAULT 0,
  last_error TEXT,
  started_at TIMESTAMPTZ,
  stopped_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS telegram_chat_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  bot_id UUID NOT NULL REFERENCES telegram_bots(id) ON DELETE CASCADE,
  chat_id BIGINT NOT NULL,
  chat_type TEXT NOT NULL DEFAULT '',
  chat_title TEXT NOT NULL DEFAULT '',
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  last_message_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (bot_id, chat_id)
);

CREATE INDEX IF NOT EXISTS idx_telegram_bots_status ON telegram_bots(status);
CREATE INDEX IF NOT EXISTS idx_telegram_chat_links_bot_id ON telegram_chat_links(bot_id);
CREATE INDEX IF NOT EXISTS idx_telegram_chat_links_conversation_id ON telegram_chat_links(conversation_id);
