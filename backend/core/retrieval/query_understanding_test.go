package retrieval

import (
	"encoding/json"
	"testing"

	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
)

func TestAnalyzeQueryWithIndexCanonicalizesLyDiToLyHon(t *testing.T) {
	index := buildQueryUnderstandingIndex([]domain.DocType{{
		Code:     "legal_normative",
		FormHash: "hash-1",
		FormJSON: mustMarshalQueryForm(t, schema.DocTypeForm{
			Version:       1,
			DocType:       schema.DocType{Code: "legal_normative", Name: "Legal Normative"},
			SegmentRules:  schema.SegmentRules{Strategy: "legal_article", Hierarchy: "article", Normalization: "basic"},
			Metadata:      schema.MetadataSchema{Fields: []schema.MetadataField{{Name: "title", Type: "string"}}},
			MappingRules:  []schema.MappingRule{{Field: "title", Group: 1}},
			ReindexPolicy: schema.ReindexPolicy{OnContentChange: true, OnFormChange: true},
			QueryProfile: schema.QueryProfile{
				CanonicalTerms:    []string{"ly hon"},
				QuerySignals:      []string{"ly hon"},
				PreferredDocTypes: []string{"law", "resolution"},
				SynonymGroups:     []schema.SynonymGroup{{Canonical: "ly hon", Aliases: []string{"ly dị", "ly di"}}},
				IntentRules:       []schema.IntentRule{{Intent: "legal_procedure_advice", Terms: []string{"thu tuc"}}},
				DomainTopicRules:  []schema.DomainTopicRule{{LegalDomain: "marriage_family", LegalTopic: "divorce", Terms: []string{"ly hon"}}},
			},
		}),
	}})

	got := analyzeQueryWithIndex("Thủ tục ly dị.", index)
	if got.CanonicalQuery != "thu tuc ly hon" {
		t.Fatalf("canonical query = %q, want %q", got.CanonicalQuery, "thu tuc ly hon")
	}
	if got.LegalDomain != "marriage_family" || got.LegalTopic != "divorce" {
		t.Fatalf("domain/topic = %q/%q, want marriage_family/divorce", got.LegalDomain, got.LegalTopic)
	}
	if got.Intent != "legal_procedure_advice" {
		t.Fatalf("intent = %q, want legal_procedure_advice", got.Intent)
	}
	if len(got.MatchedDocTypes) == 0 || got.MatchedDocTypes[0] != "legal_normative" {
		t.Fatalf("matched doc types = %v, want legal_normative", got.MatchedDocTypes)
	}
}

func mustMarshalQueryForm(t *testing.T, form schema.DocTypeForm) []byte {
	t.Helper()
	raw, err := json.Marshal(form)
	if err != nil {
		t.Fatalf("marshal form: %v", err)
	}
	return raw
}
