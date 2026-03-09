package answer

import (
	"context"
	"strconv"
	"strings"
)

type Service struct {
	Client       *Client
	SystemPrompt string
	Tone         string
	Temperature  float64
	MaxTokens    int
	Retry        int
}

type Source struct {
	Text     string
	Citation Citation
}

func (s *Service) Generate(ctx context.Context, question string, sources []Source) (string, error) {
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
		ans, err := s.Client.Answer(ctx, msgs, CompletionConfig{
			Temperature: s.Temperature,
			MaxTokens:   s.MaxTokens,
		})
		if err == nil {
			return ans, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func buildPrompt(question string, sources []Source) string {
	var b strings.Builder
	b.WriteString("Question:\n")
	b.WriteString(question)
	b.WriteString("\n\nSources:\n")
	for i, s := range sources {
		b.WriteString("[Source ")
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString("]\n")
		b.WriteString("Document: ")
		b.WriteString(s.Citation.DocumentTitle)
		b.WriteString("\n")
		if s.Citation.DocumentNumber != "" {
			b.WriteString("Number: ")
			b.WriteString(s.Citation.DocumentNumber)
			b.WriteString("\n")
		}
		if s.Citation.Article != "" {
			b.WriteString("Article: ")
			b.WriteString(s.Citation.Article)
			b.WriteString("\n")
		}
		if s.Citation.Clause != "" {
			b.WriteString("Clause: ")
			b.WriteString(s.Citation.Clause)
			b.WriteString("\n")
		}
		if s.Citation.Year > 0 {
			b.WriteString("Year: ")
			b.WriteString(strconv.Itoa(s.Citation.Year))
			b.WriteString("\n")
		}
		formatted := FormatLegalCitation(s.Citation)
		if formatted != "" {
			b.WriteString("Legal Citation: ")
			b.WriteString(formatted)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(s.Text)
		if i < len(sources)-1 {
			b.WriteString("\n\n")
		}
	}
	b.WriteString("\n\nAnswer in Vietnamese. Use only the sources above and cite legal provisions clearly.")
	return b.String()
}
