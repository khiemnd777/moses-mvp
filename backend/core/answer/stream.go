package answer

import "context"

type Citation struct {
	Text       string `json:"text"`
	CitationID string `json:"citation_id"`
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
		{Role: "system", Content: s.Guard},
		{Role: "system", Content: s.Tone},
		{Role: "user", Content: prompt},
	}
	if err := s.Client.StreamAnswer(ctx, msgs, handler.OnToken); err != nil {
		handler.OnError(err)
		return err
	}
	citations := make([]Citation, 0, len(sources))
	for _, s := range sources {
		citations = append(citations, Citation{Text: s.Text, CitationID: s.CitationID})
	}
	if err := handler.OnCitations(citations); err != nil {
		handler.OnError(err)
		return err
	}
	handler.OnDone()
	return nil
}
