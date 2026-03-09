package ingest

import "strings"

type chunkOverlapEngine struct {
	overlapTokens int
}

func (e chunkOverlapEngine) Apply(parts []chunkPart) []chunkPart {
	if e.overlapTokens <= 0 || len(parts) < 2 {
		return parts
	}
	out := make([]chunkPart, 0, len(parts))
	prevSource := ""
	for i, part := range parts {
		current := part
		if i > 0 {
			overlap := trailingTokens(prevSource, e.overlapTokens)
			if overlap != "" {
				current.Text = strings.TrimSpace(overlap + "\n" + current.Text)
			}
		}
		out = append(out, current)
		prevSource = part.Text
	}
	return out
}

func trailingTokens(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	if len(words) <= limit {
		return strings.Join(words, " ")
	}
	return strings.Join(words[len(words)-limit:], " ")
}
