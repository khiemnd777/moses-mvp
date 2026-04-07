package schema

import "testing"

func TestDocTypeFormValidateRejectsDuplicateQueryProfileAlias(t *testing.T) {
	form := DocTypeForm{
		Version:      1,
		DocType:      DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules: SegmentRules{Strategy: "legal_article", Hierarchy: "article", Normalization: "basic"},
		Metadata:     MetadataSchema{Fields: []MetadataField{{Name: "title", Type: "string"}}},
		MappingRules: []MappingRule{{Field: "title", Group: 1}},
		ReindexPolicy: ReindexPolicy{
			OnContentChange: true,
			OnFormChange:    true,
		},
		QueryProfile: QueryProfile{
			SynonymGroups: []SynonymGroup{
				{Canonical: "ly hon", Aliases: []string{"ly di"}},
				{Canonical: "ly than", Aliases: []string{"ly di"}},
			},
		},
	}

	if err := form.Validate(); err == nil {
		t.Fatalf("expected duplicate alias validation error")
	}
}
