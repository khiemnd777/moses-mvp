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
	msgs := s.BuildMessages(question, sources)
	return s.generateWithMessages(ctx, msgs)
}

func (s *Service) GenerateWithHistory(ctx context.Context, history []ConversationMessage, question string, sources []Source, opts PromptBuildOptions) (string, error) {
	msgs := s.BuildConversationMessages(history, question, sources, opts)
	return s.generateWithMessages(ctx, msgs)
}

func (s *Service) generateWithMessages(ctx context.Context, msgs []message) (string, error) {
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

func (s *Service) BuildMessages(question string, sources []Source) []message {
	prompt := buildPrompt(question, sources)
	return []message{
		{Role: "system", Content: s.SystemPrompt},
		{Role: "system", Content: s.Tone},
		{Role: "user", Content: prompt},
	}
}

func (s *Service) PromptSnapshot(question string, sources []Source) string {
	return strings.TrimSpace(strings.Join([]string{
		"System Prompt:\n" + strings.TrimSpace(s.SystemPrompt),
		"Tone Prompt:\n" + strings.TrimSpace(s.Tone),
		"User Query and Context:\n" + strings.TrimSpace(buildPrompt(question, sources)),
	}, "\n\n"))
}

func buildPrompt(question string, sources []Source) string {
	var b strings.Builder
	b.WriteString("User Question:\n")
	b.WriteString(question)
	b.WriteString("\n\nLegal Context:\n")
	seen := map[string]struct{}{}
	sourceIndex := 0
	for _, s := range sources {
		sourceKey := strings.TrimSpace(s.Citation.ID)
		if sourceKey != "" {
			if _, ok := seen[sourceKey]; ok {
				continue
			}
			seen[sourceKey] = struct{}{}
		}
		sourceIndex++
		b.WriteString("[Source ")
		b.WriteString(strconv.Itoa(sourceIndex))
		b.WriteString("]\n")
		b.WriteString("Document Title: ")
		b.WriteString(s.Citation.DocumentTitle)
		b.WriteString("\n")
		if s.Citation.DocumentNumber != "" {
			b.WriteString("Document Number: ")
			b.WriteString(s.Citation.DocumentNumber)
			b.WriteString("\n")
		}
		if s.Citation.DocumentType != "" {
			b.WriteString("Document Type: ")
			b.WriteString(s.Citation.DocumentType)
			b.WriteString("\n")
		}
		if s.Citation.IssuingAuthority != "" {
			b.WriteString("Issuing Authority: ")
			b.WriteString(s.Citation.IssuingAuthority)
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
		if s.Citation.EffectiveStatus != "" {
			b.WriteString("Effective Status: ")
			b.WriteString(s.Citation.EffectiveStatus)
			b.WriteString("\n")
		}
		formatted := strings.TrimSpace(s.Citation.CitationLabel)
		if formatted == "" {
			formatted = FormatLegalCitation(s.Citation)
		}
		if formatted != "" {
			b.WriteString("Citation Label: ")
			b.WriteString(formatted)
			b.WriteString("\n")
		}
		b.WriteString("Excerpt:\n")
		b.WriteString(s.Text)
		b.WriteString("\n\n")
	}
	b.WriteString("Instructions:\n")
	b.WriteString("You are a Vietnamese legal assistant. Use only the Legal Context above.\n")
	b.WriteString("Do not invent legal provisions or facts not present in the context.\n")
	b.WriteString("Cite legal provisions in human-readable form and tie each conclusion to retrieved sources.\n")
	b.WriteString("If facts or legal basis are missing, refuse and ask for additional legal evidence.\n")
	b.WriteString("Answer in Vietnamese with this exact structure:\n")
	b.WriteString("1. Legal Issue\n")
	b.WriteString("2. Applicable Law\n")
	b.WriteString("3. Legal Analysis\n")
	b.WriteString("4. Conclusion\n")
	return b.String()
}
