package service

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type fakeStore struct {
	getChunksByIDsFn                   func(context.Context, []string) ([]domain.Chunk, error)
	getDocumentVersionFn               func(context.Context, string) (domain.DocumentVersion, error)
	getDocumentFn                      func(context.Context, string) (domain.Document, error)
	listDocumentVersionIDsByDocumentFn func(context.Context, string) ([]string, error)
	enqueueIngestJobFn                 func(context.Context, string) (domain.IngestJob, bool, error)
	listDocumentVersionIDsForReindexFn func(context.Context, infra.ReindexScopeQuery) ([]string, error)
}

func (f *fakeStore) GetChunksByIDs(ctx context.Context, ids []string) ([]domain.Chunk, error) {
	if f.getChunksByIDsFn != nil {
		return f.getChunksByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (f *fakeStore) GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error) {
	if f.getDocumentVersionFn != nil {
		return f.getDocumentVersionFn(ctx, id)
	}
	return domain.DocumentVersion{}, sql.ErrNoRows
}

func (f *fakeStore) GetDocument(ctx context.Context, id string) (domain.Document, error) {
	if f.getDocumentFn != nil {
		return f.getDocumentFn(ctx, id)
	}
	return domain.Document{}, sql.ErrNoRows
}

func (f *fakeStore) ListDocumentVersionIDsByDocument(ctx context.Context, documentID string) ([]string, error) {
	if f.listDocumentVersionIDsByDocumentFn != nil {
		return f.listDocumentVersionIDsByDocumentFn(ctx, documentID)
	}
	return nil, nil
}

func (f *fakeStore) EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error) {
	if f.enqueueIngestJobFn != nil {
		return f.enqueueIngestJobFn(ctx, documentVersionID)
	}
	return domain.IngestJob{}, false, nil
}

func (f *fakeStore) ListDocumentVersionIDsForReindex(ctx context.Context, scope infra.ReindexScopeQuery) ([]string, error) {
	if f.listDocumentVersionIDsForReindexFn != nil {
		return f.listDocumentVersionIDsForReindexFn(ctx, scope)
	}
	return nil, nil
}

type fakeQdrant struct {
	getCollectionInfoFn    func(context.Context) (infra.CollectionInfo, error)
	listCollectionsFn      func(context.Context) ([]infra.CollectionListItem, error)
	getCollectionDetailsFn func(context.Context, string) (infra.CollectionDetails, error)
	searchFn               func(context.Context, string, []float64, int, *infra.SearchFilter) ([]infra.SearchResult, error)
	countPointsFn          func(context.Context, string, *infra.Filter) (int64, bool, error)
	deleteByFilterFn       func(context.Context, string, infra.Filter) error
}

func (f *fakeQdrant) GetCollectionInfo(ctx context.Context) (infra.CollectionInfo, error) {
	if f.getCollectionInfoFn != nil {
		return f.getCollectionInfoFn(ctx)
	}
	return infra.CollectionInfo{VectorSize: 1536, Distance: "Cosine"}, nil
}

func (f *fakeQdrant) ListCollections(ctx context.Context) ([]infra.CollectionListItem, error) {
	if f.listCollectionsFn != nil {
		return f.listCollectionsFn(ctx)
	}
	return []infra.CollectionListItem{{Name: "legal_chunks"}}, nil
}

func (f *fakeQdrant) GetCollectionDetails(ctx context.Context, collection string) (infra.CollectionDetails, error) {
	if f.getCollectionDetailsFn != nil {
		return f.getCollectionDetailsFn(ctx, collection)
	}
	return infra.CollectionDetails{Name: collection, VectorSize: 1536, Distance: "Cosine"}, nil
}

func (f *fakeQdrant) SearchInCollection(ctx context.Context, collection string, vector []float64, limit int, filter *infra.SearchFilter) ([]infra.SearchResult, error) {
	if f.searchFn != nil {
		return f.searchFn(ctx, collection, vector, limit, filter)
	}
	return nil, nil
}

func (f *fakeQdrant) CountPoints(ctx context.Context, collection string, filter *infra.Filter) (int64, bool, error) {
	if f.countPointsFn != nil {
		return f.countPointsFn(ctx, collection, filter)
	}
	return 0, true, nil
}

func (f *fakeQdrant) DeleteByFilter(ctx context.Context, collection string, filter infra.Filter) error {
	if f.deleteByFilterFn != nil {
		return f.deleteByFilterFn(ctx, collection, filter)
	}
	return nil
}

type fakeEmbedder struct {
	embedFn func(context.Context, []string) ([][]float64, error)
}

func (f *fakeEmbedder) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	if f.embedFn != nil {
		return f.embedFn(ctx, inputs)
	}
	return [][]float64{{0.1, 0.2}}, nil
}

func TestSearchDebug_ValidQueryBoundedTopKFiltersAndChunkResolution(t *testing.T) {
	store := &fakeStore{
		getChunksByIDsFn: func(ctx context.Context, ids []string) ([]domain.Chunk, error) {
			if !reflect.DeepEqual(ids, []string{"chunk-1", "chunk-missing"}) {
				t.Fatalf("unexpected chunk IDs: %#v", ids)
			}
			return []domain.Chunk{{ID: "chunk-1", DocumentVersionID: "ver-1", Index: 2, Text: "chunk text"}}, nil
		},
	}
	qdrant := &fakeQdrant{
		searchFn: func(ctx context.Context, collection string, vector []float64, limit int, filter *infra.SearchFilter) ([]infra.SearchResult, error) {
			if collection != "legal_chunks" {
				t.Fatalf("unexpected collection: %s", collection)
			}
			if limit != 5 {
				t.Fatalf("expected top_k=5, got %d", limit)
			}
			if filter == nil || !reflect.DeepEqual(filter.DocumentType, []string{"law"}) {
				t.Fatalf("unexpected filter: %#v", filter)
			}
			return []infra.SearchResult{
				{ID: "p1", ChunkID: "chunk-1", Score: 0.9, Payload: map[string]interface{}{"chunk_id": "chunk-1"}},
				{ID: "p2", ChunkID: "chunk-missing", Score: 0.8, Payload: map[string]interface{}{"chunk_id": "chunk-missing"}},
			}, nil
		},
	}
	embed := &fakeEmbedder{}

	svc := NewQdrantControlPlaneServiceWithDeps(store, qdrant, embed, "legal_chunks", 1536, nil, func(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error) {
		return infra.VectorConsistencyReport{}, nil
	})

	out, err := svc.SearchDebug(context.Background(), SearchDebugInput{
		QueryText:           "tax law",
		TopK:                5,
		Collection:          "legal_chunks",
		IncludePayload:      true,
		IncludeChunkPreview: true,
		Filters:             SearchDebugFilter{DocumentType: []string{"law"}},
	})
	if err != nil {
		t.Fatalf("search debug failed: %v", err)
	}
	if out.TopK != 5 || out.Collection != "legal_chunks" {
		t.Fatalf("unexpected response envelope: %#v", out)
	}
	if len(out.Hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(out.Hits))
	}
	if out.Hits[0].Chunk == nil || out.Hits[0].Chunk.ID != "chunk-1" {
		t.Fatalf("expected resolved chunk for first hit: %#v", out.Hits[0])
	}
	if out.Hits[1].Chunk != nil {
		t.Fatalf("expected unresolved chunk to stay nil: %#v", out.Hits[1])
	}
}

func TestSearchDebug_InvalidTopKRejected(t *testing.T) {
	svc := NewQdrantControlPlaneServiceWithDeps(&fakeStore{}, &fakeQdrant{}, &fakeEmbedder{}, "legal_chunks", 1536, nil, nil)
	_, err := svc.SearchDebug(context.Background(), SearchDebugInput{QueryText: "x", TopK: 51})
	if !errors.Is(err, ErrInvalidTopK) {
		t.Fatalf("expected ErrInvalidTopK, got %v", err)
	}
}

func TestVectorHealth_PassesBoundedOptionsAndSupportsCancellation(t *testing.T) {
	var captured struct {
		mode            infra.VectorScanMode
		chunkBatchSize  int
		vectorBatchSize int
		maxChunks       int
		maxVectors      int
		maxDuration     time.Duration
	}
	svc := NewQdrantControlPlaneServiceWithDeps(&fakeStore{}, &fakeQdrant{}, &fakeEmbedder{}, "legal_chunks", 1536, nil, func(ctx context.Context, mode infra.VectorScanMode, chunkBatchSize, vectorBatchSize, maxChunks, maxVectors int, maxDuration time.Duration) (infra.VectorConsistencyReport, error) {
		captured.mode = mode
		captured.chunkBatchSize = chunkBatchSize
		captured.vectorBatchSize = vectorBatchSize
		captured.maxChunks = maxChunks
		captured.maxVectors = maxVectors
		captured.maxDuration = maxDuration
		select {
		case <-ctx.Done():
			return infra.VectorConsistencyReport{}, ctx.Err()
		default:
			return infra.VectorConsistencyReport{Mode: string(mode), Bounded: true, ScannedChunkCount: 20, ScannedVectorCount: 20}, nil
		}
	})

	report, err := svc.VectorHealth(context.Background(), infra.VectorScanQuick, 128, 256, 50, 60, 2*time.Second)
	if err != nil {
		t.Fatalf("vector health quick failed: %v", err)
	}
	if report.Mode != "quick" || !report.Bounded {
		t.Fatalf("unexpected quick report: %#v", report)
	}
	if captured.mode != infra.VectorScanQuick || captured.maxVectors != 60 || captured.maxDuration != 2*time.Second {
		t.Fatalf("unexpected forwarded options: %#v", captured)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = svc.VectorHealth(ctx, infra.VectorScanFull, 128, 128, 0, 0, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestDeleteByFilter_DryRunConfirmAndGuardrails(t *testing.T) {
	deleteCalled := 0
	svc := NewQdrantControlPlaneServiceWithDeps(&fakeStore{}, &fakeQdrant{
		countPointsFn: func(ctx context.Context, collection string, filter *infra.Filter) (int64, bool, error) {
			return 7, true, nil
		},
		deleteByFilterFn: func(ctx context.Context, collection string, filter infra.Filter) error {
			deleteCalled++
			return nil
		},
	}, &fakeEmbedder{}, "legal_chunks", 1536, nil, nil)

	in := DeleteByFilterInput{
		Collection: "legal_chunks",
		DryRun:     true,
		Filter: infra.Filter{Must: []infra.FieldCondition{{
			Key: "document_id", Match: infra.FieldMatch{Value: "doc-1"},
		}}},
	}
	out, err := svc.DeleteByFilter(context.Background(), in)
	if err != nil {
		t.Fatalf("dry_run delete failed: %v", err)
	}
	if deleteCalled != 0 {
		t.Fatalf("dry_run must not execute delete")
	}
	if !out.ScopeEstimated || out.EstimatedScope == nil || *out.EstimatedScope != 7 {
		t.Fatalf("unexpected dry_run result: %#v", out)
	}

	in.DryRun = false
	in.Confirm = true
	_, err = svc.DeleteByFilter(context.Background(), in)
	if err != nil {
		t.Fatalf("confirm delete failed: %v", err)
	}
	if deleteCalled != 1 {
		t.Fatalf("expected delete called once, got %d", deleteCalled)
	}

	_, err = svc.DeleteByFilter(context.Background(), DeleteByFilterInput{Collection: "legal_chunks", DryRun: true, Filter: infra.Filter{}})
	if err == nil || !strings.Contains(err.Error(), "empty filter") {
		t.Fatalf("expected empty filter rejection, got %v", err)
	}

	_, err = svc.DeleteByFilter(context.Background(), DeleteByFilterInput{
		Collection: "legal_chunks",
		DryRun:     true,
		Filter: infra.Filter{Must: []infra.FieldCondition{{
			Key: "illegal_field", Match: infra.FieldMatch{Value: "x"},
		}}},
	})
	if err == nil || !strings.Contains(err.Error(), "rejected field") {
		t.Fatalf("expected unapproved field rejection, got %v", err)
	}
}

func TestReindexDocument_ValidAndInvalidTargets(t *testing.T) {
	enqueued := []string{}
	svc := NewQdrantControlPlaneServiceWithDeps(&fakeStore{
		getDocumentVersionFn: func(ctx context.Context, id string) (domain.DocumentVersion, error) {
			if id == "ver-404" {
				return domain.DocumentVersion{}, sql.ErrNoRows
			}
			return domain.DocumentVersion{ID: id}, nil
		},
		enqueueIngestJobFn: func(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error) {
			enqueued = append(enqueued, documentVersionID)
			return domain.IngestJob{ID: "job-" + documentVersionID, DocumentVersionID: documentVersionID, Status: "queued"}, true, nil
		},
	}, &fakeQdrant{}, &fakeEmbedder{}, "legal_chunks", 1536, nil, nil)

	jobs, created, err := svc.EnqueueReindexDocument(context.Background(), ReindexDocumentInput{DocumentVersionID: "ver-1"})
	if err != nil {
		t.Fatalf("enqueue reindex document failed: %v", err)
	}
	if len(jobs) != 1 || len(created) != 1 || jobs[0].Status != "queued" || !created[0] {
		t.Fatalf("unexpected enqueue response: jobs=%#v created=%#v", jobs, created)
	}
	if !reflect.DeepEqual(enqueued, []string{"ver-1"}) {
		t.Fatalf("unexpected enqueued versions: %#v", enqueued)
	}

	_, _, err = svc.EnqueueReindexDocument(context.Background(), ReindexDocumentInput{})
	if !errors.Is(err, ErrInvalidReindexScope) {
		t.Fatalf("expected invalid scope, got %v", err)
	}

	_, _, err = svc.EnqueueReindexDocument(context.Background(), ReindexDocumentInput{DocumentVersionID: "ver-404"})
	if !errors.Is(err, ErrInvalidReindexScope) {
		t.Fatalf("expected invalid scope for missing version, got %v", err)
	}
}

func TestReindexAll_GuardrailsAndScopedExecution(t *testing.T) {
	enqueued := []string{}
	capturedScope := infra.ReindexScopeQuery{}
	svc := NewQdrantControlPlaneServiceWithDeps(&fakeStore{
		listDocumentVersionIDsForReindexFn: func(ctx context.Context, scope infra.ReindexScopeQuery) ([]string, error) {
			capturedScope = scope
			return []string{"ver-1", "ver-2"}, nil
		},
		enqueueIngestJobFn: func(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error) {
			enqueued = append(enqueued, documentVersionID)
			return domain.IngestJob{ID: "job-" + documentVersionID, DocumentVersionID: documentVersionID, Status: "queued"}, true, nil
		},
	}, &fakeQdrant{}, &fakeEmbedder{}, "legal_chunks", 1536, nil, nil)

	_, _, err := svc.EnqueueReindexAll(context.Background(), ReindexAllInput{Confirm: false, Reason: "x"})
	if !errors.Is(err, ErrInvalidReindexScope) {
		t.Fatalf("expected confirm guardrail, got %v", err)
	}

	_, _, err = svc.EnqueueReindexAll(context.Background(), ReindexAllInput{Confirm: true, Reason: ""})
	if !errors.Is(err, ErrInvalidReindexScope) {
		t.Fatalf("expected reason guardrail, got %v", err)
	}

	_, _, err = svc.EnqueueReindexAll(context.Background(), ReindexAllInput{Confirm: true, Reason: "x", Collection: "wrong"})
	if !errors.Is(err, ErrInvalidReindexScope) {
		t.Fatalf("expected collection guardrail, got %v", err)
	}

	jobs, created, err := svc.EnqueueReindexAll(context.Background(), ReindexAllInput{
		Confirm:     true,
		Reason:      "ops",
		DocTypeCode: "LAW",
		Collection:  "legal_chunks",
		Status:      "queued",
		Limit:       99999,
	})
	if err != nil {
		t.Fatalf("scoped reindex_all failed: %v", err)
	}
	if len(jobs) != 2 || len(created) != 2 || !created[0] || !created[1] {
		t.Fatalf("unexpected scoped reindex response: jobs=%#v created=%#v", jobs, created)
	}
	if !reflect.DeepEqual(enqueued, []string{"ver-1", "ver-2"}) {
		t.Fatalf("unexpected enqueued IDs: %#v", enqueued)
	}
	if capturedScope.DocTypeCode != "LAW" || capturedScope.Status != "queued" || capturedScope.Limit != MaxReindexAllLimit {
		t.Fatalf("unexpected scoped query: %#v", capturedScope)
	}
}
