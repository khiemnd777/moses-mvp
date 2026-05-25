package language

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	legacyVietnameseRuneReplacer = strings.NewReplacer(
		"Ð", "Đ",
		"ð", "đ",
	)
	accentFoldRuneReplacer = strings.NewReplacer(
		"Đ", "D",
		"đ", "d",
		"Ð", "D",
		"ð", "d",
	)
	searchSeparatorReplacer = strings.NewReplacer(
		"_", " ",
		"-", " ",
		"–", " ",
		"—", " ",
	)
	documentNumberDashReplacer = strings.NewReplacer(
		"–", "-",
		"—", "-",
		"−", "-",
	)
	legalAbbreviationAliases = map[string]string{
		"nd cp":                         "NĐ-CP",
		"nghi dinh":                     "NĐ-CP",
		"nghi dinh chinh phu":           "NĐ-CP",
		"nq hdtp":                       "NQ-HĐTP",
		"nghi quyet hdtp":               "NQ-HĐTP",
		"nghi quyet hoi dong tham phan": "NQ-HĐTP",
		"tt btp":                        "TT-BTP",
		"thong tu btp":                  "TT-BTP",
		"thong tu bo tu phap":           "TT-BTP",
	}
	legalAbbreviationTokenPattern = regexp.MustCompile(`^[A-Za-zĐđÐð\-]+$`)
)

func NormalizeNFC(value string) string {
	return norm.NFC.String(legacyVietnameseRuneReplacer.Replace(value))
}

func FoldAccentsForSearch(value string) string {
	value = accentFoldRuneReplacer.Replace(NormalizeNFC(value))
	decomposed := norm.NFD.String(value)
	var b strings.Builder
	b.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.ToLower(norm.NFC.String(b.String()))
}

func SearchKey(value string) string {
	value = FoldAccentsForSearch(value)
	value = searchSeparatorReplacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func CanonicalLegalAbbreviation(value string) string {
	if canonical, ok := LookupLegalAbbreviation(value); ok {
		return canonical
	}
	return strings.TrimSpace(NormalizeNFC(value))
}

func LookupLegalAbbreviation(value string) (string, bool) {
	key := SearchKey(value)
	canonical, ok := legalAbbreviationAliases[key]
	return canonical, ok
}

func NormalizeLegalDocumentNumber(value string) string {
	value = strings.TrimSpace(documentNumberDashReplacer.Replace(NormalizeNFC(value)))
	if value == "" {
		return ""
	}
	parts := strings.Split(value, "/")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if canonical, ok := LookupLegalAbbreviation(part); ok {
			parts[i] = canonical
			continue
		}
		if legalAbbreviationTokenPattern.MatchString(part) {
			parts[i] = strings.ToUpper(part)
			continue
		}
		parts[i] = part
	}
	return strings.Join(parts, "/")
}

func KnownLegalAbbreviations() []string {
	out := make([]string, 0, len(legalAbbreviationAliases))
	seen := map[string]struct{}{}
	for _, canonical := range legalAbbreviationAliases {
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	sort.Strings(out)
	return out
}
