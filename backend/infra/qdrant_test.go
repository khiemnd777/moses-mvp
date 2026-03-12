package infra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

func TestDeleteByFilter_RejectsEmptyFilter(t *testing.T) {
	t.Parallel()

	client := NewQdrantClient("http://example.invalid", "legal_chunks")
	err := client.DeleteByFilter(context.Background(), "legal_chunks", Filter{})
	if err == nil || !strings.Contains(err.Error(), "empty filter") {
		t.Fatalf("expected empty-filter error, got %v", err)
	}
}

func TestDeleteByFilter_RejectsUnknownField(t *testing.T) {
	t.Parallel()

	client := NewQdrantClient("http://example.invalid", "legal_chunks")
	err := client.DeleteByFilter(context.Background(), "legal_chunks", Filter{
		Must: []FieldCondition{{Key: "effective_status", Match: FieldMatch{Value: "active"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "rejected field") {
		t.Fatalf("expected field rejection, got %v", err)
	}
}

func TestDeleteByFilter_SendsFilterPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections/legal_chunks/points/count" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"count":8}}`))
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/collections/legal_chunks/points/delete" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("wait"); got != "true" {
			t.Fatalf("expected wait=true, got %q", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		filter, ok := payload["filter"].(map[string]interface{})
		if !ok {
			t.Fatalf("missing filter in payload: %#v", payload)
		}
		must, ok := filter["must"].([]interface{})
		if !ok || len(must) != 1 {
			t.Fatalf("unexpected must: %#v", filter)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := NewQdrantClient(srv.URL, "legal_chunks")
	err := client.DeleteByFilter(context.Background(), "legal_chunks", Filter{
		Must: []FieldCondition{{
			Key:   "document_version_id",
			Match: FieldMatch{Value: "version-1"},
		}},
	})
	if err != nil {
		t.Fatalf("DeleteByFilter returned error: %v", err)
	}
}

func TestDeleteByFilter_RejectsBroadEstimatedScope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/collections/legal_chunks/points/count" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"count":999999}}`))
			return
		}
		t.Fatalf("delete endpoint should not be called when scope is too broad")
	}))
	defer srv.Close()

	client := NewQdrantClient(srv.URL, "legal_chunks")
	err := client.DeleteByFilter(context.Background(), "legal_chunks", Filter{
		Must: []FieldCondition{{Key: "document_id", Match: FieldMatch{Value: "doc-1"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "broad scope") {
		t.Fatalf("expected broad scope rejection, got %v", err)
	}
}

func TestUpsert_RetriesOnTransient500(t *testing.T) {
	t.Parallel()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/legal_chunks/points" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("wait"); got != "true" {
			t.Fatalf("expected wait=true, got %q", got)
		}
		call := atomic.AddInt32(&calls, 1)
		if call < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := NewQdrantClient(srv.URL, "legal_chunks")
	err := client.Upsert(context.Background(), []PointInput{{ID: "p1", Vector: []float64{0.1}, Payload: map[string]interface{}{"chunk_id": "c1"}}})
	if err != nil {
		t.Fatalf("Upsert returned error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("expected 3 calls with retries, got %d", got)
	}
}

func TestUpsert_DoesNotRetryOn400(t *testing.T) {
	t.Parallel()

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.Error(w, "bad payload", http.StatusBadRequest)
	}))
	defer srv.Close()

	client := NewQdrantClient(srv.URL, "legal_chunks")
	err := client.Upsert(context.Background(), []PointInput{{ID: "p1", Vector: []float64{0.1}, Payload: map[string]interface{}{"chunk_id": "c1"}}})
	if err == nil {
		t.Fatalf("expected upsert error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected no retry for 4xx, got %d calls", got)
	}
}
