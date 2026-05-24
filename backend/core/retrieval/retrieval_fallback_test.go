package retrieval

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/khiemnd777/legal_api/infra"
)

func TestSearchWithFallbackPreservesEffectiveStatusAndExactFilters(t *testing.T) {
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
	if len(stages) != 3 {
		t.Fatalf("expected 3 fallback stages, got %d", len(stages))
	}
	if len(captured) != 3 {
		t.Fatalf("expected 3 search requests, got %d", len(captured))
	}
	for idx, keys := range captured {
		if _, ok := keys["effective_status"]; !ok {
			t.Fatalf("request %d dropped effective_status: %#v", idx+1, keys)
		}
		if _, ok := keys["document_number"]; !ok {
			t.Fatalf("request %d dropped document_number: %#v", idx+1, keys)
		}
		if _, ok := keys["article_number"]; !ok {
			t.Fatalf("request %d dropped article_number: %#v", idx+1, keys)
		}
	}
}
