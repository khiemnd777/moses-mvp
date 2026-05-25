package retrieval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/khiemnd777/legal_api/infra"
)

func TestSearchWithFallbackRelaxesBroadFiltersBeforeExactReferences(t *testing.T) {
	t.Parallel()

	captured := []map[string]struct{}{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/collections/legal_chunks/points/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload struct {
			Filter *struct {
				Must []struct {
					Key string `json:"key"`
				} `json:"must"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		keys := map[string]struct{}{}
		if payload.Filter != nil {
			for _, condition := range payload.Filter.Must {
				keys[condition.Key] = struct{}{}
			}
		}
		captured = append(captured, keys)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer srv.Close()

	svc := &Service{Qdrant: infra.NewQdrantClient(srv.URL, "legal_chunks")}
	_, stages, err := svc.searchWithFallback(context.Background(), []float64{0.1}, 5, &infra.SearchFilter{
		LegalDomain:     []string{"marriage_family"},
		DocumentType:    []string{"law"},
		EffectiveStatus: []string{"active"},
		DocumentNumber:  []string{"123/2015/ND-CP"},
		ArticleNumber:   []string{"54"},
	})
	if err != nil {
		t.Fatalf("searchWithFallback returned error: %v", err)
	}
	if len(stages) != 4 {
		t.Fatalf("expected 4 fallback stages, got %d", len(stages))
	}
	if len(captured) != 4 {
		t.Fatalf("expected 4 search requests, got %d", len(captured))
	}

	assertHasKey := func(idx int, key string) {
		t.Helper()
		if _, ok := captured[idx][key]; !ok {
			t.Fatalf("request %d missing %s: %#v", idx+1, key, captured[idx])
		}
	}
	assertNoKey := func(idx int, key string) {
		t.Helper()
		if _, ok := captured[idx][key]; ok {
			t.Fatalf("request %d still had %s: %#v", idx+1, key, captured[idx])
		}
	}

	assertHasKey(0, "legal_domain")
	assertHasKey(0, "document_type")
	assertHasKey(0, "effective_status")
	assertNoKey(1, "legal_domain")
	assertHasKey(1, "document_type")
	assertHasKey(1, "effective_status")
	assertNoKey(2, "legal_domain")
	assertNoKey(2, "document_type")
	assertHasKey(2, "effective_status")
	assertNoKey(3, "legal_domain")
	assertNoKey(3, "document_type")
	assertNoKey(3, "effective_status")

	for idx := range captured {
		if _, ok := captured[idx]["document_number"]; !ok {
			t.Fatalf("request %d dropped document_number: %#v", idx+1, captured[idx])
		}
		if _, ok := captured[idx]["article_number"]; !ok {
			t.Fatalf("request %d dropped article_number: %#v", idx+1, captured[idx])
		}
	}
	if stages[3].Reason != "removed_effective_status" {
		t.Fatalf("expected final fallback to remove effective_status, got %q", stages[3].Reason)
	}
}
