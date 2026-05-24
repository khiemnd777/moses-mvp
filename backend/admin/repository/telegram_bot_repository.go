package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
	"github.com/lib/pq"
)

type TelegramBotRepository struct {
	Store *infra.Store
}

func NewTelegramBotRepository(store *infra.Store) *TelegramBotRepository {
	return &TelegramBotRepository{Store: store}
}

func (r *TelegramBotRepository) List(ctx context.Context) ([]domain.TelegramBot, error) {
	rows, err := r.Store.DB.QueryContext(ctx, telegramBotSelectQuery()+`
ORDER BY b.updated_at DESC, b.created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.TelegramBot{}
	for rows.Next() {
		item, err := scanTelegramBot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TelegramBotRepository) ListByStatus(ctx context.Context, status string) ([]domain.TelegramBot, error) {
	rows, err := r.Store.DB.QueryContext(ctx, telegramBotSelectQuery()+`
WHERE b.status = $1
ORDER BY b.updated_at DESC, b.created_at DESC
`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.TelegramBot{}
	for rows.Next() {
		item, err := scanTelegramBot(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TelegramBotRepository) GetByID(ctx context.Context, id string) (domain.TelegramBot, error) {
	row := r.Store.DB.QueryRowContext(ctx, telegramBotSelectQuery()+`
WHERE b.id = $1
`, id)
	return scanTelegramBot(row)
}

func (r *TelegramBotRepository) Create(ctx context.Context, item domain.TelegramBot) (domain.TelegramBot, error) {
	var created domain.TelegramBot
	var chatIDs pq.Int64Array = item.AllowedChatIDs
	err := r.Store.DB.QueryRowContext(ctx, `
INSERT INTO telegram_bots (
	name,
	bot_token,
	token_hint,
	status,
	default_tone,
	default_top_k,
	default_effective_status,
	default_domain,
	default_doc_type,
	allowed_chat_ids,
	welcome_message
)
VALUES ($1, $2, $3, 'stopped', $4, $5, $6, $7, $8, $9, $10)
RETURNING id, name, bot_token, token_hint, bot_username, status, default_tone, default_top_k,
	default_effective_status, default_domain, default_doc_type, allowed_chat_ids, welcome_message,
	last_update_id, last_error, started_at, stopped_at, 0, created_at, updated_at
`,
		item.Name,
		item.Token,
		item.TokenHint,
		item.DefaultTone,
		item.DefaultTopK,
		item.DefaultEffectiveStatus,
		item.DefaultDomain,
		item.DefaultDocType,
		chatIDs,
		item.WelcomeMessage,
	).Scan(
		&created.ID,
		&created.Name,
		&created.Token,
		&created.TokenHint,
		&created.BotUsername,
		&created.Status,
		&created.DefaultTone,
		&created.DefaultTopK,
		&created.DefaultEffectiveStatus,
		&created.DefaultDomain,
		&created.DefaultDocType,
		pq.Array(&created.AllowedChatIDs),
		&created.WelcomeMessage,
		&created.LastUpdateID,
		&created.LastError,
		&created.StartedAt,
		&created.StoppedAt,
		&created.ChatCount,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	return created, err
}

func (r *TelegramBotRepository) Update(ctx context.Context, id string, item domain.TelegramBot, updateToken bool) (domain.TelegramBot, error) {
	var updated domain.TelegramBot
	var chatIDs pq.Int64Array = item.AllowedChatIDs
	var err error
	if updateToken {
		err = r.Store.DB.QueryRowContext(ctx, `
UPDATE telegram_bots
SET
	name = $2,
	bot_token = $3,
	token_hint = $4,
	bot_username = '',
	default_tone = $5,
	default_top_k = $6,
	default_effective_status = $7,
	default_domain = $8,
	default_doc_type = $9,
	allowed_chat_ids = $10,
	welcome_message = $11,
	updated_at = NOW()
WHERE id = $1
RETURNING id, name, bot_token, token_hint, bot_username, status, default_tone, default_top_k,
	default_effective_status, default_domain, default_doc_type, allowed_chat_ids, welcome_message,
	last_update_id, last_error, started_at, stopped_at,
	(SELECT COUNT(1)::INT FROM telegram_chat_links l WHERE l.bot_id = telegram_bots.id),
	created_at, updated_at
`,
			id,
			item.Name,
			item.Token,
			item.TokenHint,
			item.DefaultTone,
			item.DefaultTopK,
			item.DefaultEffectiveStatus,
			item.DefaultDomain,
			item.DefaultDocType,
			chatIDs,
			item.WelcomeMessage,
		).Scan(
			&updated.ID,
			&updated.Name,
			&updated.Token,
			&updated.TokenHint,
			&updated.BotUsername,
			&updated.Status,
			&updated.DefaultTone,
			&updated.DefaultTopK,
			&updated.DefaultEffectiveStatus,
			&updated.DefaultDomain,
			&updated.DefaultDocType,
			pq.Array(&updated.AllowedChatIDs),
			&updated.WelcomeMessage,
			&updated.LastUpdateID,
			&updated.LastError,
			&updated.StartedAt,
			&updated.StoppedAt,
			&updated.ChatCount,
			&updated.CreatedAt,
			&updated.UpdatedAt,
		)
		return updated, err
	}

	err = r.Store.DB.QueryRowContext(ctx, `
UPDATE telegram_bots
SET
	name = $2,
	default_tone = $3,
	default_top_k = $4,
	default_effective_status = $5,
	default_domain = $6,
	default_doc_type = $7,
	allowed_chat_ids = $8,
	welcome_message = $9,
	updated_at = NOW()
WHERE id = $1
RETURNING id, name, bot_token, token_hint, bot_username, status, default_tone, default_top_k,
	default_effective_status, default_domain, default_doc_type, allowed_chat_ids, welcome_message,
	last_update_id, last_error, started_at, stopped_at,
	(SELECT COUNT(1)::INT FROM telegram_chat_links l WHERE l.bot_id = telegram_bots.id),
	created_at, updated_at
`,
		id,
		item.Name,
		item.DefaultTone,
		item.DefaultTopK,
		item.DefaultEffectiveStatus,
		item.DefaultDomain,
		item.DefaultDocType,
		chatIDs,
		item.WelcomeMessage,
	).Scan(
		&updated.ID,
		&updated.Name,
		&updated.Token,
		&updated.TokenHint,
		&updated.BotUsername,
		&updated.Status,
		&updated.DefaultTone,
		&updated.DefaultTopK,
		&updated.DefaultEffectiveStatus,
		&updated.DefaultDomain,
		&updated.DefaultDocType,
		pq.Array(&updated.AllowedChatIDs),
		&updated.WelcomeMessage,
		&updated.LastUpdateID,
		&updated.LastError,
		&updated.StartedAt,
		&updated.StoppedAt,
		&updated.ChatCount,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	return updated, err
}

func (r *TelegramBotRepository) Delete(ctx context.Context, id string) (bool, error) {
	res, err := r.Store.DB.ExecContext(ctx, `DELETE FROM telegram_bots WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r *TelegramBotRepository) SetStatus(ctx context.Context, id, status string, lastError *string) (domain.TelegramBot, error) {
	row := r.Store.DB.QueryRowContext(ctx, `
UPDATE telegram_bots
SET
	status = $2,
	last_error = $3,
	started_at = CASE WHEN $2 = 'running' THEN NOW() ELSE started_at END,
	stopped_at = CASE WHEN $2 = 'stopped' THEN NOW() ELSE stopped_at END,
	updated_at = NOW()
WHERE id = $1
RETURNING id, name, bot_token, token_hint, bot_username, status, default_tone, default_top_k,
	default_effective_status, default_domain, default_doc_type, allowed_chat_ids, welcome_message,
	last_update_id, last_error, started_at, stopped_at,
	(SELECT COUNT(1)::INT FROM telegram_chat_links l WHERE l.bot_id = telegram_bots.id),
	created_at, updated_at
`, id, status, lastError)
	return scanTelegramBot(row)
}

func (r *TelegramBotRepository) SetIdentity(ctx context.Context, id, username string) error {
	_, err := r.Store.DB.ExecContext(ctx, `
UPDATE telegram_bots
SET bot_username = $2, updated_at = NOW()
WHERE id = $1
`, id, strings.TrimSpace(username))
	return err
}

func (r *TelegramBotRepository) UpdateLastUpdateID(ctx context.Context, id string, updateID int64) error {
	_, err := r.Store.DB.ExecContext(ctx, `
UPDATE telegram_bots
SET last_update_id = GREATEST(last_update_id, $2), updated_at = NOW()
WHERE id = $1
`, id, updateID)
	return err
}

func (r *TelegramBotRepository) GetOrCreateChatLink(ctx context.Context, bot domain.TelegramBot, chatID int64, chatType, chatTitle string) (domain.TelegramChatLink, bool, error) {
	tx, err := r.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.TelegramChatLink{}, false, err
	}
	defer tx.Rollback()

	link, err := getTelegramChatLink(ctx, tx, bot.ID, chatID)
	if err == nil {
		updated, updateErr := updateTelegramChatLinkMetadata(ctx, tx, link.ID, chatType, chatTitle)
		if updateErr != nil {
			return domain.TelegramChatLink{}, false, updateErr
		}
		if err := tx.Commit(); err != nil {
			return domain.TelegramChatLink{}, false, err
		}
		return updated, false, nil
	}
	if err != sql.ErrNoRows {
		return domain.TelegramChatLink{}, false, err
	}

	conversationTitle := strings.TrimSpace(chatTitle)
	if conversationTitle == "" {
		conversationTitle = "Telegram chat"
	}
	var conversationID string
	if err := tx.QueryRowContext(ctx, `
INSERT INTO conversations (title, user_id)
VALUES ($1, NULL)
RETURNING id
`, conversationTitle).Scan(&conversationID); err != nil {
		return domain.TelegramChatLink{}, false, err
	}

	created, err := insertTelegramChatLink(ctx, tx, bot.ID, chatID, chatType, chatTitle, conversationID)
	if err != nil {
		return domain.TelegramChatLink{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return domain.TelegramChatLink{}, false, err
	}
	return created, true, nil
}

func (r *TelegramBotRepository) TouchChatLink(ctx context.Context, botID string, chatID int64, at time.Time) error {
	_, err := r.Store.DB.ExecContext(ctx, `
UPDATE telegram_chat_links
SET last_message_at = $3, updated_at = NOW()
WHERE bot_id = $1 AND chat_id = $2
`, botID, chatID, at)
	return err
}

func telegramBotSelectQuery() string {
	return `
SELECT
	b.id,
	b.name,
	b.bot_token,
	b.token_hint,
	b.bot_username,
	b.status,
	b.default_tone,
	b.default_top_k,
	b.default_effective_status,
	b.default_domain,
	b.default_doc_type,
	b.allowed_chat_ids,
	b.welcome_message,
	b.last_update_id,
	b.last_error,
	b.started_at,
	b.stopped_at,
	COALESCE((SELECT COUNT(1)::INT FROM telegram_chat_links l WHERE l.bot_id = b.id), 0) AS chat_count,
	b.created_at,
	b.updated_at
FROM telegram_bots b
`
}

type telegramBotScanner interface {
	Scan(dest ...interface{}) error
}

func scanTelegramBot(scanner telegramBotScanner) (domain.TelegramBot, error) {
	var item domain.TelegramBot
	err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Token,
		&item.TokenHint,
		&item.BotUsername,
		&item.Status,
		&item.DefaultTone,
		&item.DefaultTopK,
		&item.DefaultEffectiveStatus,
		&item.DefaultDomain,
		&item.DefaultDocType,
		pq.Array(&item.AllowedChatIDs),
		&item.WelcomeMessage,
		&item.LastUpdateID,
		&item.LastError,
		&item.StartedAt,
		&item.StoppedAt,
		&item.ChatCount,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	return item, err
}

func getTelegramChatLink(ctx context.Context, exec dbtx, botID string, chatID int64) (domain.TelegramChatLink, error) {
	row := exec.QueryRowContext(ctx, `
SELECT id, bot_id, chat_id, chat_type, chat_title, conversation_id, last_message_at, created_at, updated_at
FROM telegram_chat_links
WHERE bot_id = $1 AND chat_id = $2
`, botID, chatID)
	return scanTelegramChatLink(row)
}

func updateTelegramChatLinkMetadata(ctx context.Context, exec dbtx, id, chatType, chatTitle string) (domain.TelegramChatLink, error) {
	row := exec.QueryRowContext(ctx, `
UPDATE telegram_chat_links
SET chat_type = $2, chat_title = $3, updated_at = NOW()
WHERE id = $1
RETURNING id, bot_id, chat_id, chat_type, chat_title, conversation_id, last_message_at, created_at, updated_at
`, id, strings.TrimSpace(chatType), strings.TrimSpace(chatTitle))
	return scanTelegramChatLink(row)
}

func insertTelegramChatLink(ctx context.Context, exec dbtx, botID string, chatID int64, chatType, chatTitle, conversationID string) (domain.TelegramChatLink, error) {
	row := exec.QueryRowContext(ctx, `
INSERT INTO telegram_chat_links (bot_id, chat_id, chat_type, chat_title, conversation_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, bot_id, chat_id, chat_type, chat_title, conversation_id, last_message_at, created_at, updated_at
`, botID, chatID, strings.TrimSpace(chatType), strings.TrimSpace(chatTitle), conversationID)
	return scanTelegramChatLink(row)
}

func scanTelegramChatLink(scanner telegramBotScanner) (domain.TelegramChatLink, error) {
	var link domain.TelegramChatLink
	err := scanner.Scan(
		&link.ID,
		&link.BotID,
		&link.ChatID,
		&link.ChatType,
		&link.ChatTitle,
		&link.ConversationID,
		&link.LastMessageAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)
	return link, err
}
