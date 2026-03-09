package answer

import (
	"context"
	"strings"
)

type Service struct {
	Client *Client
	Guard  string
	Tone   string
}

type Source struct {
	Text       string
	CitationID string
}

func (s *Service) Generate(ctx context.Context, question string, sources []Source) (string, error) {
	prompt := buildPrompt(question, sources)
	msgs := []message{
		{Role: "system", Content: s.Guard},
		{Role: "system", Content: s.Tone},
		{Role: "user", Content: prompt},
	}
	return s.Client.Answer(ctx, msgs)
}

func buildPrompt(question string, sources []Source) string {
	var b strings.Builder
	b.WriteString("Question:\n")
	b.WriteString(question)
	b.WriteString("\n\nSources:\n")
	for i, s := range sources {
		b.WriteString("[")
		b.WriteString(s.CitationID)
		b.WriteString("] ")
		b.WriteString(s.Text)
		if i < len(sources)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n\nAnswer with citations like [citation_id].")
	return b.String()
}
