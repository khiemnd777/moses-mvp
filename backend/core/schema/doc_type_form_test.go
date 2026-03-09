package schema

import "testing"

func TestDocTypeFormHashDeterministic(t *testing.T) {
	form := DocTypeForm{
		Version: 1,
		DocType: DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules: SegmentRules{Strategy: "free_text", Hierarchy: "none", Normalization: "basic"},
		Metadata: MetadataSchema{Fields: []MetadataField{{Name: "title", Type: "string"}, {Name: "date", Type: "date"}}},
		MappingRules: []MappingRule{
			{Field: "title", Regex: "^Title:(.*)$", Group: 1},
			{Field: "date", Regex: "^Date:(.*)$", Group: 1},
		},
		ReindexPolicy: ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}
	first, err := form.Hash()
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	second, err := form.Hash()
	if err != nil {
		t.Fatalf("hash error: %v", err)
	}
	if first != second {
		t.Fatalf("expected deterministic hash")
	}
}

func TestDocTypeFormValidateMappingRulesAligned(t *testing.T) {
	form := DocTypeForm{
		Version: 1,
		DocType: DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules: SegmentRules{
			Strategy:      "free_text",
			Hierarchy:     "none",
			Normalization: "basic",
		},
		Metadata: MetadataSchema{
			Fields: []MetadataField{
				{Name: "doc_category", Type: "string"},
				{Name: "doc_year", Type: "number"},
			},
		},
		MappingRules: []MappingRule{
			{
				Field: "doc_category",
				Regex: "(Luật|Nghị định)",
				Group: 1,
				ValueMap: map[string]string{
					"Luật":     "LAW",
					"Nghị định": "DECREE",
				},
			},
			{
				Field: "doc_year",
				Regex: "(\\d{4})",
				Group: 1,
			},
		},
		ReindexPolicy: ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}
	if err := form.Validate(); err != nil {
		t.Fatalf("expected valid form, got error: %v", err)
	}
}

func TestDocTypeFormValidateRejectsMissingAndUnknownMappingFields(t *testing.T) {
	form := DocTypeForm{
		Version: 1,
		DocType: DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules: SegmentRules{
			Strategy:      "free_text",
			Hierarchy:     "none",
			Normalization: "basic",
		},
		Metadata: MetadataSchema{
			Fields: []MetadataField{
				{Name: "doc_category", Type: "string"},
				{Name: "doc_year", Type: "number"},
			},
		},
		MappingRules: []MappingRule{
			{Field: "doc_category", Regex: "(Luật|Nghị định)", Group: 1},
			{Field: "doc_number", Regex: "Luật số:\\s*([0-9/]+)", Group: 1},
		},
		ReindexPolicy: ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}
	if err := form.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestDocTypeFormAlignMappingRules(t *testing.T) {
	form := DocTypeForm{
		Version: 1,
		DocType: DocType{Code: "legal_normative", Name: "Legal Normative"},
		SegmentRules: SegmentRules{
			Strategy:      "free_text",
			Hierarchy:     "none",
			Normalization: "basic",
		},
		Metadata: MetadataSchema{
			Fields: []MetadataField{
				{Name: "doc_category", Type: "string"},
				{Name: "doc_year", Type: "number"},
			},
		},
		MappingRules: []MappingRule{
			{Field: "doc_category", Regex: "(Luật|Nghị định)", Group: 1},
			{Field: "doc_number", Regex: "Luật số:\\s*([0-9/]+)", Group: 1},
		},
		ReindexPolicy: ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}

	aligned := form.AlignMappingRules()
	if len(aligned.MappingRules) != 2 {
		t.Fatalf("expected 2 mapping rules after alignment, got %d", len(aligned.MappingRules))
	}
	if aligned.MappingRules[0].Field != "doc_category" {
		t.Fatalf("expected first mapping rule for doc_category")
	}
	if aligned.MappingRules[1].Field != "doc_year" {
		t.Fatalf("expected second mapping rule for doc_year")
	}
	if aligned.MappingRules[1].Group != 1 {
		t.Fatalf("expected default group for synthesized mapping rule")
	}
}
