package legalmeta

import "strings"

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
	case "con hieu luc", "còn hiệu lực", "co hieu luc", "có hiệu lực":
		return "active"
	case "het hieu luc", "hết hiệu lực", "expired":
		return "archived"
	}
	if strings.Contains(normalized, "co hieu luc") || strings.Contains(normalized, "có hiệu lực") {
		return "active"
	}
	if strings.Contains(normalized, "het hieu luc") || strings.Contains(normalized, "hết hiệu lực") {
		return "archived"
	}
	return strings.ReplaceAll(normalized, " ", "_")
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
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return strings.Join(strings.Fields(value), " ")
}
