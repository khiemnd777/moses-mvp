package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/khiemnd777/legal_api/admin/repository"
	adminservice "github.com/khiemnd777/legal_api/admin/service"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/domain"
)

const (
	telegramAPIBaseURL     = "https://api.telegram.org"
	telegramPollTimeoutSec = 30
	telegramMaxReplyChars  = 3900
	telegramMaxCitations   = 5
)

type TelegramManager struct {
	repo       *repository.TelegramBotRepository
	handler    *Handler
	logger     *slog.Logger
	httpClient *http.Client

	mu      sync.Mutex
	runners map[string]context.CancelFunc
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID int64        `json:"message_id"`
	From      telegramUser `json:"from"`
	Chat      telegramChat `json:"chat"`
	Text      string       `json:"text"`
	Date      int64        `json:"date"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type telegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type telegramMe struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username"`
}

type telegramOutgoingMessage struct {
	Text        string
	ReplyMarkup *telegramInlineKeyboardMarkup
}

type telegramInlineKeyboardMarkup struct {
	InlineKeyboard [][]telegramInlineKeyboardButton `json:"inline_keyboard"`
}

type telegramInlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type telegramAPIClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

type telegramAPIResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
}

func NewTelegramManager(repo *repository.TelegramBotRepository, handler *Handler, logger *slog.Logger) *TelegramManager {
	return &TelegramManager{
		repo:       repo,
		handler:    handler,
		logger:     logger,
		httpClient: &http.Client{Timeout: time.Duration(telegramPollTimeoutSec+10) * time.Second},
		runners:    map[string]context.CancelFunc{},
	}
}

func (m *TelegramManager) StartAll(ctx context.Context) {
	items, err := m.repo.ListByStatus(ctx, domain.TelegramBotStatusRunning)
	if err != nil {
		m.logError("telegram_start_all_failed", slog.String("error", err.Error()))
		return
	}
	for _, item := range items {
		m.startRunner(ctx, item)
	}
}

func (m *TelegramManager) StartTelegramBot(ctx context.Context, id string) (domain.TelegramBot, error) {
	bot, err := m.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TelegramBot{}, adminservice.ErrTelegramBotNotFound
		}
		return domain.TelegramBot{}, err
	}
	client := m.newClient(bot.Token)
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	me, err := client.GetMe(checkCtx)
	if err != nil {
		message := err.Error()
		_, _ = m.repo.SetStatus(context.Background(), id, domain.TelegramBotStatusError, &message)
		return domain.TelegramBot{}, err
	}
	if strings.TrimSpace(me.Username) != "" {
		_ = m.repo.SetIdentity(context.Background(), id, me.Username)
		bot.BotUsername = me.Username
	}
	started, err := m.repo.SetStatus(ctx, id, domain.TelegramBotStatusRunning, nil)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TelegramBot{}, adminservice.ErrTelegramBotNotFound
		}
		return domain.TelegramBot{}, err
	}
	if strings.TrimSpace(me.Username) != "" {
		started.BotUsername = me.Username
	}
	m.startRunner(context.Background(), started)
	return started, nil
}

func (m *TelegramManager) StopTelegramBot(ctx context.Context, id string) (domain.TelegramBot, error) {
	m.StopTelegramBotIfRunning(id)
	stopped, err := m.repo.SetStatus(ctx, id, domain.TelegramBotStatusStopped, nil)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.TelegramBot{}, adminservice.ErrTelegramBotNotFound
		}
		return domain.TelegramBot{}, err
	}
	return stopped, nil
}

func (m *TelegramManager) StopTelegramBotIfRunning(id string) {
	m.mu.Lock()
	cancel := m.runners[id]
	delete(m.runners, id)
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (m *TelegramManager) startRunner(parent context.Context, bot domain.TelegramBot) {
	if strings.TrimSpace(bot.Token) == "" {
		return
	}
	m.StopTelegramBotIfRunning(bot.ID)
	runnerCtx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.runners[bot.ID] = cancel
	m.mu.Unlock()
	go m.runBot(runnerCtx, bot)
}

func (m *TelegramManager) runBot(ctx context.Context, bot domain.TelegramBot) {
	client := m.newClient(bot.Token)
	offset := bot.LastUpdateID + 1
	m.logInfo("telegram_bot_started", slog.String("bot_id", bot.ID), slog.String("bot_username", bot.BotUsername))
	defer func() {
		m.logInfo("telegram_bot_stopped", slog.String("bot_id", bot.ID))
	}()

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		updates, err := client.GetUpdates(ctx, offset, telegramPollTimeoutSec)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			message := err.Error()
			_, _ = m.repo.SetStatus(context.Background(), bot.ID, domain.TelegramBotStatusError, &message)
			m.logError("telegram_poll_failed", slog.String("bot_id", bot.ID), slog.String("error", message))
			return
		}
		for _, update := range updates {
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
				_ = m.repo.UpdateLastUpdateID(context.Background(), bot.ID, update.UpdateID)
			}
			if update.Message == nil {
				continue
			}
			m.handleMessage(ctx, bot, client, *update.Message)
		}
	}
}

func (m *TelegramManager) handleMessage(ctx context.Context, bot domain.TelegramBot, client *telegramAPIClient, msg telegramMessage) {
	if latest, err := m.repo.GetByID(ctx, bot.ID); err == nil {
		bot = latest
	}
	content := strings.TrimSpace(msg.Text)
	if content == "" || msg.From.IsBot {
		return
	}
	if !telegramChatAllowed(bot.AllowedChatIDs, msg.Chat.ID) {
		_ = client.SendMessage(ctx, msg.Chat.ID, "Chat này chưa được cấp quyền sử dụng bot.", nil)
		return
	}
	chatTitle := telegramChatTitle(msg.Chat)
	link, _, err := m.repo.GetOrCreateChatLink(ctx, bot, msg.Chat.ID, msg.Chat.Type, chatTitle)
	if err != nil {
		m.logError("telegram_chat_link_failed", slog.String("bot_id", bot.ID), slog.Int64("chat_id", msg.Chat.ID), slog.String("error", err.Error()))
		_ = client.SendMessage(ctx, msg.Chat.ID, "Không thể khởi tạo cuộc hội thoại Telegram.", nil)
		return
	}
	_ = m.repo.TouchChatLink(context.Background(), bot.ID, msg.Chat.ID, time.Now())
	if strings.HasPrefix(content, "/start") {
		welcome := strings.TrimSpace(bot.WelcomeMessage)
		if welcome == "" {
			welcome = "Xin chào. Hãy gửi câu hỏi pháp lý, tôi sẽ trả lời dựa trên dữ liệu đã được nạp."
		}
		_ = client.SendMessage(ctx, msg.Chat.ID, welcome, nil)
		return
	}
	result, err := m.handler.runChatTurn(ctx, nil, link.ConversationID, content, telegramBotFilters(bot))
	if err != nil {
		m.logError("telegram_chat_turn_failed", slog.String("bot_id", bot.ID), slog.Int64("chat_id", msg.Chat.ID), slog.String("error", err.Error()))
		_ = client.SendMessage(ctx, msg.Chat.ID, "Tôi chưa xử lý được câu hỏi này. Vui lòng thử lại sau.", nil)
		return
	}
	assistantMessage := toChatMessageResponse(result.AssistantMessage)
	outgoing := m.handler.buildTelegramOutgoingMessages(assistantMessage.Content, assistantMessage.Citations, telegramMaxReplyChars)
	for _, item := range outgoing {
		if err := client.SendMessage(ctx, msg.Chat.ID, item.Text, item.ReplyMarkup); err != nil {
			m.logError("telegram_send_failed", slog.String("bot_id", bot.ID), slog.Int64("chat_id", msg.Chat.ID), slog.String("error", err.Error()))
			return
		}
	}
}

func telegramBotFilters(bot domain.TelegramBot) ChatFilters {
	return ChatFilters{
		Tone:            bot.DefaultTone,
		TopK:            bot.DefaultTopK,
		EffectiveStatus: bot.DefaultEffectiveStatus,
		Domain:          bot.DefaultDomain,
		DocType:         bot.DefaultDocType,
	}
}

func telegramChatAllowed(allowed []int64, chatID int64) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, allowedID := range allowed {
		if allowedID == chatID {
			return true
		}
	}
	return false
}

func telegramChatTitle(chat telegramChat) string {
	for _, value := range []string{chat.Title, chat.Username, strings.TrimSpace(chat.FirstName + " " + chat.LastName)} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return "Telegram: " + trimmed
		}
	}
	return fmt.Sprintf("Telegram: %d", chat.ID)
}

func (h *Handler) buildTelegramOutgoingMessages(content string, citations []answer.Citation, maxChars int) []telegramOutgoingMessage {
	if maxChars <= 0 || maxChars > telegramMaxReplyChars {
		maxChars = telegramMaxReplyChars
	}
	content = strings.TrimSpace(content)
	if content == "" {
		content = "Không có nội dung trả lời."
	}
	citations = validateCitations(citations)
	citationText, markup := h.telegramCitationBlock(citations)
	fullText := content
	if citationText != "" {
		fullText = strings.TrimSpace(fullText + "\n\n" + citationText)
	}
	parts := splitTelegramText(fullText, maxChars-32)
	out := make([]telegramOutgoingMessage, 0, len(parts))
	for idx, part := range parts {
		text := part
		if len(parts) > 1 {
			text = fmt.Sprintf("Phần %d/%d\n\n%s", idx+1, len(parts), part)
		}
		msg := telegramOutgoingMessage{Text: text}
		if idx == len(parts)-1 {
			msg.ReplyMarkup = markup
		}
		out = append(out, msg)
	}
	if len(out) == 0 {
		out = append(out, telegramOutgoingMessage{Text: content, ReplyMarkup: markup})
	}
	return out
}

func (h *Handler) telegramCitationBlock(citations []answer.Citation) (string, *telegramInlineKeyboardMarkup) {
	if len(citations) == 0 {
		return "", nil
	}
	lines := []string{"Nguồn:"}
	keyboard := make([][]telegramInlineKeyboardButton, 0)
	for idx, citation := range citations {
		if idx >= telegramMaxCitations {
			break
		}
		label := firstNonEmpty(citation.CitationLabel, citation.DocumentTitle, citation.LawName, citation.DocumentNumber, fmt.Sprintf("Nguồn %d", idx+1))
		publicURL := h.buildPublicCitationURL(citation)
		line := fmt.Sprintf("[%d] %s", idx+1, label)
		if publicURL != "" {
			line += " - " + publicURL
			keyboard = append(keyboard, []telegramInlineKeyboardButton{{
				Text: fmt.Sprintf("Xem nguồn %d", idx+1),
				URL:  publicURL,
			}})
		} else if strings.HasPrefix(citation.URL, "http://") || strings.HasPrefix(citation.URL, "https://") {
			line += " - " + citation.URL
			keyboard = append(keyboard, []telegramInlineKeyboardButton{{
				Text: fmt.Sprintf("Xem nguồn %d", idx+1),
				URL:  citation.URL,
			}})
		}
		lines = append(lines, line)
	}
	if len(citations) > telegramMaxCitations {
		lines = append(lines, fmt.Sprintf("... và %d nguồn khác.", len(citations)-telegramMaxCitations))
	}
	var markup *telegramInlineKeyboardMarkup
	if len(keyboard) > 0 {
		markup = &telegramInlineKeyboardMarkup{InlineKeyboard: keyboard}
	}
	return strings.Join(lines, "\n"), markup
}

func splitTelegramText(text string, maxChars int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if maxChars <= 0 {
		maxChars = telegramMaxReplyChars
	}
	paragraphs := strings.Split(text, "\n\n")
	parts := make([]string, 0, 1)
	var current strings.Builder
	flush := func() {
		if strings.TrimSpace(current.String()) != "" {
			parts = append(parts, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		if len([]rune(paragraph)) > maxChars {
			flush()
			parts = append(parts, splitLongRunes(paragraph, maxChars)...)
			continue
		}
		next := paragraph
		if current.Len() > 0 {
			next = current.String() + "\n\n" + paragraph
		}
		if len([]rune(next)) > maxChars {
			flush()
			current.WriteString(paragraph)
			continue
		}
		current.Reset()
		current.WriteString(next)
	}
	flush()
	return parts
}

func splitLongRunes(text string, maxChars int) []string {
	runes := []rune(text)
	parts := make([]string, 0, len(runes)/maxChars+1)
	for start := 0; start < len(runes); start += maxChars {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, strings.TrimSpace(string(runes[start:end])))
	}
	return parts
}

func (m *TelegramManager) newClient(token string) *telegramAPIClient {
	return &telegramAPIClient{
		token:      token,
		baseURL:    telegramAPIBaseURL,
		httpClient: m.httpClient,
	}
}

func (c *telegramAPIClient) GetMe(ctx context.Context) (telegramMe, error) {
	var out telegramMe
	if err := c.call(ctx, "getMe", nil, &out); err != nil {
		return telegramMe{}, err
	}
	return out, nil
}

func (c *telegramAPIClient) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]telegramUpdate, error) {
	payload := map[string]interface{}{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message"},
	}
	var out []telegramUpdate
	if err := c.call(ctx, "getUpdates", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *telegramAPIClient) SendMessage(ctx context.Context, chatID int64, text string, markup *telegramInlineKeyboardMarkup) error {
	payload := map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if markup != nil && len(markup.InlineKeyboard) > 0 {
		payload["reply_markup"] = markup
	}
	return c.call(ctx, "sendMessage", payload, nil)
}

func (c *telegramAPIClient) call(ctx context.Context, method string, payload interface{}, result interface{}) error {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var envelope json.RawMessage
	var base telegramAPIResponse[json.RawMessage]
	if err := json.NewDecoder(resp.Body).Decode(&base); err != nil {
		return err
	}
	if !base.OK {
		if base.Description == "" {
			base.Description = resp.Status
		}
		return fmt.Errorf("telegram api %s failed: %s", method, base.Description)
	}
	envelope = base.Result
	if result != nil {
		if err := json.Unmarshal(envelope, result); err != nil {
			return err
		}
	}
	return nil
}

func (m *TelegramManager) logInfo(message string, attrs ...slog.Attr) {
	if m.logger == nil {
		return
	}
	m.logger.LogAttrs(context.Background(), slog.LevelInfo, message, attrs...)
}

func (m *TelegramManager) logError(message string, attrs ...slog.Attr) {
	if m.logger == nil {
		return
	}
	m.logger.LogAttrs(context.Background(), slog.LevelError, message, attrs...)
}
