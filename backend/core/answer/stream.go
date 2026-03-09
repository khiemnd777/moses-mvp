package answer

import (
	"context"
	"strconv"
	"strings"
)

type Citation struct {
	ID             string `json:"id"`
	DocumentTitle  string `json:"document_title"`
	DocumentNumber string `json:"document_number"`
	Article        string `json:"article"`
	Clause         string `json:"clause"`
	Year           int    `json:"year"`
	Excerpt        string `json:"excerpt"`
	URL            string `json:"url"`
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
	prompt := buildPrompt(question, sources)
	msgs := []message{
		{Role: "system", Content: s.SystemPrompt},
		{Role: "system", Content: s.Tone},
		{Role: "user", Content: prompt},
	}
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
