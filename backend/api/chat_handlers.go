package api

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/core/answer"
	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/internal/auth"
	"github.com/khiemnd777/legal_api/observability"
)

const (
	defaultConversationTitle = "Cuộc hội thoại mới"
	maxConversationTitleLen  = 60
	streamHeartbeatInterval  = 15 * time.Second
	streamTimeout            = 90 * time.Second
)

type conversationRequest struct {
	Title string `json:"title"`
}

type createMessageRequest struct {
	ConversationID string      `json:"conversation_id"`
	Content        string      `json:"content"`
	Filters        ChatFilters `json:"filters"`
}

type conversationResponse struct {
	ID            string                `json:"conversation_id"`
	Title         string                `json:"title"`
	UserID        *string               `json:"user_id,omitempty"`
	LastMessage   string                `json:"last_message_preview,omitempty"`
	LastMessageAt *time.Time            `json:"last_message_at,omitempty"`
	MessageCount  int                   `json:"message_count"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at"`
	Messages      []chatMessageResponse `json:"messages,omitempty"`
}

type chatMessageResponse struct {
	MessageID      string            `json:"message_id"`
	ConversationID string            `json:"conversation_id"`
	Role           string            `json:"role"`
	Content        string            `json:"content"`
	Citations      []answer.Citation `json:"citations"`
	TraceID        *string           `json:"trace_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

type chatStreamState struct {
	writer     *sseWriter
	builder    strings.Builder
	citations  []answer.Citation
	done       bool
	tokenSent  bool
	lastErr    error
	sendErrors bool
}

type citationDetailResponse struct {
	Citation    answer.Citation `json:"citation"`
	Content     string          `json:"content"`
	SourceType  string          `json:"source_type"`
	FileName    string          `json:"file_name,omitempty"`
	ContentType string          `json:"content_type,omitempty"`
}

func (s *chatStreamState) OnToken(delta string) error {
	s.tokenSent = true
	s.builder.WriteString(delta)
	return s.writer.writeEvent("token", fiber.Map{"delta": delta})
}

func (s *chatStreamState) OnCitations(citations []answer.Citation) error {
	s.citations = citations
	return nil
}

func (s *chatStreamState) OnError(err error) {
	s.lastErr = err
	if !s.sendErrors {
		return
	}
	_ = s.writer.writeEvent("error", fiber.Map{"code": "stream_error", "message": "stream interrupted"})
}

func (s *chatStreamState) OnDone() {
	s.done = true
}

func (s *chatStreamState) EmitCitations(citations []answer.Citation) error {
	s.citations = citations
	return s.writer.writeEvent("citations", citations)
}

func (s *chatStreamState) EmitDone() error {
	s.done = true
	return s.writer.writeEvent("done", fiber.Map{"ok": true})
}

func (h *Handler) CreateConversation(c *fiber.Ctx) error {
	var req conversationRequest
	if err := c.BodyParser(&req); err != nil && len(c.Body()) > 0 {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	userID := currentUserID(c)
	title := sanitizeConversationTitle(req.Title)
	if title == "" {
		title = defaultConversationTitle
	}
	convo, err := h.Store.CreateConversation(c.UserContext(), title, userID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create conversation", err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(toConversationResponse(convo))
}

func (h *Handler) ListConversations(c *fiber.Ctx) error {
	items, err := h.Store.ListConversations(c.UserContext(), currentUserID(c))
	if err != nil {
		return respondError(c, 500, "db_error", "failed to list conversations", err.Error())
	}
	resp := make([]conversationResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toConversationResponse(item))
	}
	return c.JSON(fiber.Map{"items": resp})
}

func (h *Handler) GetConversation(c *fiber.Ctx) error {
	conversationID := strings.TrimSpace(c.Params("id"))
	if conversationID == "" {
		return respondError(c, 400, "validation", "conversation id is required", nil)
	}
	convo, err := h.Store.GetConversation(c.UserContext(), conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "conversation not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load conversation", err.Error())
	}
	if !canAccessConversation(convo, currentUserID(c)) {
		return respondError(c, 404, "not_found", "conversation not found", nil)
	}
	msgs, err := h.Store.ListMessagesByConversation(c.UserContext(), conversationID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to load messages", err.Error())
	}
	resp := toConversationResponse(convo)
	resp.Messages = mapMessages(msgs)
	return c.JSON(resp)
}

func (h *Handler) DeleteConversation(c *fiber.Ctx) error {
	conversationID := strings.TrimSpace(c.Params("id"))
	if conversationID == "" {
		return respondError(c, 400, "validation", "conversation id is required", nil)
	}
	convo, err := h.Store.GetConversation(c.UserContext(), conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "conversation not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load conversation", err.Error())
	}
	if !canAccessConversation(convo, currentUserID(c)) {
		return respondError(c, 404, "not_found", "conversation not found", nil)
	}
	deleted, err := h.Store.DeleteConversation(c.UserContext(), conversationID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to delete conversation", err.Error())
	}
	if !deleted {
		return respondError(c, 404, "not_found", "conversation not found", nil)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) ListMessages(c *fiber.Ctx) error {
	conversationID := strings.TrimSpace(c.Query("conversation_id"))
	if conversationID == "" {
		return respondError(c, 400, "validation", "conversation_id is required", nil)
	}
	convo, err := h.Store.GetConversation(c.UserContext(), conversationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "conversation not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load conversation", err.Error())
	}
	if !canAccessConversation(convo, currentUserID(c)) {
		return respondError(c, 404, "not_found", "conversation not found", nil)
	}
	msgs, err := h.Store.ListMessagesByConversation(c.UserContext(), conversationID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to list messages", err.Error())
	}
	return c.JSON(fiber.Map{"items": mapMessages(msgs)})
}

func (h *Handler) CreateMessage(c *fiber.Ctx) error {
	var req createMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	result, err := h.runChatTurn(c.UserContext(), currentUserID(c), req.ConversationID, req.Content, req.Filters)
	if err != nil {
		return h.respondChatError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"conversation": toConversationResponse(result.Conversation),
		"user_message": toChatMessageResponse(result.UserMessage),
		"message":      toChatMessageResponse(result.AssistantMessage),
		"trace_id":     result.TraceID,
	})
}

func (h *Handler) StreamMessage(c *fiber.Ctx) error {
	var req createMessageRequest
	if err := c.BodyParser(&req); err != nil {
		return respondError(c, 400, "invalid_request", "invalid json", err.Error())
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return respondError(c, 400, "validation", "content is required", nil)
	}
	ctx := c.UserContext()
	userID := currentUserID(c)

	convo, userMsg, _, err := h.prepareConversationTurn(ctx, userID, req.ConversationID, content)
	if err != nil {
		return h.respondChatError(c, err)
	}

	started := time.Now()
	traceCtx, traceSvc, traceID, err := h.startAnswerTrace(ctx, "stream", content)
	if err != nil {
		return respondError(c, 500, "trace_error", "failed to create answer trace", err.Error())
	}
	runtimeCfg, err := h.loadRuntimeAnswerConfig(traceCtx, req.Filters.Tone)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return respondError(c, 500, "config_error", "failed to load answer runtime config", err.Error())
	}

	results, history, decision, sources, promptOpts, ansSvc, err := h.prepareChatResponse(traceCtx, convo.ID, content, req.Filters, runtimeCfg, traceSvc, started)
	if err != nil {
		return h.respondChatError(c, err)
	}

	assistantMsg, err := h.Store.CreateMessage(traceCtx, convo.ID, "assistant", "", []byte("[]"), &traceID)
	if err != nil {
		return respondError(c, 500, "db_error", "failed to create assistant message", err.Error())
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		streamCtx, cancel := context.WithTimeout(traceCtx, streamTimeout)
		defer cancel()

		reqCtx := c.Context()
		if reqCtx != nil && reqCtx.Done() != nil {
			go func() {
				<-reqCtx.Done()
				cancel()
			}()
		}

		writer := newSSEWriter(w)
		state := &chatStreamState{writer: writer}
		_ = writer.writeEvent("meta", fiber.Map{
			"conversation_id":      convo.ID,
			"user_message_id":      userMsg.ID,
			"assistant_message_id": assistantMsg.ID,
			"trace_id":             traceID,
			"title":                convo.Title,
		})

		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(streamHeartbeatInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = writer.writeHeartbeat()
				case <-heartbeatDone:
					return
				}
			}
		}()
		defer close(heartbeatDone)

		if !decision.Allow() {
			state.OnToken(decision.Message)
			_ = state.OnCitations([]answer.Citation{})
			state.OnDone()
			_ = h.Store.UpdateMessage(streamCtx, assistantMsg.ID, decision.Message, []byte("[]"), &traceID)
			traceSvc.OnResponse(decision.Message, true, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_done", convo.ID, assistantMsg.ID, traceID, results, started, state.builder.String(), nil)
			return
		}

		llmStarted := time.Now()
		if err := ansSvc.StreamWithHistory(streamCtx, history, content, sources, promptOpts, state); err != nil {
			traceSvc.OnError(err, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, state.builder.String(), err)
			return
		}
		streamedContent := state.builder.String()
		finalContent := streamedContent
		finalContent, finalCitations, valid, validationErr := h.validateGeneratedLegalAnswerForStream(streamCtx, finalContent, sources)
		if validationErr != nil {
			traceSvc.OnError(validationErr, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_validation_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, validationErr)
			return
		}
		if !valid {
			finalCitations = []answer.Citation{}
		}
		if strings.HasPrefix(finalContent, streamedContent) {
			if delta := strings.TrimPrefix(finalContent, streamedContent); delta != "" {
				if err := state.OnToken(delta); err != nil {
					traceSvc.OnError(err, traceLatency(started))
					h.logChatLifecycle(streamCtx, "chat_stream_emit_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, err)
					return
				}
			}
		}
		if err := state.EmitCitations(finalCitations); err != nil {
			traceSvc.OnError(err, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_emit_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, err)
			return
		}
		if err := state.EmitDone(); err != nil {
			traceSvc.OnError(err, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_emit_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, err)
			return
		}

		if updateErr := h.Store.UpdateMessage(streamCtx, assistantMsg.ID, finalContent, marshalCitations(finalCitations), &traceID); updateErr != nil {
			traceSvc.OnError(updateErr, traceLatency(started))
			h.logChatLifecycle(streamCtx, "chat_stream_persist_error", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, updateErr)
			return
		}
		traceSvc.OnResponse(finalContent, true, traceLatency(started))
		h.logChatLifecycle(streamCtx, "chat_stream_done", convo.ID, assistantMsg.ID, traceID, results, llmStarted, finalContent, nil)
	})

	return nil
}

func (h *Handler) DownloadAsset(c *fiber.Ctx) error {
	assetID := strings.TrimSpace(c.Params("id"))
	if assetID == "" {
		return respondError(c, 400, "validation", "asset id is required", nil)
	}
	asset, err := h.Store.GetDocumentAsset(c.UserContext(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "asset not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load asset", err.Error())
	}
	fullPath := filepath.Join(h.Storage.Root, asset.StoragePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return respondError(c, 404, "not_found", "asset file not found", nil)
		}
		return respondError(c, 500, "storage_error", "failed to stat asset file", err.Error())
	}

	downloadName := strings.TrimSpace(asset.FileName)
	if downloadName == "" {
		downloadName = filepath.Base(asset.StoragePath)
	}

	c.Set(fiber.HeaderContentType, resolveDownloadContentType(downloadName, asset.ContentType, fullPath))
	c.Set(fiber.HeaderContentDisposition, buildAttachmentDisposition(downloadName))
	c.Set(fiber.HeaderContentLength, strconv.FormatInt(info.Size(), 10))
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.SendFile(fullPath)
}

func resolveDownloadContentType(fileName, storedContentType, fullPath string) string {
	normalizedStored := strings.TrimSpace(storedContentType)
	ext := strings.ToLower(filepath.Ext(fileName))

	if officeType := officeContentTypeByExtension(ext); officeType != "" {
		switch normalizedStored {
		case "", fiber.MIMEOctetStream, "application/zip", "application/x-zip-compressed", "multipart/form-data":
			return officeType
		}
	}

	if normalizedStored != "" {
		return normalizedStored
	}

	if guessed := mime.TypeByExtension(ext); guessed != "" {
		return guessed
	}

	file, err := os.Open(fullPath)
	if err == nil {
		defer file.Close()
		header := make([]byte, 512)
		n, readErr := file.Read(header)
		if readErr == nil || errors.Is(readErr, io.EOF) {
			return http.DetectContentType(header[:n])
		}
	}
	return fiber.MIMEOctetStream
}

func officeContentTypeByExtension(ext string) string {
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return ""
	}
}

func buildAttachmentDisposition(fileName string) string {
	escapedFileName := strings.NewReplacer("\\", "\\\\", `"`, `\"`).Replace(fileName)
	return `attachment; filename="` + escapedFileName + `"; filename*=UTF-8''` + url.PathEscape(fileName)
}

func (h *Handler) GetCitationDetail(c *fiber.Ctx) error {
	chunkID := strings.TrimSpace(c.Query("chunk_id"))
	assetID := strings.TrimSpace(c.Query("asset_id"))
	if chunkID == "" && assetID == "" {
		return respondError(c, 400, "validation", "chunk_id or asset_id is required", nil)
	}

	if chunkID != "" {
		chunks, err := h.Store.GetChunksByIDs(c.UserContext(), []string{chunkID})
		if err != nil {
			return respondError(c, 500, "db_error", "failed to load citation chunk", err.Error())
		}
		if len(chunks) > 0 {
			detail, err := h.buildCitationDetailFromChunk(c.UserContext(), chunks[0])
			if err != nil {
				return respondError(c, 500, "citation_detail_error", "failed to build citation detail", err.Error())
			}
			return c.JSON(detail)
		}
		if assetID == "" {
			return respondError(c, 404, "not_found", "citation chunk not found", nil)
		}
	}

	asset, err := h.Store.GetDocumentAsset(c.UserContext(), assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return respondError(c, 404, "not_found", "citation asset not found", nil)
		}
		return respondError(c, 500, "db_error", "failed to load citation asset", err.Error())
	}
	content, err := h.Storage.Read(asset.StoragePath)
	if err != nil {
		return respondError(c, 500, "storage_error", "failed to read citation asset", err.Error())
	}
	fileURL := "/assets/" + asset.ID + "/download"
	return c.JSON(citationDetailResponse{
		Citation: answer.Citation{
			AssetID: asset.ID,
			FileURL: fileURL,
			URL:     fileURL,
		},
		Content:     content,
		SourceType:  "asset",
		FileName:    asset.FileName,
		ContentType: asset.ContentType,
	})
}

type chatTurnResult struct {
	Conversation     domain.Conversation
	UserMessage      domain.Message
	AssistantMessage domain.Message
	TraceID          string
}

func (h *Handler) runChatTurn(ctx context.Context, userID *string, conversationID, content string, filters ChatFilters) (chatTurnResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return chatTurnResult{}, fiber.NewError(fiber.StatusBadRequest, "content is required")
	}
	convo, userMsg, created, err := h.prepareConversationTurn(ctx, userID, conversationID, content)
	if err != nil {
		return chatTurnResult{}, err
	}

	started := time.Now()
	traceCtx, traceSvc, traceID, err := h.startAnswerTrace(ctx, "answer", content)
	if err != nil {
		return chatTurnResult{}, err
	}
	runtimeCfg, err := h.loadRuntimeAnswerConfig(traceCtx, filters.Tone)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return chatTurnResult{}, err
	}
	results, history, decision, sources, promptOpts, ansSvc, err := h.prepareChatResponse(traceCtx, convo.ID, content, filters, runtimeCfg, traceSvc, started)
	if err != nil {
		return chatTurnResult{}, err
	}

	var assistantContent string
	var citations []answer.Citation
	if !decision.Allow() {
		assistantContent = decision.Message
		citations = []answer.Citation{}
		traceSvc.OnResponse(assistantContent, true, traceLatency(started))
	} else {
		llmStarted := time.Now()
		assistantContent, err = ansSvc.GenerateWithHistory(traceCtx, history, content, sources, promptOpts)
		if err != nil {
			traceSvc.OnError(err, traceLatency(started))
			h.logChatLifecycle(traceCtx, "chat_completion_error", convo.ID, "", traceID, results, llmStarted, "", err)
			return chatTurnResult{}, err
		}
		assistantContent, citations, _, err = h.validateGeneratedLegalAnswer(traceCtx, assistantContent, sources)
		if err != nil {
			traceSvc.OnError(err, traceLatency(started))
			h.logChatLifecycle(traceCtx, "chat_completion_validation_error", convo.ID, "", traceID, results, llmStarted, assistantContent, err)
			return chatTurnResult{}, err
		}
		traceSvc.OnResponse(assistantContent, true, traceLatency(started))
		h.logChatLifecycle(traceCtx, "chat_completion_done", convo.ID, "", traceID, results, llmStarted, assistantContent, nil)
	}

	assistantMsg, err := h.Store.CreateMessage(traceCtx, convo.ID, "assistant", assistantContent, marshalCitations(citations), &traceID)
	if err != nil {
		return chatTurnResult{}, err
	}
	if created {
		if title := generateConversationTitle(content); title != "" && title != convo.Title {
			if err := h.Store.UpdateConversationTitle(traceCtx, convo.ID, title); err == nil {
				convo.Title = title
			}
		}
	}
	convo, _ = h.Store.GetConversation(traceCtx, convo.ID)
	return chatTurnResult{
		Conversation:     convo,
		UserMessage:      userMsg,
		AssistantMessage: assistantMsg,
		TraceID:          traceID,
	}, nil
}

func (h *Handler) prepareConversationTurn(ctx context.Context, userID *string, conversationID, content string) (domain.Conversation, domain.Message, bool, error) {
	conversationID = strings.TrimSpace(conversationID)
	created := false
	var (
		convo domain.Conversation
		err   error
	)
	if conversationID == "" {
		title := generateConversationTitle(content)
		if title == "" {
			title = defaultConversationTitle
		}
		convo, err = h.Store.CreateConversation(ctx, title, userID)
		created = true
	} else {
		convo, err = h.Store.GetConversation(ctx, conversationID)
	}
	if err != nil {
		return domain.Conversation{}, domain.Message{}, false, err
	}
	if !canAccessConversation(convo, userID) {
		return domain.Conversation{}, domain.Message{}, false, sql.ErrNoRows
	}
	msg, err := h.Store.CreateMessage(ctx, convo.ID, "user", strings.TrimSpace(content), []byte("[]"), nil)
	if err != nil {
		return domain.Conversation{}, domain.Message{}, false, err
	}
	return convo, msg, created, nil
}

func (h *Handler) prepareChatResponse(
	ctx context.Context,
	conversationID, content string,
	filters ChatFilters,
	runtimeCfg runtimeAnswerConfig,
	traceSvc *observability.TraceService,
	started time.Time,
) ([]retrieval.Result, []answer.ConversationMessage, guardDecision, []answer.Source, answer.PromptBuildOptions, *answer.Service, error) {
	normalizedFilters := normalizeChatFilters(filters, h.Tones)
	initialAnalysis := h.Retriever.AnalyzeQuery(ctx, content)
	if decision, ok, normalized := h.detectSmallTalkDecision(ctx, content); ok {
		traceSvc.OnRetrieval(normalized, map[string]interface{}{
			"canonical_query":       initialAnalysis.CanonicalQuery,
			"matched_doc_types":     initialAnalysis.MatchedDocTypes,
			"matched_query_rules":   initialAnalysis.MatchedQueryRules,
			"query_profile_hashes":  initialAnalysis.QueryProfileHashes,
			"inferred_intent":       initialAnalysis.Intent,
			"inferred_legal_domain": initialAnalysis.LegalDomain,
			"inferred_legal_topic":  initialAnalysis.LegalTopic,
			"conversation_id":       conversationID,
			"retrieval_query":       content,
			"legal_domain":          normalizedFilters.Domain,
			"document_type":         normalizedFilters.DocType,
			"effective_status":      normalizedFilters.EffectiveStatus,
			"document_number":       normalizedFilters.DocumentNumber,
			"article_number":        normalizedFilters.ArticleNumber,
			"retrieved_chunks":      0,
			"max_similarity":        0.0,
			"guard_decision":        string(decision.Decision),
			"prompt_type_used":      decision.PromptType,
			"retrieved_chunk_ids":   []string{},
			"smalltalk_detected":    true,
			"retrieval": fiber.Map{
				"chunks":         0,
				"max_similarity": 0.0,
			},
			"guard": fiber.Map{
				"decision": string(decision.Decision),
			},
			"prompt": fiber.Map{
				"type": decision.PromptType,
			},
		}, []string{})
		return nil, nil, decision, nil, answer.PromptBuildOptions{}, nil, nil
	}
	history, err := h.loadConversationHistory(ctx, conversationID)
	if err != nil {
		return nil, nil, guardDecision{}, nil, answer.PromptBuildOptions{}, nil, err
	}

	retrievalQuery := h.Retriever.BuildFollowUpSearchQuery(ctx, history, content)
	retrievalAnalysis := h.Retriever.AnalyzeQuery(ctx, retrievalQuery)
	results, err := h.Retriever.Search(ctx, retrievalQuery, retrieval.SearchOptions{
		TopK:            normalizedFilters.TopK,
		Domain:          normalizedFilters.Domain,
		DocType:         normalizedFilters.DocType,
		EffectiveStatus: normalizedFilters.EffectiveStatus,
		DocumentNumber:  normalizedFilters.DocumentNumber,
		ArticleNumber:   normalizedFilters.ArticleNumber,
	})
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return nil, nil, guardDecision{}, nil, answer.PromptBuildOptions{}, nil, err
	}

	decision, diag, err := h.evaluateGuardPolicy(ctx, runtimeCfg.Policy, results)
	if err != nil {
		traceSvc.OnError(err, traceLatency(started))
		return nil, nil, guardDecision{}, nil, answer.PromptBuildOptions{}, nil, err
	}
	promptTypeUsed := decision.PromptType
	var promptCfg domain.AIPrompt
	if decision.Allow() {
		promptCfg, promptTypeUsed, err = h.getAnswerPrompt(ctx)
		if err != nil {
			traceSvc.OnError(err, traceLatency(started))
			return nil, nil, guardDecision{}, nil, answer.PromptBuildOptions{}, nil, err
		}
	}

	traceSvc.OnRetrieval(retrievalAnalysis.NormalizedQuery, map[string]interface{}{
		"canonical_query":       retrievalAnalysis.CanonicalQuery,
		"matched_doc_types":     retrievalAnalysis.MatchedDocTypes,
		"matched_query_rules":   retrievalAnalysis.MatchedQueryRules,
		"query_profile_hashes":  retrievalAnalysis.QueryProfileHashes,
		"inferred_intent":       retrievalAnalysis.Intent,
		"inferred_legal_domain": retrievalAnalysis.LegalDomain,
		"inferred_legal_topic":  retrievalAnalysis.LegalTopic,
		"conversation_id":       conversationID,
		"retrieval_query":       retrievalQuery,
		"legal_domain":          normalizedFilters.Domain,
		"document_type":         normalizedFilters.DocType,
		"effective_status":      normalizedFilters.EffectiveStatus,
		"document_number":       normalizedFilters.DocumentNumber,
		"article_number":        normalizedFilters.ArticleNumber,
		"retrieved_chunks":      diag.RetrievedChunks,
		"max_similarity":        diag.MaxSimilarity,
		"guard_decision":        string(decision.Decision),
		"prompt_type_used":      promptTypeUsed,
		"retrieved_chunk_ids":   traceChunkIDs(results),
		"retrieval": fiber.Map{
			"chunks":         diag.RetrievedChunks,
			"max_similarity": diag.MaxSimilarity,
		},
		"guard": fiber.Map{
			"decision": string(decision.Decision),
		},
		"prompt": fiber.Map{
			"type": promptTypeUsed,
		},
	}, traceChunkIDs(results))
	if !decision.Allow() {
		return results, history, decision, nil, answer.PromptBuildOptions{}, nil, nil
	}
	sources := h.buildChatAnswerSources(ctx, results)
	promptOpts := answer.PromptBuildOptions{
		MaxInputTokens:   7000,
		ReservedTokens:   max(1200, promptCfg.MaxTokens),
		MaxHistoryTurns:  16,
		MaxSourceChars:   10000,
		MaxQuestionChars: 2000,
	}
	ansSvc := &answer.Service{
		Client:       h.AnswerCli,
		SystemPrompt: promptCfg.SystemPrompt,
		Tone:         runtimeCfg.Tone,
		Temperature:  promptCfg.Temperature,
		MaxTokens:    promptCfg.MaxTokens,
		Retry:        promptCfg.Retry,
	}
	traceSvc.OnPromptSnapshot(ansSvc.PromptSnapshotWithHistory(history, content, sources, promptOpts))
	traceSvc.OnLLMCall(h.AnswerCli.Model, promptCfg.Temperature, promptCfg.MaxTokens, promptCfg.Retry)
	return results, history, decision, sources, promptOpts, ansSvc, nil
}

func (h *Handler) buildChatAnswerSources(ctx context.Context, results []retrieval.Result) []answer.Source {
	sources := make([]answer.Source, 0, len(results))
	versionBundleCache := make(map[string]struct {
		asset domain.DocumentAsset
		doc   domain.Document
	})
	for _, r := range results {
		var bundle struct {
			asset domain.DocumentAsset
			doc   domain.Document
		}
		if cached, ok := versionBundleCache[r.VersionID]; ok {
			bundle = cached
		} else {
			_, doc, asset, _, err := h.Store.GetDocumentVersionBundle(ctx, r.VersionID)
			if err == nil {
				bundle = struct {
					asset domain.DocumentAsset
					doc   domain.Document
				}{asset: asset, doc: doc}
				versionBundleCache[r.VersionID] = bundle
			}
		}

		citation := answer.Citation{
			ID:               r.CitationID,
			ChunkID:          r.ChunkID,
			DocumentTitle:    pickString(r.Metadata, "document_title", "title", "doc_title", "law_name"),
			LawName:          pickString(r.Metadata, "law_name", "document_title", "title", "doc_title"),
			Chapter:          pickString(r.Metadata, "chapter", "chuong"),
			DocumentNumber:   pickString(r.Metadata, "document_number", "number", "doc_number", "doc_code"),
			DocumentType:     pickString(r.Metadata, "document_type", "doc_type"),
			IssuingAuthority: pickString(r.Metadata, "issuing_authority", "authority", "co_quan_ban_hanh"),
			EffectiveStatus:  pickString(r.Metadata, "effective_status", "status", "hieu_luc"),
			Article:          pickString(r.Metadata, "article", "article_number", "dieu"),
			Clause:           pickString(r.Metadata, "clause", "clause_number", "khoan"),
			Year:             pickInt(r.Metadata, "year", "document_year", "signed_year", "nam"),
			Excerpt:          excerptText(r.Text, 320),
			URL:              pickString(r.Metadata, "url", "document_url", "source_url"),
			AssetID:          pickString(r.Metadata, "asset_id"),
			FileURL:          pickString(r.Metadata, "file_url"),
		}
		if citation.DocumentTitle == "" {
			citation.DocumentTitle = bundle.doc.Title
		}
		if citation.LawName == "" {
			citation.LawName = citation.DocumentTitle
		}
		if citation.AssetID == "" {
			citation.AssetID = bundle.asset.ID
		}
		if citation.FileURL == "" && citation.AssetID != "" {
			citation.FileURL = "/assets/" + citation.AssetID + "/download"
		}
		if citation.URL == "" {
			citation.URL = citation.FileURL
		}
		citation.CitationLabel = buildDeterministicCitationLabel(citation)
		sources = append(sources, answer.Source{Text: r.Text, Citation: citation})
	}
	return dedupeSources(sources)
}

func (h *Handler) loadConversationHistory(ctx context.Context, conversationID string) ([]answer.ConversationMessage, error) {
	msgs, err := h.Store.ListMessagesByConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	out := make([]answer.ConversationMessage, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, answer.ConversationMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return out, nil
}

func (h *Handler) buildCitationDetailFromChunk(ctx context.Context, chunk domain.Chunk) (citationDetailResponse, error) {
	var metadata map[string]interface{}
	if len(chunk.MetadataJSON) > 0 {
		if err := json.Unmarshal(chunk.MetadataJSON, &metadata); err != nil {
			return citationDetailResponse{}, err
		}
	}

	citation := answer.Citation{
		ID:               pickString(metadata, "citation_id"),
		ChunkID:          chunk.ID,
		DocumentTitle:    pickString(metadata, "document_title", "title", "doc_title", "law_name"),
		LawName:          pickString(metadata, "law_name", "document_title", "title", "doc_title"),
		Chapter:          pickString(metadata, "chapter", "chuong"),
		DocumentNumber:   pickString(metadata, "document_number", "number", "doc_number", "doc_code"),
		DocumentType:     pickString(metadata, "document_type", "doc_type"),
		IssuingAuthority: pickString(metadata, "issuing_authority", "authority", "co_quan_ban_hanh"),
		EffectiveStatus:  pickString(metadata, "effective_status", "status", "hieu_luc"),
		Article:          pickString(metadata, "article", "article_number", "dieu"),
		Clause:           pickString(metadata, "clause", "clause_number", "khoan"),
		Year:             pickInt(metadata, "year", "document_year", "signed_year", "nam"),
		Excerpt:          excerptText(chunk.Text, 320),
		URL:              pickString(metadata, "url", "document_url", "source_url"),
		AssetID:          pickString(metadata, "asset_id"),
		FileURL:          pickString(metadata, "file_url"),
	}

	var (
		fileName    string
		contentType string
	)
	if chunk.DocumentVersionID != "" {
		_, doc, asset, _, err := h.Store.GetDocumentVersionBundle(ctx, chunk.DocumentVersionID)
		if err == nil {
			if citation.DocumentTitle == "" {
				citation.DocumentTitle = doc.Title
			}
			if citation.LawName == "" {
				citation.LawName = citation.DocumentTitle
			}
			if citation.AssetID == "" {
				citation.AssetID = asset.ID
			}
			fileName = asset.FileName
			contentType = asset.ContentType
		}
	}
	if citation.FileURL == "" && citation.AssetID != "" {
		citation.FileURL = "/assets/" + citation.AssetID + "/download"
	}
	if citation.URL == "" {
		citation.URL = citation.FileURL
	}
	citation.CitationLabel = buildDeterministicCitationLabel(citation)

	return citationDetailResponse{
		Citation:    citation,
		Content:     chunk.Text,
		SourceType:  "chunk",
		FileName:    fileName,
		ContentType: contentType,
	}, nil
}

func (h *Handler) logChatLifecycle(ctx context.Context, eventName, conversationID, messageID, traceID string, results []retrieval.Result, started time.Time, content string, err error) {
	fields := map[string]interface{}{
		"conversation_id":     conversationID,
		"message_id":          messageID,
		"trace_id":            traceID,
		"retrieved_chunk_ids": traceChunkIDs(results),
		"llm_latency_ms":      time.Since(started).Milliseconds(),
		"prompt_tokens_est":   estimateTokensFromText(content),
	}
	if err != nil {
		fields["error"] = err.Error()
		observability.LogError(ctx, h.Logger, "chat", eventName, fields)
		return
	}
	fields["completion_tokens_est"] = estimateTokensFromText(content)
	observability.LogInfo(ctx, h.Logger, "chat", eventName, fields)
}

func (h *Handler) respondChatError(c *fiber.Ctx, err error) error {
	var fiberErr *fiber.Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return respondError(c, 404, "not_found", "resource not found", nil)
	case errors.As(err, &fiberErr):
		return respondError(c, fiberErr.Code, "validation", fiberErr.Message, nil)
	default:
		return respondError(c, 500, "chat_error", "failed to process chat message", err.Error())
	}
}

func toConversationResponse(convo domain.Conversation) conversationResponse {
	resp := conversationResponse{
		ID:            convo.ID,
		Title:         convo.Title,
		UserID:        convo.UserID,
		LastMessageAt: convo.LastMessageAt,
		MessageCount:  convo.MessageCount,
		CreatedAt:     convo.CreatedAt,
		UpdatedAt:     convo.UpdatedAt,
	}
	if convo.LastMessage != nil {
		resp.LastMessage = truncatePreview(*convo.LastMessage, 96)
	}
	return resp
}

func mapMessages(items []domain.Message) []chatMessageResponse {
	resp := make([]chatMessageResponse, 0, len(items))
	for _, item := range items {
		resp = append(resp, toChatMessageResponse(item))
	}
	return resp
}

func toChatMessageResponse(msg domain.Message) chatMessageResponse {
	var citations []answer.Citation
	if len(msg.CitationsJSON) > 0 {
		_ = json.Unmarshal(msg.CitationsJSON, &citations)
	}
	return chatMessageResponse{
		MessageID:      msg.ID,
		ConversationID: msg.ConversationID,
		Role:           msg.Role,
		Content:        msg.Content,
		Citations:      validateCitations(citations),
		TraceID:        msg.TraceID,
		CreatedAt:      msg.CreatedAt,
	}
}

func currentUserID(c *fiber.Ctx) *string {
	identity, ok := auth.GetIdentity(c)
	if !ok || strings.TrimSpace(identity.UserID) == "" {
		return nil
	}
	userID := identity.UserID
	return &userID
}

func generateConversationTitle(question string) string {
	title := strings.TrimSpace(question)
	replacements := []string{
		"Cho toi hoi", "",
		"Xin hoi", "",
		"Tu van", "",
		"Toi muon hoi", "",
		"Cho hoi", "",
	}
	replacer := strings.NewReplacer(replacements...)
	title = strings.TrimSpace(replacer.Replace(title))
	title = sanitizeConversationTitle(title)
	if title == "" {
		return defaultConversationTitle
	}
	return title
}

func sanitizeConversationTitle(title string) string {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	if len(title) > maxConversationTitleLen {
		title = strings.TrimSpace(title[:maxConversationTitleLen])
	}
	return title
}

func normalizeChatFilters(filters ChatFilters, tones map[string]string) ChatFilters {
	_, normalized := normalizeAnswerRequest(answerRequest{Filters: filters}, tones)
	return normalized
}

func marshalCitations(citations []answer.Citation) []byte {
	citations = validateCitations(citations)
	if len(citations) == 0 {
		return []byte("[]")
	}
	raw, err := json.Marshal(citations)
	if err != nil {
		return []byte("[]")
	}
	return raw
}

func validateCitations(citations []answer.Citation) []answer.Citation {
	out := make([]answer.Citation, 0, len(citations))
	seen := map[string]struct{}{}
	for _, citation := range citations {
		if citation.ChunkID == "" {
			citation.ChunkID = citation.ID
		}
		if citation.ID == "" {
			citation.ID = citation.ChunkID
		}
		if citation.FileURL == "" {
			citation.FileURL = citation.URL
		}
		if citation.URL == "" {
			citation.URL = citation.FileURL
		}
		citation.CitationLabel = buildDeterministicCitationLabel(citation)
		key := citation.ChunkID
		if key == "" {
			key = citation.CitationLabel
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, citation)
	}
	return out
}

func buildDeterministicCitationLabel(c answer.Citation) string {
	parts := make([]string, 0, 4)
	if c.LawName != "" {
		parts = append(parts, c.LawName)
	} else if c.DocumentTitle != "" {
		parts = append(parts, c.DocumentTitle)
	}
	if c.Chapter != "" {
		parts = append(parts, "Chuong "+strings.TrimSpace(c.Chapter))
	}
	if c.Article != "" {
		parts = append(parts, "Dieu "+strings.TrimSpace(c.Article))
	}
	if c.Clause != "" {
		parts = append(parts, "Khoan "+strings.TrimSpace(c.Clause))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func dedupeSources(sources []answer.Source) []answer.Source {
	out := make([]answer.Source, 0, len(sources))
	seen := map[string]struct{}{}
	for _, src := range sources {
		key := src.Citation.ChunkID
		if key == "" {
			key = src.Citation.ID
		}
		if key == "" {
			key = src.Citation.CitationLabel
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, src)
	}
	return out
}

func truncatePreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}

func estimateTokensFromText(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return max(1, len(value)/4)
}

func canAccessConversation(convo domain.Conversation, userID *string) bool {
	if userID == nil {
		return convo.UserID == nil
	}
	if convo.UserID == nil {
		return false
	}
	return strings.TrimSpace(*convo.UserID) == strings.TrimSpace(*userID)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
