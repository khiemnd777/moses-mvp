package ingest

import (
	"regexp"
	"strings"

	"github.com/khiemnd777/legal_api/core/schema"
	"golang.org/x/text/unicode/norm"
)

type hierarchyLevel string

const (
	levelChapter hierarchyLevel = "chapter"
	levelArticle hierarchyLevel = "article"
	levelClause  hierarchyLevel = "clause"
	levelPoint   hierarchyLevel = "point"
)

var defaultLevelPatterns = map[hierarchyLevel]string{
	levelChapter: `(?im)^\s*(?:chương|chuong)\s*([ivxlcdm0-9]+[a-zA-Z]*)\s*[\.:]?\s*(.*)$`,
	levelArticle: `(?im)^\s*điều\s*([0-9]+[a-zA-Z]*)\s*[\.:]?\s*(.*)$`,
	levelClause:  `(?im)^\s*(?:khoản\s+)?([0-9]+)\s*[\.\)]?\s*(.*)$`,
	levelPoint:   `(?im)^\s*(?:điểm\s+)?(?:[0-9]+\.)?([a-zđ])(?:[\)\.])\s*(.+)?$`,
}

func submatch(matches []string, idx int) string {
	if idx < 0 || idx >= len(matches) {
		return ""
	}
	return strings.TrimSpace(matches[idx])
}

type legalDocument struct {
	Articles []articleNode
}

type articleNode struct {
	Chapter string
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

type legalStructureParser struct {
	levels        []hierarchyLevel
	normalization string
	patterns      map[hierarchyLevel]*regexp.Regexp
}

func newLegalStructureParser(rules schema.SegmentRules) legalStructureParser {
	levels := parseHierarchyLevels(rules.Hierarchy)
	patterns := compileLevelPatterns(rules.LevelPatterns)
	normalization := strings.TrimSpace(strings.ToLower(rules.Normalization))
	if normalization == "" {
		normalization = "basic"
	}
	return legalStructureParser{
		levels:        levels,
		normalization: normalization,
		patterns:      patterns,
	}
}

func parseHierarchyLevels(hierarchy string) []hierarchyLevel {
	hierarchy = strings.TrimSpace(strings.ToLower(hierarchy))
	if hierarchy == "" {
		return []hierarchyLevel{levelArticle, levelClause, levelPoint}
	}
	parts := strings.FieldsFunc(hierarchy, func(r rune) bool {
		return r == '>' || r == ',' || r == '/' || r == '|' || r == ';'
	})
	out := make([]hierarchyLevel, 0, len(parts))
	seen := map[hierarchyLevel]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch hierarchyLevel(part) {
		case levelChapter, levelArticle, levelClause, levelPoint:
			lv := hierarchyLevel(part)
			if _, ok := seen[lv]; ok {
				continue
			}
			seen[lv] = struct{}{}
			out = append(out, lv)
		}
	}
	if len(out) == 0 {
		return []hierarchyLevel{levelArticle, levelClause, levelPoint}
	}
	return out
}

func compileLevelPatterns(overrides map[string]string) map[hierarchyLevel]*regexp.Regexp {
	patterns := make(map[hierarchyLevel]*regexp.Regexp, len(defaultLevelPatterns))
	for lv, raw := range defaultLevelPatterns {
		patterns[lv] = regexp.MustCompile(raw)
	}
	for k, pattern := range overrides {
		lv := hierarchyLevel(strings.TrimSpace(strings.ToLower(k)))
		if _, ok := patterns[lv]; !ok {
			continue
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		patterns[lv] = compiled
	}
	return patterns
}

func (p legalStructureParser) has(level hierarchyLevel) bool {
	for _, lv := range p.levels {
		if lv == level {
			return true
		}
	}
	return false
}

func (p legalStructureParser) Parse(text string) legalDocument {
	if len(p.levels) == 0 || len(p.patterns) == 0 {
		p = newLegalStructureParser(schema.SegmentRules{
			Strategy:      "legal_article",
			Hierarchy:     "article>clause>point",
			Normalization: "basic",
		})
	}
	normalized := p.normalize(text)
	lines := strings.Split(normalized, "\n")
	articles := make([]articleNode, 0)

	currentChapter := ""
	current := articleNode{}
	hasArticle := false
	buffer := make([]string, 0)
	flushArticle := func() {
		if !hasArticle && len(buffer) == 0 {
			return
		}
		if !hasArticle {
			current = articleNode{
				Chapter: currentChapter,
				Header:  "",
				Number:  "",
				Content: joinNonEmpty(buffer),
			}
		} else {
			current.Content = joinNonEmpty(buffer)
		}
		current.Clauses = p.parseClauses(current.Content)
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

		if p.has(levelChapter) {
			if matches := p.patterns[levelChapter].FindStringSubmatch(trimmed); matches != nil {
				chapter := submatch(matches, 1)
				if chapter == "" {
					chapter = strings.TrimSpace(trimmed)
				}
				currentChapter = chapter
				continue
			}
		}

		if p.has(levelArticle) {
			if matches := p.patterns[levelArticle].FindStringSubmatch(trimmed); matches != nil {
				flushArticle()
				number := submatch(matches, 1)
				current = articleNode{
					Chapter: currentChapter,
					Number:  number,
					Header:  trimmed,
				}
				hasArticle = true
				continue
			}
		}

		buffer = append(buffer, trimmed)
	}

	flushArticle()
	if len(articles) == 0 {
		articles = append(articles, articleNode{
			Chapter: currentChapter,
			Content: normalized,
			Clauses: p.parseClauses(normalized),
		})
	}
	return legalDocument{Articles: articles}
}

func (p legalStructureParser) parseClauses(text string) []clauseNode {
	if !p.has(levelClause) {
		return nil
	}
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
		current.Points = p.parsePoints(current.Content)
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

		if matches := p.patterns[levelClause].FindStringSubmatch(trimmed); matches != nil {
			flushClause()
			number := submatch(matches, 1)
			if number == "" {
				buffer = append(buffer, trimmed)
				continue
			}
			current = clauseNode{Number: number}
			hasClause = true
			if tail := submatch(matches, 2); tail != "" {
				buffer = append(buffer, tail)
			}
			continue
		}

		buffer = append(buffer, trimmed)
	}

	flushClause()
	return clauses
}

func (p legalStructureParser) parsePoints(text string) []pointNode {
	if !p.has(levelPoint) {
		return nil
	}
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
		if matches := p.patterns[levelPoint].FindStringSubmatch(trimmed); matches != nil {
			flushPoint()
			marker := submatch(matches, 1)
			if marker == "" {
				buffer = append(buffer, trimmed)
				continue
			}
			current = pointNode{Marker: marker}
			hasPoint = true
			if tail := submatch(matches, 2); tail != "" {
				buffer = append(buffer, tail)
			}
			continue
		}
		buffer = append(buffer, trimmed)
	}

	flushPoint()
	return points
}

func (p legalStructureParser) normalize(in string) string {
	switch p.normalization {
	case "none":
		return strings.TrimSpace(strings.ReplaceAll(in, "\r", ""))
	default:
		return normalizeLegalText(in)
	}
}

func normalizeLegalText(in string) string {
	in = norm.NFC.String(in)
	replacer := strings.NewReplacer(
		"\u00A0", " ",
		"Ð", "Đ",
		"ð", "đ",
		"Ðiều", "Điều",
		"ÐIỀU", "Điều",
	)
	in = replacer.Replace(in)
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
