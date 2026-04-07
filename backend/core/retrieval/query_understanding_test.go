package retrieval

import (
	"encoding/json"
	"testing"

	"github.com/khiemnd777/legal_api/core/answer"
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

func TestAnalyzeQueryWithRepresentativeProfilesRoutesMarriageFamilyAndCivilQueries(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))

	divorce := analyzeQueryWithIndex("Thủ tục ly dị.", index)
	if divorce.CanonicalQuery != "thu tuc ly hon" {
		t.Fatalf("canonical query = %q, want %q", divorce.CanonicalQuery, "thu tuc ly hon")
	}
	if len(divorce.MatchedDocTypes) == 0 || divorce.MatchedDocTypes[0] != "vn_marriage_family_law" {
		t.Fatalf("matched doc types = %v, want vn_marriage_family_law first", divorce.MatchedDocTypes)
	}
	if divorce.LegalDomain != "marriage_family" || divorce.LegalTopic != "divorce" {
		t.Fatalf("domain/topic = %q/%q, want marriage_family/divorce", divorce.LegalDomain, divorce.LegalTopic)
	}
	if divorce.Intent != "legal_procedure_advice" {
		t.Fatalf("intent = %q, want legal_procedure_advice", divorce.Intent)
	}

	civil := analyzeQueryWithIndex("tranh chấp hợp đồng", index)
	if len(civil.MatchedDocTypes) == 0 || civil.MatchedDocTypes[0] != "vn_civil_code" {
		t.Fatalf("matched doc types = %v, want vn_civil_code first", civil.MatchedDocTypes)
	}
	if civil.LegalDomain != "civil" || civil.LegalTopic != "contract" {
		t.Fatalf("domain/topic = %q/%q, want civil/contract", civil.LegalDomain, civil.LegalTopic)
	}
}

func TestAnalyzeQueryWithRepresentativeProfilesSkipsGreetingSignals(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	got := analyzeQueryWithIndex("xin chào", index)
	if len(got.MatchedDocTypes) != 0 {
		t.Fatalf("expected greeting to avoid doc type matches, got %v", got.MatchedDocTypes)
	}
	if containsLegalSignal(index, got.CanonicalQuery) {
		t.Fatalf("expected greeting to avoid legal signals")
	}
}

func TestBuildFollowUpSearchQueryWithRepresentativeProfilesPreservesDivorceContext(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	history := []answer.ConversationMessage{
		{Role: "user", Content: "Thủ tục ly hôn như thế nào?"},
		{Role: "assistant", Content: "Cần hồ sơ và căn cứ cụ thể."},
	}

	query := buildFollowUpSearchQueryWithIndex(index, history, "Cảm ơn, hỏi thêm về ly dị")
	if query == "Cảm ơn, hỏi thêm về ly dị" {
		t.Fatalf("expected follow-up query to include prior history")
	}

	got := analyzeQueryWithIndex(query, index)
	if got.LegalDomain != "marriage_family" || got.LegalTopic != "divorce" {
		t.Fatalf("domain/topic = %q/%q, want marriage_family/divorce", got.LegalDomain, got.LegalTopic)
	}
	if len(got.MatchedDocTypes) == 0 || got.MatchedDocTypes[0] != "vn_marriage_family_law" {
		t.Fatalf("matched doc types = %v, want vn_marriage_family_law first", got.MatchedDocTypes)
	}
}

func TestBuildRetrievalPlanNormalizesExplicitFilters(t *testing.T) {
	cfg := defaultRuntimeConfig()
	qu := QueryUnderstandingResult{
		OriginalQuery:   "Thủ tục ly dị.",
		NormalizedQuery: "thu tuc ly di",
		CanonicalQuery:  "thu tuc ly hon",
		Filters:         map[string]interface{}{},
	}

	got := BuildRetrievalPlan(qu, SearchOptions{
		Domain:          "Hôn nhân và gia đình",
		DocType:         "BỘ LUẬT",
		EffectiveStatus: "có hiệu lực thi hành từ ngày 01 tháng 01 năm 2017",
	}, cfg)

	if got.Filters["legal_domain"] != "marriage_family" {
		t.Fatalf("legal_domain = %#v, want marriage_family", got.Filters["legal_domain"])
	}
	if got.Filters["document_type"] != "law" {
		t.Fatalf("document_type = %#v, want law", got.Filters["document_type"])
	}
	if got.Filters["effective_status"] != "active" {
		t.Fatalf("effective_status = %#v, want active", got.Filters["effective_status"])
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

func representativeProfileDocTypes(t *testing.T) []domain.DocType {
	t.Helper()
	profiles := []struct {
		code    string
		name    string
		profile schema.QueryProfile
	}{
		{
			code: "vn_marriage_family_law",
			name: "Vietnam Marriage & Family Law",
			profile: schema.QueryProfile{
				CanonicalTerms:    []string{"ly hon", "ket hon"},
				QuerySignals:      []string{"ly hon", "ket hon", "thu tuc", "vo chong"},
				PreferredDocTypes: []string{"law", "resolution", "decree"},
				SynonymGroups: []schema.SynonymGroup{
					{Canonical: "ly hon", Aliases: []string{"ly dị", "ly di", "ly hôn"}},
					{Canonical: "ket hon", Aliases: []string{"kết hôn", "dang ky ket hon"}},
				},
				IntentRules: []schema.IntentRule{
					{Intent: "legal_procedure_advice", Terms: []string{"thu tuc", "ho so"}},
				},
				DomainTopicRules: []schema.DomainTopicRule{
					{LegalDomain: "marriage_family", LegalTopic: "divorce", Terms: []string{"ly hon"}},
					{LegalDomain: "marriage_family", LegalTopic: "marriage_registration", Terms: []string{"ket hon"}},
				},
				LegalSignalRules: []string{"ly hon", "ket hon", "vo chong", "dieu"},
				FollowUpMarkers:  []string{"cam on", "hoi them", "them nua"},
				RoutingPriority:  100,
			},
		},
		{
			code: "vn_civil_code",
			name: "Vietnam Civil Code",
			profile: schema.QueryProfile{
				CanonicalTerms:    []string{"hop dong", "giao dich", "tai san"},
				QuerySignals:      []string{"hop dong", "giao dich", "tranh chap", "dan su"},
				PreferredDocTypes: []string{"law", "decree", "resolution"},
				SynonymGroups: []schema.SynonymGroup{
					{Canonical: "hop dong", Aliases: []string{"hợp đồng"}},
					{Canonical: "giao dich", Aliases: []string{"giao dịch"}},
				},
				IntentRules: []schema.IntentRule{
					{Intent: "legal_dispute_resolution", Terms: []string{"tranh chap", "vi pham hop dong"}},
					{Intent: "legal_rights_obligations", Terms: []string{"hop dong", "tai san"}},
				},
				DomainTopicRules: []schema.DomainTopicRule{
					{LegalDomain: "civil", LegalTopic: "contract", Terms: []string{"hop dong", "tranh chap hop dong", "giao dich"}},
				},
				LegalSignalRules: []string{"hop dong", "giao dich", "dan su", "dieu"},
				FollowUpMarkers:  []string{"cam on", "hoi them", "them nua"},
				RoutingPriority:  95,
			},
		},
		{
			code: "legal_normative",
			name: "Legal Normative",
			profile: schema.QueryProfile{
				CanonicalTerms:    []string{"phap luat", "quy dinh"},
				QuerySignals:      []string{"phap luat", "quy dinh", "van ban"},
				PreferredDocTypes: []string{"law", "resolution", "decree"},
				SynonymGroups: []schema.SynonymGroup{
					{Canonical: "phap luat", Aliases: []string{"pháp luật"}},
				},
				IntentRules: []schema.IntentRule{
					{Intent: "legal_basis_lookup", Terms: []string{"quy dinh", "dieu", "khoan"}},
				},
				DomainTopicRules: []schema.DomainTopicRule{
					{LegalDomain: "general_legal", LegalTopic: "legal_basis", Terms: []string{"phap luat", "quy dinh"}},
				},
				LegalSignalRules: []string{"phap luat", "quy dinh", "dieu", "khoan"},
				FollowUpMarkers:  []string{"cam on", "hoi them", "them nua"},
				RoutingPriority:  10,
			},
		},
	}
	out := make([]domain.DocType, 0, len(profiles))
	for _, item := range profiles {
		form := schema.DocTypeForm{
			Version:       2,
			DocType:       schema.DocType{Code: item.code, Name: item.name},
			SegmentRules:  schema.SegmentRules{Strategy: "legal_article", Hierarchy: "article", Normalization: "basic"},
			Metadata:      schema.MetadataSchema{Fields: []schema.MetadataField{{Name: "title", Type: "string"}}},
			MappingRules:  []schema.MappingRule{{Field: "title", Group: 1}},
			ReindexPolicy: schema.ReindexPolicy{OnContentChange: true, OnFormChange: true},
			QueryProfile:  item.profile,
		}
		hash, err := form.Hash()
		if err != nil {
			t.Fatalf("form.Hash(%s): %v", item.code, err)
		}
		out = append(out, domain.DocType{
			Code:     item.code,
			FormHash: hash,
			FormJSON: mustMarshalQueryForm(t, form),
		})
	}
	return out
}
