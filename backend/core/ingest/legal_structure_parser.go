package ingest

import (
	"strings"

	"github.com/khiemnd777/legal_api/core/schema"
	"golang.org/x/text/unicode/norm"
)

func submatch(matches []string, idx int) string {
	if idx < 0 || idx >= len(matches) {
		return ""
	}
	return strings.TrimSpace(matches[idx])
}

type legalDocument struct {
	Nodes    []segmentNode
	Articles []articleNode
}

type segmentNode struct {
	Level    string
	Value    string
	Header   string
	Content  string
	Path     structuralPath
	Children []segmentNode
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
	plan segmentPlan
}

func newLegalStructureParser(rules schema.SegmentRules) (legalStructureParser, error) {
	plan, err := compileSegmentPlan(rules)
	if err != nil {
		return legalStructureParser{}, err
	}
	return legalStructureParser{plan: plan}, nil
}

func (p legalStructureParser) Parse(text string) legalDocument {
	normalized := p.normalize(text)
	if len(p.plan.Levels) == 0 {
		raw := segmentNode{
			Content: normalized,
			Path:    newStructuralPath(nil),
		}
		return legalDocument{
			Nodes:    []segmentNode{raw},
			Articles: projectArticles([]segmentNode{raw}),
		}
	}
	lines := strings.Split(normalized, "\n")
	paths := newStructuralPath(planLevelNames(p.plan))
	nodes := p.parseLevel(lines, 0, paths)
	if len(nodes) == 0 {
		raw := segmentNode{
			Content: normalized,
			Path:    paths,
		}
		nodes = []segmentNode{raw}
	}
	return legalDocument{
		Nodes:    nodes,
		Articles: projectArticles(nodes),
	}
}

func (p legalStructureParser) parseLevel(lines []string, depth int, parentPath structuralPath) []segmentNode {
	if depth >= len(p.plan.Levels) {
		return nil
	}
	level := p.plan.Levels[depth]
	type section struct {
		header string
		value  string
		body   []string
	}
	sections := make([]section, 0)
	preamble := make([]string, 0)
	current := section{}
	hasCurrent := false

	flushCurrent := func() {
		if !hasCurrent {
			return
		}
		sections = append(sections, current)
		current = section{}
		hasCurrent = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := level.Pattern.FindStringSubmatch(trimmed); matches != nil {
			flushCurrent()
			current = section{
				header: trimmed,
				value:  submatch(matches, 1),
				body:   make([]string, 0),
			}
			if depth >= len(p.plan.Levels)-2 {
				if tail := submatch(matches, 2); tail != "" {
					current.body = append(current.body, tail)
				}
			}
			hasCurrent = true
			continue
		}
		if hasCurrent {
			current.body = append(current.body, trimmed)
			continue
		}
		preamble = append(preamble, trimmed)
	}
	flushCurrent()

	nodes := make([]segmentNode, 0, len(sections)+1)
	preambleText := joinNonEmpty(preamble)
	if preambleText != "" && len(sections) == 0 {
		rawPath := parentPath
		rawNode := segmentNode{
			Content: preambleText,
			Path:    rawPath,
		}
		if depth+1 < len(p.plan.Levels) {
			rawNode.Children = p.parseLevel(strings.Split(preambleText, "\n"), depth+1, rawPath)
		}
		nodes = append(nodes, rawNode)
	}
	for _, section := range sections {
		path := parentPath.With(level.Name, section.value)
		content := joinNonEmpty(section.body)
		node := segmentNode{
			Level:   level.Name,
			Value:   section.value,
			Header:  section.header,
			Content: content,
			Path:    path,
		}
		if depth+1 < len(p.plan.Levels) && content != "" {
			node.Children = p.parseLevel(strings.Split(content, "\n"), depth+1, path)
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func projectArticles(nodes []segmentNode) []articleNode {
	articles := make([]articleNode, 0)
	var walk func([]segmentNode)
	walk = func(items []segmentNode) {
		for _, node := range items {
			if node.Level == "article" {
				articles = append(articles, articleNode{
					Chapter: node.Path.Value("chapter"),
					Number:  node.Value,
					Header:  node.Header,
					Content: node.Content,
					Clauses: projectClauses(node.Children),
				})
			}
			if len(node.Children) > 0 {
				walk(node.Children)
			}
		}
	}
	walk(nodes)
	if len(articles) == 0 && len(nodes) > 0 {
		articles = append(articles, articleNode{
			Chapter: nodes[0].Path.Value("chapter"),
			Content: nodes[0].Content,
			Clauses: projectClauses(nodes[0].Children),
		})
	}
	return articles
}

func projectClauses(nodes []segmentNode) []clauseNode {
	clauses := make([]clauseNode, 0)
	for _, node := range nodes {
		if node.Level != "clause" {
			continue
		}
		clauses = append(clauses, clauseNode{
			Number:  node.Value,
			Content: node.Content,
			Points:  projectPoints(node.Children),
		})
	}
	return clauses
}

func projectPoints(nodes []segmentNode) []pointNode {
	points := make([]pointNode, 0)
	for _, node := range nodes {
		if node.Level != "point" {
			continue
		}
		points = append(points, pointNode{
			Marker:  node.Value,
			Content: node.Content,
		})
	}
	return points
}

func planLevelNames(plan segmentPlan) []string {
	out := make([]string, 0, len(plan.Levels))
	for _, level := range plan.Levels {
		out = append(out, level.Name)
	}
	return out
}

func (p legalStructureParser) normalize(in string) string {
	switch p.plan.Normalization {
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
