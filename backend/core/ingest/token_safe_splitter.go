package ingest

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxChunkTokens    = 1200
	defaultTargetChunkTokens = 700
	defaultOverlapTokens     = 100
	hardAbortChunkTokens     = 7000
)

type tokenSafeSplitter struct {
	maxTokens    int
	targetTokens int
}

type chunkPart struct {
	Text string
	Path structuralPath
}

func (s tokenSafeSplitter) Split(text string, path structuralPath) ([]chunkPart, error) {
	if tokens := estimateTokenCount(text); tokens > hardAbortChunkTokens {
		return nil, fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
	}
	return s.splitRecursive(text, path, splitBySentences, splitByParagraphs)
}

func (s tokenSafeSplitter) splitRecursive(text string, path structuralPath, splitters ...func(string) []string) ([]chunkPart, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	if tokens := estimateTokenCount(text); tokens <= s.maxTokens {
		return []chunkPart{{Text: text, Path: path}}, nil
	} else if tokens > hardAbortChunkTokens {
		return nil, fmt.Errorf("chunk exceeds hard safety limit: estimated_tokens=%d limit=%d", tokens, hardAbortChunkTokens)
	}

	for idx, splitter := range splitters {
		pieces := splitter(text)
		if len(pieces) <= 1 {
			continue
		}
		grouped := groupByTokenBudget(pieces, s.targetTokens)
		if len(grouped) <= 1 {
			continue
		}
		out := make([]chunkPart, 0, len(grouped))
		for _, groupedText := range grouped {
			split, err := s.splitRecursive(groupedText, path, splitters[idx+1:]...)
			if err != nil {
				return nil, err
			}
			out = append(out, split...)
		}
		return out, nil
	}

	return nil, fmt.Errorf("unable to split chunk below token limit: estimated_tokens=%d limit=%d", estimateTokenCount(text), s.maxTokens)
}

func estimateTokenCount(text string) int {
	words := len(strings.Fields(text))
	runes := utf8.RuneCountInString(text)
	if words == 0 {
		return 0
	}
	conservative := runes / 3
	if conservative > words {
		return conservative
	}
	return words
}

func groupByTokenBudget(parts []string, target int) []string {
	if target <= 0 {
		target = defaultTargetChunkTokens
	}
	out := make([]string, 0, len(parts))
	current := make([]string, 0)
	currentTokens := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		partTokens := estimateTokenCount(part)
		if len(current) > 0 && currentTokens+partTokens > target {
			out = append(out, strings.TrimSpace(strings.Join(current, "\n")))
			current = current[:0]
			currentTokens = 0
		}
		current = append(current, part)
		currentTokens += partTokens
	}
	if len(current) > 0 {
		out = append(out, strings.TrimSpace(strings.Join(current, "\n")))
	}
	return out
}

func splitBySentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Không split theo ":" vì văn bản pháp lý tiếng Việt dùng ":" rất nhiều
	// ở tiêu đề, Điều, Khoản, Điểm, căn cứ, quyết định...
	replacer := strings.NewReplacer(
		";", ";\n",
		"!", "!\n",
		"?", "?\n",
		". ", ".\n",
	)
	text = replacer.Replace(text)

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func splitByParagraphs(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{text}
	}
	return out
}
