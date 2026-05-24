package legalmeta

import (
	"strings"

	"github.com/khiemnd777/legal_api/core/language"
)

var documentTypeAliases = map[string]string{
	"law":        "law",
	"luat":       "law",
	"luật":       "law",
	"bo luat":    "law",
	"bộ luật":    "law",
	"code":       "law",
	"resolution": "resolution",
	"nghi quyet": "resolution",
	"nghị quyết": "resolution",
	"decree":     "decree",
	"nghi dinh":  "decree",
	"nghị định":  "decree",
	"circular":   "circular",
	"thong tu":   "circular",
	"thông tư":   "circular",
}

var legalDomainAliases = map[string]string{
	"general legal":        "general_legal",
	"general_legal":        "general_legal",
	"civil":                "civil",
	"dan su":               "civil",
	"dân sự":               "civil",
	"marriage family":      "marriage_family",
	"marriage_family":      "marriage_family",
	"hon nhan va gia dinh": "marriage_family",
	"hôn nhân và gia đình": "marriage_family",
	"hon nhan gia dinh":    "marriage_family",
	"hôn nhân gia đình":    "marriage_family",
	"criminal law":         "criminal_law",
	"criminal_law":         "criminal_law",
	"hinh su":              "criminal_law",
	"hình sự":              "criminal_law",
	"civil status":         "civil_status",
	"civil_status":         "civil_status",
	"ho tich":              "civil_status",
	"hộ tịch":              "civil_status",
}

func NormalizeDocumentType(value string) string {
	return normalizeAlias(value, documentTypeAliases)
}

func NormalizeLegalDomain(value string) string {
	return normalizeAlias(value, legalDomainAliases)
}

func NormalizeEffectiveStatus(value string) string {
	normalized := normalizeText(value)
	switch normalized {
	case "", "active", "archived":
		return normalized
	case "con hieu luc", "co hieu luc":
		return "active"
	case "het hieu luc", "expired":
		return "archived"
	}
	if strings.Contains(normalized, "co hieu luc") {
		return "active"
	}
	if strings.Contains(normalized, "het hieu luc") {
		return "archived"
	}
	return strings.ReplaceAll(normalized, " ", "_")
}

func NormalizeDocumentNumber(value string) string {
	return language.NormalizeLegalDocumentNumber(value)
}

func normalizeAlias(value string, aliases map[string]string) string {
	normalized := normalizeText(value)
	if normalized == "" {
		return ""
	}
	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return strings.ReplaceAll(normalized, " ", "_")
}

func normalizeText(value string) string {
	return language.SearchKey(value)
}
