package answer

import (
	"context"
	"strconv"
	"strings"
)

type Citation struct {
	ID               string `json:"id"`
	DocumentTitle    string `json:"document_title"`
	LawName          string `json:"law_name,omitempty"`
	Chapter          string `json:"chapter,omitempty"`
	DocumentNumber   string `json:"document_number"`
	DocumentType     string `json:"document_type,omitempty"`
	IssuingAuthority string `json:"issuing_authority,omitempty"`
	EffectiveStatus  string `json:"effective_status,omitempty"`
	Article          string `json:"article"`
	Clause           string `json:"clause"`
	Year             int    `json:"year"`
	CitationLabel    string `json:"citation_label,omitempty"`
	Excerpt          string `json:"excerpt"`
	URL              string `json:"url"`
	ChunkID          string `json:"chunk_id,omitempty"`
	AssetID          string `json:"asset_id,omitempty"`
	FileURL          string `json:"file_url,omitempty"`
}

func FormatLegalCitation(c Citation) string {
	parts := make([]string, 0, 4)
	if c.Article != "" {
		parts = append(parts, "Điều "+c.Article)
	}
	if c.DocumentTitle != "" {
		parts = append(parts, c.DocumentTitle)
	}
	if c.Year > 0 {
		parts = append(parts, strconv.Itoa(c.Year))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

type StreamHandler interface {
	OnToken(delta string) error
	OnCitations(citations []Citation) error
	OnError(err error)
	OnDone()
}

func (s *Service) Stream(ctx context.Context, question string, sources []Source, handler StreamHandler) error {
	msgs := s.BuildMessages(question, sources)
	return s.streamWithMessages(ctx, msgs, sources, handler)
}

func (s *Service) StreamWithHistory(ctx context.Context, history []ConversationMessage, question string, sources []Source, opts PromptBuildOptions, handler StreamHandler) error {
	msgs := s.BuildConversationMessages(history, question, sources, opts)
	return s.streamWithMessages(ctx, msgs, sources, handler)
}

func (s *Service) streamWithMessages(ctx context.Context, msgs []message, sources []Source, handler StreamHandler) error {
	retryCount := s.Retry
	if retryCount < 0 {
		retryCount = 0
	}
	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		emitted := false
		err := s.Client.StreamAnswer(ctx, msgs, CompletionConfig{
			Temperature: s.Temperature,
			MaxTokens:   s.MaxTokens,
		}, func(delta string) error {
			emitted = true
			return handler.OnToken(delta)
		})
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		if emitted {
			handler.OnError(err)
			return err
		}
	}
	if lastErr != nil {
		handler.OnError(lastErr)
		return lastErr
	}
	citations := make([]Citation, 0, len(sources))
	for _, s := range sources {
		citations = append(citations, s.Citation)
	}
	if err := handler.OnCitations(citations); err != nil {
		handler.OnError(err)
		return err
	}
	handler.OnDone()
	return nil
}
