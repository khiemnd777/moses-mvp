package ingest

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/khiemnd777/legal_api/core/schema"
)

func TestExtractMetadataAppliesValueMap(t *testing.T) {
	text := "Loại văn bản: Luật\nNăm: 2024"
	rules := []schema.MappingRule{
		{
			Field: "doc_category",
			Regex: "Loại văn bản:\\s*(Luật|Nghị định)",
			Group: 1,
			ValueMap: map[string]string{
				"Luật":      "LAW",
				"Nghị định": "DECREE",
			},
		},
		{
			Field: "doc_year",
			Regex: "Năm:\\s*(\\d{4})",
			Group: 1,
		},
	}

	got := extractMetadata(text, rules)
	want := map[string]interface{}{
		"doc_category": "LAW",
		"doc_year":     "2024",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected metadata, got=%v want=%v", got, want)
	}
}

func TestLegalStructureParserParsesHierarchy(t *testing.T) {
	text := `
Điều 81 Quyền, nghĩa vụ
1. Cha mẹ có nghĩa vụ.
a) Bảo vệ quyền lợi.
b) Chăm sóc con.
2. Nhà nước bảo hộ.
`

	doc := legalStructureParser{}.Parse(text)
	if len(doc.Articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(doc.Articles))
	}
	article := doc.Articles[0]
	if article.Number != "81" {
		t.Fatalf("unexpected article number: %q", article.Number)
	}
	if len(article.Clauses) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(article.Clauses))
	}
	if len(article.Clauses[0].Points) != 2 {
		t.Fatalf("expected 2 points in clause 1, got %d", len(article.Clauses[0].Points))
	}
}

func TestLegalChunkGeneratorDeterministicAndAnnotated(t *testing.T) {
	text := `
Điều 81 Quyền, nghĩa vụ
1. Cha mẹ có nghĩa vụ bảo vệ con chưa thành niên.
a) Tôn trọng ý kiến của con.
b) Bảo vệ lợi ích hợp pháp của con.
2. Nhà nước bảo hộ quyền trẻ em.
`
	base := map[string]interface{}{"document_type": "law"}
	generator := newLegalChunkGenerator(schema.SegmentRules{
		Strategy:      "legal_article",
		Hierarchy:     "article>clause>point",
		Normalization: "basic",
	})

	first, statsA, err := generator.Generate("doc-1", "54", text, base)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	second, statsB, err := generator.Generate("doc-1", "54", text, base)
	if err != nil {
		t.Fatalf("Generate() second error = %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected chunks")
	}
	if !reflect.DeepEqual(statsA, statsB) {
		t.Fatalf("stats not deterministic: %+v vs %+v", statsA, statsB)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("generated chunks are not deterministic")
	}
	if first[0].Index != 0 {
		t.Fatalf("expected first chunk index 0, got %d", first[0].Index)
	}
	if !strings.Contains(first[0].Text, "Điều 81") || !strings.Contains(first[0].Text, "Khoản 1") {
		t.Fatalf("chunk text missing article/clause context: %q", first[0].Text)
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(first[0].Metadata, &meta); err != nil {
		t.Fatalf("metadata unmarshal error = %v", err)
	}
	structural, ok := meta["structural"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected structural metadata map, got %+v", meta)
	}
	if structural["article"] != "81" || structural["clause"] != "1" {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}

func TestLegalStructureParserRespectsConfiguredHierarchy(t *testing.T) {
	parser := newLegalStructureParser(schema.SegmentRules{
		Strategy:      "legal_article",
		Hierarchy:     "article",
		Normalization: "basic",
	})
	text := `
Điều 5. Quyền
1. Cha mẹ có nghĩa vụ.
2. Nhà nước bảo hộ.
`
	doc := parser.Parse(text)
	if len(doc.Articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(doc.Articles))
	}
	if len(doc.Articles[0].Clauses) != 0 {
		t.Fatalf("expected no clauses when hierarchy excludes clause, got %d", len(doc.Articles[0].Clauses))
	}
}

func TestLegalStructureParserPointRegexDoesNotOvermatch(t *testing.T) {
	parser := newLegalStructureParser(schema.SegmentRules{
		Strategy:      "legal_article",
		Hierarchy:     "article>clause>point",
		Normalization: "basic",
	})
	text := `
Điều 10. Quyền trẻ em
1. Cha mẹ có nghĩa vụ bảo vệ con.
`
	doc := parser.Parse(text)
	if len(doc.Articles) != 1 || len(doc.Articles[0].Clauses) != 1 {
		t.Fatalf("unexpected parse result: %+v", doc)
	}
	if got := len(doc.Articles[0].Clauses[0].Points); got != 0 {
		t.Fatalf("expected 0 points, got %d", got)
	}
}

func TestTokenSafeSplitterSplitsOversizedText(t *testing.T) {
	parts, err := tokenSafeSplitter{
		maxTokens:    40,
		targetTokens: 20,
	}.Split(strings.Repeat("nghia vu cua cong dan; ", 50), "")
	if err != nil {
		t.Fatalf("Split() error = %v", err)
	}
	if len(parts) < 2 {
		t.Fatalf("expected multiple parts, got %d", len(parts))
	}
	for _, part := range parts {
		if tokens := estimateTokenCount(part.Text); tokens > 40 {
			t.Fatalf("part exceeds limit: %d", tokens)
		}
	}
}

func TestVectorPointID_IsDeterministicUUID(t *testing.T) {
	t.Parallel()

	versionID := "eadcc28b-4f5e-4ed1-87d4-7e9f3309ecda"
	idA := VectorPointID(versionID, 0)
	idB := VectorPointID(versionID, 0)
	idC := VectorPointID(versionID, 1)

	if idA != idB {
		t.Fatalf("expected deterministic id, got %q and %q", idA, idB)
	}
	if idA == idC {
		t.Fatalf("expected different ids for different chunk index, got %q", idA)
	}
	if _, err := uuid.Parse(idA); err != nil {
		t.Fatalf("expected valid UUID, got %q err=%v", idA, err)
	}
}
