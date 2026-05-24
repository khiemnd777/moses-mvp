package retrieval

import (
	"testing"

	"github.com/khiemnd777/legal_api/domain"
)

func TestVietnameseEvalSuiteQueryUnderstanding(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	cases := []struct {
		name             string
		query            string
		wantCanonical    string
		wantDocType      string
		wantDomain       string
		wantTopic        string
		wantIntent       string
		wantArticle      string
		wantClause       string
		wantPoint        string
		wantDocument     string
		wantLegalDocKind string
		wantLegalSignal  bool
	}{
		{
			name:            "diacritic divorce synonym",
			query:           "Thủ tục ly dị theo Điều 56 khoản 3.",
			wantCanonical:   "thu tuc ly hon theo dieu 56 khoan 3",
			wantDocType:     "vn_marriage_family_law",
			wantDomain:      "marriage_family",
			wantTopic:       "divorce",
			wantIntent:      "legal_procedure_advice",
			wantArticle:     "56",
			wantClause:      "3",
			wantLegalSignal: true,
		},
		{
			name:             "no diacritic decree reference",
			query:            "Dieu 6 ND 123/2015/ND-CP ve giay khai sinh",
			wantCanonical:    "dieu 6 nd 123 2015 nd cp ve giay khai sinh",
			wantArticle:      "6",
			wantDocument:     "123/2015/ND-CP",
			wantLegalDocKind: "decree",
			wantLegalSignal:  true,
		},
		{
			name:             "resolution reference with vietnamese D",
			query:            "NQ 01/2024/NQ-HĐTP về án phí ly hôn",
			wantCanonical:    "nq 01 2024 nq hdtp ve an phi ly hon",
			wantDocType:      "vn_marriage_family_law",
			wantDomain:       "marriage_family",
			wantTopic:        "divorce",
			wantDocument:     "01/2024/NQ-HĐTP",
			wantLegalDocKind: "resolution",
			wantLegalSignal:  true,
		},
		{
			name:            "civil contract dispute",
			query:           "tranh chấp hợp đồng dân sự",
			wantCanonical:   "tranh chap hop dong dan su",
			wantDocType:     "vn_civil_code",
			wantDomain:      "civil",
			wantTopic:       "contract",
			wantIntent:      "legal_dispute_resolution",
			wantLegalSignal: true,
		},
		{
			name:            "non legal greeting",
			query:           "xin chào",
			wantCanonical:   "xin chao",
			wantLegalSignal: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzeQueryWithIndex(tt.query, index)
			if got.CanonicalQuery != tt.wantCanonical {
				t.Fatalf("CanonicalQuery = %q, want %q", got.CanonicalQuery, tt.wantCanonical)
			}
			if tt.wantDocType != "" {
				if len(got.MatchedDocTypes) == 0 || got.MatchedDocTypes[0] != tt.wantDocType {
					t.Fatalf("MatchedDocTypes = %v, want %q first", got.MatchedDocTypes, tt.wantDocType)
				}
			} else if len(got.MatchedDocTypes) != 0 {
				t.Fatalf("MatchedDocTypes = %v, want none", got.MatchedDocTypes)
			}
			if got.LegalDomain != tt.wantDomain {
				t.Fatalf("LegalDomain = %q, want %q", got.LegalDomain, tt.wantDomain)
			}
			if got.LegalTopic != tt.wantTopic {
				t.Fatalf("LegalTopic = %q, want %q", got.LegalTopic, tt.wantTopic)
			}
			if got.Intent != tt.wantIntent {
				t.Fatalf("Intent = %q, want %q", got.Intent, tt.wantIntent)
			}
			assertEntity(t, got, "article_number", tt.wantArticle)
			assertEntity(t, got, "clause_number", tt.wantClause)
			assertEntity(t, got, "point_marker", tt.wantPoint)
			assertEntity(t, got, "document_number", tt.wantDocument)
			assertEntity(t, got, "legal_doc_kind", tt.wantLegalDocKind)
			if gotSignal := containsLegalSignal(index, got.CanonicalQuery); gotSignal != tt.wantLegalSignal {
				t.Fatalf("containsLegalSignal = %v, want %v", gotSignal, tt.wantLegalSignal)
			}
		})
	}
}

func TestVietnameseEvalSuiteRetrievalPlanNormalizesFilters(t *testing.T) {
	index := buildQueryUnderstandingIndex(representativeProfileDocTypes(t))
	analysis := analyzeQueryWithIndex("Thủ tục ly dị", index)
	plan := BuildRetrievalPlan(analysis, SearchOptions{
		Domain:          "hon nhan gia dinh",
		DocType:         "nghi dinh",
		EffectiveStatus: "có hiệu lực thi hành từ ngày 01 tháng 01 năm 2017",
		DocumentNumber:  "123/2015/ND-CP",
		ArticleNumber:   "6",
	}, defaultRuntimeConfig())

	wantFilters := map[string]string{
		"legal_domain":     "marriage_family",
		"document_type":    "decree",
		"effective_status": "active",
		"document_number":  "123/2015/NĐ-CP",
		"article_number":   "6",
	}
	for key, want := range wantFilters {
		if got := pickString(plan.Filters, key); got != want {
			t.Fatalf("plan.Filters[%q] = %q, want %q", key, got, want)
		}
	}
	if len(plan.PreferredDocTypes) != 1 || plan.PreferredDocTypes[0] != "decree" {
		t.Fatalf("PreferredDocTypes = %v, want [decree]", plan.PreferredDocTypes)
	}
}

func TestVietnameseEvalSuiteReranksExactLegalReferences(t *testing.T) {
	qu := UnderstandQuery("Điều 56 khoản 3 NĐ 123/2015/NĐ-CP về ly hôn")
	candidates := []RetrievalCandidate{
		{
			Chunk: domain.Chunk{
				ID:   "wrong-article",
				Text: "Điều 95. Quy định khác trong Nghị định 123/2015/NĐ-CP.",
			},
			ChunkID:     "wrong-article",
			VectorScore: 0.50,
			Metadata: map[string]interface{}{
				"document_number": "123/2015/NĐ-CP",
				"document_type":   "decree",
				"article_number":  "95",
				"clause_number":   "1",
			},
		},
		{
			Chunk: domain.Chunk{
				ID:   "exact-no-diacritic-metadata",
				Text: "Điều 56. Ly hôn. 3. Khoản này áp dụng theo Nghị định 123/2015/NĐ-CP.",
			},
			ChunkID:     "exact-no-diacritic-metadata",
			VectorScore: 0.45,
			Metadata: map[string]interface{}{
				"document_number": "123/2015/ND-CP",
				"document_type":   "Nghị định",
				"article_number":  "56",
				"clause_number":   "3",
			},
		},
	}

	trace := rerankCandidates(candidates, qu, RetrievalPlan{Filters: map[string]interface{}{}}, defaultRuntimeConfig())
	if len(trace) != 2 {
		t.Fatalf("expected 2 reranked chunks, got %d", len(trace))
	}
	if trace[0].ChunkID != "exact-no-diacritic-metadata" {
		t.Fatalf("top reranked chunk = %q, want exact-no-diacritic-metadata; trace=%#v", trace[0].ChunkID, trace)
	}
	if trace[0].ArticleScore <= trace[1].ArticleScore {
		t.Fatalf("exact ArticleScore = %f, want above wrong article score %f", trace[0].ArticleScore, trace[1].ArticleScore)
	}
}

func assertEntity(t *testing.T, got QueryUnderstandingResult, key, want string) {
	t.Helper()
	if want == "" {
		if gotValue := entityString(got, key); gotValue != "" {
			t.Fatalf("entity %s = %q, want empty", key, gotValue)
		}
		return
	}
	if gotValue := entityString(got, key); gotValue != want {
		t.Fatalf("entity %s = %q, want %q", key, gotValue, want)
	}
}
