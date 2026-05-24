package retrieval

import (
	"testing"

	"github.com/khiemnd777/legal_api/domain"
)

func TestRerankCandidatesBoostsExactLegalReferenceMatches(t *testing.T) {
	qu := UnderstandQuery("Điều 56 khoản 3 NĐ 123/2015/NĐ-CP về ly hôn")
	candidates := []RetrievalCandidate{
		{
			Chunk: domain.Chunk{
				ID:   "exact",
				Text: "Điều 56. Quy định về ly hôn. 3. Khoản này áp dụng theo Nghị định 123/2015/NĐ-CP.",
			},
			ChunkID:     "exact",
			VectorScore: 0.40,
			Metadata: map[string]interface{}{
				"document_number": "123/2015/ND-CP",
				"document_type":   "decree",
				"article_number":  "56",
				"clause_number":   "3",
			},
		},
		{
			Chunk: domain.Chunk{
				ID:   "near",
				Text: "Điều 95. Quy định khác trong Nghị định 123/2015/NĐ-CP.",
			},
			ChunkID:     "near",
			VectorScore: 0.40,
			Metadata: map[string]interface{}{
				"document_number": "123/2015/NĐ-CP",
				"document_type":   "decree",
				"article_number":  "95",
				"clause_number":   "1",
			},
		},
	}

	trace := rerankCandidates(candidates, qu, RetrievalPlan{Filters: map[string]interface{}{}}, defaultRuntimeConfig())
	if len(trace) != 2 {
		t.Fatalf("expected 2 reranked chunks, got %d", len(trace))
	}
	if trace[0].ChunkID != "exact" {
		t.Fatalf("top reranked chunk = %q, want exact; trace=%#v", trace[0].ChunkID, trace)
	}
	if trace[0].ArticleScore <= trace[1].ArticleScore {
		t.Fatalf("expected exact legal reference score to exceed near match: exact=%f near=%f", trace[0].ArticleScore, trace[1].ArticleScore)
	}
}
