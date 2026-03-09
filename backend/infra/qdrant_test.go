package infra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetPayloadByPointID_UsesWithVectorField(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/collections/legal_chunks/points" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := payload["with_vector"]; !ok {
			t.Fatalf("missing with_vector field in request payload: %#v", payload)
		}
		if _, ok := payload["with_vectors"]; ok {
			t.Fatalf("unexpected with_vectors field in request payload: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"id":"p1","payload":{"content_hash":"abc"}}]}`))
	}))
	defer srv.Close()

	client := NewQdrantClient(srv.URL, "legal_chunks")
	got, found, err := client.GetPayloadByPointID(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetPayloadByPointID returned error: %v", err)
	}
	if !found {
		t.Fatalf("expected point to be found")
	}
	if got["content_hash"] != "abc" {
		t.Fatalf("unexpected payload: %#v", got)
	}
}

