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

func TestAnalyzeQueryExtractsVietnameseLegalReferences(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	cases := []struct {
		name           string
		query          string
		article        string
		clause         string
		point          string
		documentNumber string
		legalDocKind   string
	}{
		{
			name:    "diacritic article clause point",
			query:   "Điều 56 khoản 3 điểm a về ly hôn",
			article: "56",
			clause:  "3",
			point:   "a",
		},
		{
			name:    "no diacritic article clause point",
			query:   "Dieu 56 khoan 3 diem a ve ly hon",
			article: "56",
			clause:  "3",
			point:   "a",
		},
		{
			name:           "diacritic decree number",
			query:          "NĐ 123/2015/NĐ-CP Điều 6",
			article:        "6",
			documentNumber: "123/2015/NĐ-CP",
			legalDocKind:   "decree",
		},
		{
			name:           "no diacritic decree number",
			query:          "ND 123/2015/ND-CP Dieu 6",
			article:        "6",
			documentNumber: "123/2015/ND-CP",
			legalDocKind:   "decree",
		},
		{
			name:           "diacritic resolution number",
			query:          "NQ 01/2024/NQ-HĐTP về ly hôn",
			documentNumber: "01/2024/NQ-HĐTP",
			legalDocKind:   "resolution",
		},
		{
			name:           "no diacritic resolution number",
			query:          "NQ 01/2024/NQ-HDTP ve ly hon",
			documentNumber: "01/2024/NQ-HDTP",
			legalDocKind:   "resolution",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzeQueryWithIndex(tt.query, index)
			if tt.article != "" && entityString(got, "article_number") != tt.article {
				t.Fatalf("article_number = %q, want %q", entityString(got, "article_number"), tt.article)
			}
			if tt.clause != "" && entityString(got, "clause_number") != tt.clause {
				t.Fatalf("clause_number = %q, want %q", entityString(got, "clause_number"), tt.clause)
			}
			if tt.point != "" && entityString(got, "point_marker") != tt.point {
				t.Fatalf("point_marker = %q, want %q", entityString(got, "point_marker"), tt.point)
			}
			if tt.documentNumber != "" && entityString(got, "document_number") != tt.documentNumber {
				t.Fatalf("document_number = %q, want %q", entityString(got, "document_number"), tt.documentNumber)
			}
			if tt.legalDocKind != "" && entityString(got, "legal_doc_kind") != tt.legalDocKind {
				t.Fatalf("legal_doc_kind = %q, want %q", entityString(got, "legal_doc_kind"), tt.legalDocKind)
			}
		})
	}
}

func TestAnalyzeQueryMatchesLyHonWithAndWithoutDiacritics(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	for _, query := range []string{"ly hon", "ly hôn"} {
		t.Run(query, func(t *testing.T) {
			got := analyzeQueryWithIndex(query, index)
			if !containsPhrase(got.CanonicalQuery, "ly hon") {
				t.Fatalf("canonical query = %q, want ly hon", got.CanonicalQuery)
			}
			if got.LegalDomain != "marriage_family" || got.LegalTopic != "divorce" {
				t.Fatalf("domain/topic = %q/%q, want marriage_family/divorce", got.LegalDomain, got.LegalTopic)
			}
		})
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

func TestBuildRetrievalPlanDoesNotApplyDefaultEffectiveStatusAsFilter(t *testing.T) {
	cfg := defaultRuntimeConfig()
	qu := QueryUnderstandingResult{
		OriginalQuery:   "Thủ tục ly hôn.",
		NormalizedQuery: "thu tuc ly hon",
		CanonicalQuery:  "thu tuc ly hon",
		Filters:         map[string]interface{}{},
	}

	got := BuildRetrievalPlan(qu, SearchOptions{}, cfg)
	if _, ok := got.Filters["effective_status"]; ok {
		t.Fatalf("expected default effective_status to be omitted, got %#v", got.Filters["effective_status"])
	}
	qf := buildQdrantFilter(got.Filters, got.PreferredDocTypes)
	if qf != nil && len(qf.EffectiveStatus) != 0 {
		t.Fatalf("expected qdrant effective_status filter to be empty, got %#v", qf.EffectiveStatus)
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
		DocumentNumber:  "123/2015/ND-CP",
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
	if got.Filters["document_number"] != "123/2015/NĐ-CP" {
		t.Fatalf("document_number = %#v, want 123/2015/NĐ-CP", got.Filters["document_number"])
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
