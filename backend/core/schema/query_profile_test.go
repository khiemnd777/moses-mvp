package schema

import "testing"

func TestQueryProfileValidateAcceptsStructuredSemanticProfile(t *testing.T) {
	profile := QueryProfile{
		CanonicalTerms: []string{"ly hon", "ket hon"},
		SynonymGroups: []SynonymGroup{
			{Canonical: "ly hon", Aliases: []string{"ly di", "ly dị"}},
			{Canonical: "ket hon", Aliases: []string{"dang ky ket hon"}},
		},
		QuerySignals: []string{"ly hon", "ket hon", "thu tuc"},
		IntentRules: []IntentRule{
			{Intent: "legal_procedure_advice", Terms: []string{"thu tuc", "ho so"}},
		},
		DomainTopicRules: []DomainTopicRule{
			{LegalDomain: "marriage_family", LegalTopic: "divorce", Terms: []string{"ly hon"}},
		},
		LegalSignalRules:  []string{"ly hon", "ket hon", "dieu"},
		FollowUpMarkers:   []string{"hoi them"},
		PreferredDocTypes: []string{"law", "resolution"},
		RoutingPriority:   100,
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("profile.Validate() error = %v", err)
	}
}

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
