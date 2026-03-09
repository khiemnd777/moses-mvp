package ingest

import (
	"reflect"
	"testing"

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
				"Luật":     "LAW",
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

