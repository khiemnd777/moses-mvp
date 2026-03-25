package ingest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/khiemnd777/legal_api/core/schema"
)

type levelPlan struct {
	Name    string
	Order   int
	Pattern *regexp.Regexp
	Raw     string
}

type segmentPlan struct {
	Strategy      string
	Normalization string
	Levels        []levelPlan
}

type structuralPath struct {
	order  []string
	values map[string]string
}

func newStructuralPath(order []string) structuralPath {
	clone := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, level := range order {
		level = strings.TrimSpace(strings.ToLower(level))
		if level == "" {
			continue
		}
		if _, ok := seen[level]; ok {
			continue
		}
		seen[level] = struct{}{}
		clone = append(clone, level)
	}
	return structuralPath{
		order:  clone,
		values: map[string]string{},
	}
}

func (p structuralPath) With(level, value string) structuralPath {
	next := structuralPath{
		order:  append([]string(nil), p.order...),
		values: make(map[string]string, len(p.values)+1),
	}
	for k, v := range p.values {
		next.values[k] = v
	}
	level = strings.TrimSpace(strings.ToLower(level))
	value = strings.TrimSpace(value)
	if level != "" && value != "" {
		next.values[level] = value
	}
	return next
}

func (p structuralPath) Value(level string) string {
	return strings.TrimSpace(p.values[strings.TrimSpace(strings.ToLower(level))])
}

func (p structuralPath) StructuralMap() map[string]interface{} {
	out := map[string]interface{}{}
	for _, level := range p.order {
		if value := strings.TrimSpace(p.values[level]); value != "" {
			out[level] = value
		}
	}
	for level, value := range p.values {
		if value == "" {
			continue
		}
		if _, ok := out[level]; !ok {
			out[level] = value
		}
	}
	return out
}

func (p structuralPath) Order() []string {
	return append([]string(nil), p.order...)
}

var defaultLevelPatterns = map[string]string{
	"part":    `(?im)^\s*(?:phần)\s+(?:thứ\s+)?([a-zà-ỹivxlcdm0-9]+[a-zà-ỹ]*)\s*[\.:]?\s*(.*)$`,
	"chapter": `(?im)^\s*(?:chương|chuong)\s*([ivxlcdm0-9]+[a-zA-Z]*)\s*[\.:]?\s*(.*)$`,
	"article": `(?im)^\s*điều\s*([0-9]+[a-zA-Z]*)\s*[\.:]?\s*(.*)$`,
	"clause":  `(?im)^\s*(?:khoản\s+)?([0-9]+)\s*[\.\)]?\s*(.*)$`,
	"point":   `(?im)^\s*(?:điểm\s+)?(?:[0-9]+\.)?([a-zđ])(?:[\)\.])\s*(.+)?$`,
}

var defaultLegalHierarchy = []string{"article", "clause", "point"}

func compileSegmentPlan(rules schema.SegmentRules) (segmentPlan, error) {
	strategy := strings.TrimSpace(strings.ToLower(rules.Strategy))
	if strategy == "" {
		return segmentPlan{}, fmt.Errorf("segment_rules.strategy is required")
	}
	normalization := strings.TrimSpace(strings.ToLower(rules.Normalization))
	if normalization == "" {
		normalization = "basic"
	}
	levels := parseHierarchyLevels(rules.Hierarchy)
	if len(levels) == 0 && strategy == "legal_article" {
		levels = append([]string(nil), defaultLegalHierarchy...)
	}
	compiled := make([]levelPlan, 0, len(levels))
	for idx, level := range levels {
		rawPattern := strings.TrimSpace(rules.LevelPatterns[level])
		if rawPattern == "" {
			rawPattern = defaultLevelPatterns[level]
		}
		if rawPattern == "" {
			return segmentPlan{}, fmt.Errorf("segment_rules.level_patterns[%q] is required", level)
		}
		pattern, err := regexp.Compile(rawPattern)
		if err != nil {
			return segmentPlan{}, fmt.Errorf("segment_rules.level_patterns[%q] is invalid: %w", level, err)
		}
		compiled = append(compiled, levelPlan{
			Name:    level,
			Order:   idx,
			Pattern: pattern,
			Raw:     rawPattern,
		})
	}
	return segmentPlan{
		Strategy:      strategy,
		Normalization: normalization,
		Levels:        compiled,
	}, nil
}

func parseHierarchyLevels(hierarchy string) []string {
	hierarchy = strings.TrimSpace(strings.ToLower(hierarchy))
	if hierarchy == "" || hierarchy == "none" {
		return nil
	}
	parts := strings.FieldsFunc(hierarchy, func(r rune) bool {
		return r == '>' || r == ',' || r == '/' || r == '|' || r == ';' || r == '.'
	})
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
