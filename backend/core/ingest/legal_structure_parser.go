package ingest

import (
	"regexp"
	"strings"
)

var (
	articleHeaderPattern = regexp.MustCompile(`(?i)^Điều\s+([0-9]+[a-zA-Z]*)\b(.*)$`)
	clausePattern        = regexp.MustCompile(`^([0-9]+)\.\s*(.*)$`)
	pointPattern         = regexp.MustCompile(`^(?:[0-9]+\.)?([a-zđ])(?:[\)\.])\s*(.*)$`)
)

type legalDocument struct {
	Articles []articleNode
}

type articleNode struct {
	Number  string
	Header  string
	Content string
	Clauses []clauseNode
}

type clauseNode struct {
	Number  string
	Content string
	Points  []pointNode
}

type pointNode struct {
	Marker  string
	Content string
}

type legalStructureParser struct{}

func (p legalStructureParser) Parse(text string) legalDocument {
	normalized := normalizeLegalText(text)
	lines := strings.Split(normalized, "\n")
	articles := make([]articleNode, 0)

	current := articleNode{}
	hasArticle := false
	buffer := make([]string, 0)
	flushArticle := func() {
		if !hasArticle && len(buffer) == 0 {
			return
		}
		if !hasArticle {
			current = articleNode{
				Header:  "",
				Number:  "",
				Content: joinNonEmpty(buffer),
			}
		} else {
			current.Content = joinNonEmpty(buffer)
		}
		current.Clauses = parseClauses(current.Content)
		articles = append(articles, current)
		buffer = buffer[:0]
		current = articleNode{}
		hasArticle = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(buffer) > 0 && buffer[len(buffer)-1] != "" {
				buffer = append(buffer, "")
			}
			continue
		}

		if matches := articleHeaderPattern.FindStringSubmatch(trimmed); matches != nil {
			flushArticle()
			current = articleNode{
				Number: strings.TrimSpace(matches[1]),
				Header: trimmed,
			}
			hasArticle = true
			continue
		}

		buffer = append(buffer, trimmed)
	}

	flushArticle()
	if len(articles) == 0 {
		articles = append(articles, articleNode{
			Content: normalized,
			Clauses: parseClauses(normalized),
		})
	}
	return legalDocument{Articles: articles}
}

func parseClauses(text string) []clauseNode {
	lines := strings.Split(text, "\n")
	clauses := make([]clauseNode, 0)
	current := clauseNode{}
	hasClause := false
	buffer := make([]string, 0)

	flushClause := func() {
		content := joinNonEmpty(buffer)
		if !hasClause {
			if content == "" {
				buffer = buffer[:0]
				return
			}
			current = clauseNode{Content: content}
		} else {
			current.Content = content
		}
		current.Points = parsePoints(current.Content)
		clauses = append(clauses, current)
		buffer = buffer[:0]
		current = clauseNode{}
		hasClause = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(buffer) > 0 && buffer[len(buffer)-1] != "" {
				buffer = append(buffer, "")
			}
			continue
		}

		if matches := clausePattern.FindStringSubmatch(trimmed); matches != nil {
			flushClause()
			current = clauseNode{Number: strings.TrimSpace(matches[1])}
			hasClause = true
			if tail := strings.TrimSpace(matches[2]); tail != "" {
				buffer = append(buffer, tail)
			}
			continue
		}

		buffer = append(buffer, trimmed)
	}

	flushClause()
	return clauses
}

func parsePoints(text string) []pointNode {
	lines := strings.Split(text, "\n")
	points := make([]pointNode, 0)
	current := pointNode{}
	hasPoint := false
	buffer := make([]string, 0)

	flushPoint := func() {
		if !hasPoint {
			buffer = buffer[:0]
			return
		}
		current.Content = joinNonEmpty(buffer)
		points = append(points, current)
		buffer = buffer[:0]
		current = pointNode{}
		hasPoint = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(buffer) > 0 && buffer[len(buffer)-1] != "" {
				buffer = append(buffer, "")
			}
			continue
		}
		if matches := pointPattern.FindStringSubmatch(trimmed); matches != nil {
			flushPoint()
			current = pointNode{Marker: strings.TrimSpace(matches[1])}
			hasPoint = true
			if tail := strings.TrimSpace(matches[2]); tail != "" {
				buffer = append(buffer, tail)
			}
			continue
		}
		buffer = append(buffer, trimmed)
	}

	flushPoint()
	return points
}

func normalizeLegalText(in string) string {
	in = strings.ReplaceAll(in, "\r", "")
	lines := strings.Split(in, "\n")
	normalized := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line == "" {
			if !lastBlank {
				normalized = append(normalized, "")
			}
			lastBlank = true
			continue
		}
		normalized = append(normalized, line)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func joinNonEmpty(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if !lastBlank && len(out) > 0 {
				out = append(out, "")
			}
			lastBlank = true
			continue
		}
		out = append(out, line)
		lastBlank = false
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
