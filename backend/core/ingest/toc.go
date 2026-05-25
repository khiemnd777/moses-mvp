package ingest

import (
	"regexp"
	"strings"

	"github.com/khiemnd777/legal_api/core/language"
)

var (
	tocPageSuffixPattern  = regexp.MustCompile(`(?:\.{2,}|…+)\s*\d+\s*$`)
	articleHeadingPattern = regexp.MustCompile(`(?im)^\s*(?:điều|dieu)\s+[0-9]+`)
	chapterHeadingPattern = regexp.MustCompile(`(?im)^\s*(?:chương|chuong)\s+[ivxlcdm0-9]+`)
)

func stripVietnameseLegalTOC(text string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if i > 80 {
			break
		}
		if isVietnameseTOCHeading(line) {
			start = i
			break
		}
	}
	if start < 0 {
		return text
	}

	end := -1
	sawEntry := false
	for i := start + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if isLikelyTOCEntry(line) {
			sawEntry = true
			continue
		}
		if isLikelyLegalBodyBoundary(line, nextNonEmptyLine(lines[i+1:])) {
			end = i
			break
		}
		if sawEntry && !isTOCContinuation(line) {
			end = i
			break
		}
	}

	if end < 0 {
		out := append([]string{}, lines[:start]...)
		out = append(out, lines[start+1:]...)
		return strings.TrimSpace(strings.Join(out, "\n"))
	}
	out := append([]string{}, lines[:start]...)
	out = append(out, lines[end:]...)
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func isVietnameseTOCHeading(line string) bool {
	key := language.SearchKey(line)
	return key == "muc luc" || strings.HasPrefix(key, "muc luc ")
}

func isLikelyTOCEntry(line string) bool {
	key := language.SearchKey(line)
	if key == "trang" || strings.HasPrefix(key, "trang ") || strings.HasPrefix(key, "page ") {
		return true
	}
	if tocPageSuffixPattern.MatchString(line) {
		return true
	}
	if strings.HasPrefix(key, "dieu ") || strings.HasPrefix(key, "chuong ") || strings.HasPrefix(key, "muc ") || strings.HasPrefix(key, "phan ") {
		return strings.HasSuffix(key, " 1") || strings.Contains(line, "\t")
	}
	return false
}

func isTOCContinuation(line string) bool {
	key := language.SearchKey(line)
	if key == "" {
		return true
	}
	if strings.HasPrefix(key, "dieu ") || strings.HasPrefix(key, "chuong ") || strings.HasPrefix(key, "muc ") || strings.HasPrefix(key, "phan ") {
		return true
	}
	return tocPageSuffixPattern.MatchString(line)
}

func isLikelyLegalBodyBoundary(line, next string) bool {
	key := language.SearchKey(line)
	switch {
	case strings.HasPrefix(key, "can cu "):
		return true
	case strings.HasPrefix(key, "quoc hoi"), strings.HasPrefix(key, "chinh phu"), strings.HasPrefix(key, "bo "):
		return true
	case strings.HasPrefix(key, "luat "), strings.HasPrefix(key, "bo luat "), strings.HasPrefix(key, "nghi dinh "), strings.HasPrefix(key, "nghi quyet "), strings.HasPrefix(key, "thong tu "):
		return true
	case articleHeadingPattern.MatchString(line):
		return next == "" || !isLikelyTOCEntry(next)
	case chapterHeadingPattern.MatchString(line):
		return next == "" || !isLikelyTOCEntry(next)
	default:
		return false
	}
}

func nextNonEmptyLine(lines []string) string {
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
