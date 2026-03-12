package ingest

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/khiemnd777/legal_api/core/schema"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type fakeIngestStore struct {
	countByVersion      int
	replaceCalled       bool
	replaceErr          error
	deleteChunksCalled  bool
	touchedJobs         int
	replacedChunkCount  int
	deletedChunkVersion string
}

func (f *fakeIngestStore) TouchJob(ctx context.Context, id string) error {
	f.touchedJobs++
	return nil
}

func (f *fakeIngestStore) CountChunksByVersion(ctx context.Context, documentVersionID string) (int, error) {
	return f.countByVersion, nil
}

func (f *fakeIngestStore) ReplaceChunks(ctx context.Context, documentVersionID string, chunks []domain.Chunk) ([]domain.Chunk, error) {
	f.replaceCalled = true
	f.replacedChunkCount = len(chunks)
	if f.replaceErr != nil {
		return nil, f.replaceErr
	}
	return chunks, nil
}

func (f *fakeIngestStore) DeleteChunksByVersion(ctx context.Context, documentVersionID string) error {
	f.deleteChunksCalled = true
	f.deletedChunkVersion = documentVersionID
	return nil
}

type fakeVectorStore struct {
	upsertCalled bool
	upsertErr    error
	deleteCalled bool
	payloadByID  map[string]map[string]interface{}
}

func (f *fakeVectorStore) Upsert(ctx context.Context, points []infra.PointInput) error {
	f.upsertCalled = true
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.payloadByID == nil {
		f.payloadByID = map[string]map[string]interface{}{}
	}
	for _, point := range points {
		f.payloadByID[point.ID] = point.Payload
	}
	return nil
}

func (f *fakeVectorStore) Delete(ctx context.Context, ids []string) error {
	f.deleteCalled = true
	return nil
}

func (f *fakeVectorStore) GetPayloadByPointID(ctx context.Context, pointID string) (map[string]interface{}, bool, error) {
	payload, ok := f.payloadByID[pointID]
	return payload, ok, nil
}

type fakeEmbedder struct{}

func (f fakeEmbedder) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	out := make([][]float64, 0, len(inputs))
	for idx := range inputs {
		out = append(out, []float64{float64(idx + 1), 0.1})
	}
	return out, nil
}

type fakeStorage struct {
	text string
}

func (f fakeStorage) Read(path string) (string, error) {
	return f.text, nil
}

func testDocTypeFormJSON(t *testing.T) []byte {
	t.Helper()
	form := schema.DocTypeForm{
		Version:      1,
		DocType:      schema.DocType{Code: "law", Name: "Law"},
		SegmentRules: schema.SegmentRules{Strategy: "plain"},
		Metadata: schema.MetadataSchema{
			Fields: []schema.MetadataField{
				{Name: "document_type", Type: "string"},
			},
		},
		MappingRules: []schema.MappingRule{
			{Field: "document_type", Regex: "Title:\\s*(.+)", Group: 1, Default: "law"},
		},
		ReindexPolicy: schema.ReindexPolicy{OnContentChange: true, OnFormChange: true},
	}
	b, err := json.Marshal(form)
	if err != nil {
		t.Fatalf("marshal form: %v", err)
	}
	return b
}

func testBundle(t *testing.T, text string) Bundle {
	t.Helper()
	return Bundle{
		Version:  domain.DocumentVersion{ID: "version-1", DocumentID: "doc-1", AssetID: "asset-1"},
		Document: domain.Document{ID: "doc-1"},
		Asset:    domain.DocumentAsset{ID: "asset-1", StoragePath: "asset.txt"},
		DocType:  domain.DocType{ID: "doctype-1", FormJSON: testDocTypeFormJSON(t)},
		Storage:  fakeStorage{text: text},
	}
}

func TestRunCrashBeforeVectorUpsert_DoesNotReplaceChunks(t *testing.T) {
	store := &fakeIngestStore{countByVersion: 0}
	vectors := &fakeVectorStore{upsertErr: errors.New("qdrant unavailable")}
	svc := &Service{Store: store, Qdrant: vectors, Embed: fakeEmbedder{}, Config: Config{ChunkSize: 50, ChunkOverlap: 0}}

	err := svc.Run(context.Background(), domain.IngestJob{ID: "job-1"}, testBundle(t, "Title: LAW\nOne two three four five six seven eight nine ten"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !vectors.upsertCalled {
		t.Fatal("expected vector upsert to be attempted")
	}
	if store.replaceCalled {
		t.Fatal("expected chunks to remain untouched when vector upsert fails")
	}
}

func TestRunCrashAfterVectorUpsertBeforeChunkCommit_IsRetrySafe(t *testing.T) {
	store := &fakeIngestStore{countByVersion: 0, replaceErr: errors.New("tx failed")}
	vectors := &fakeVectorStore{}
	svc := &Service{Store: store, Qdrant: vectors, Embed: fakeEmbedder{}, Config: Config{ChunkSize: 50, ChunkOverlap: 0}}

	err := svc.Run(context.Background(), domain.IngestJob{ID: "job-1"}, testBundle(t, "Title: LAW\nOne two three four five six seven eight nine ten"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !vectors.upsertCalled {
		t.Fatal("expected vector upsert to be completed before chunk replacement")
	}
	if !store.replaceCalled {
		t.Fatal("expected chunk replacement to be attempted")
	}
}

func TestShouldSkipIngestFalseWhenStaleTailVectorExists(t *testing.T) {
	store := &fakeIngestStore{countByVersion: 2}
	vectors := &fakeVectorStore{
		payloadByID: map[string]map[string]interface{}{
			VectorPointID("version-1", 0): {"content_hash": "c1", "form_hash": "f1"},
			VectorPointID("version-1", 2): {"content_hash": "c0", "form_hash": "f0"},
		},
	}
	svc := &Service{Store: store, Qdrant: vectors}

	skip, err := svc.shouldSkipIngest(context.Background(), "version-1", 2, "c1", "f1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skip {
		t.Fatal("expected skip=false when stale tail vector exists")
	}
}

func TestRecoverPartialIngestDeletesOrphanChunks(t *testing.T) {
	store := &fakeIngestStore{countByVersion: 3}
	vectors := &fakeVectorStore{payloadByID: map[string]map[string]interface{}{}}
	svc := &Service{Store: store, Qdrant: vectors}

	if err := svc.recoverPartialIngest(context.Background(), "version-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.deleteChunksCalled {
		t.Fatal("expected orphan chunks to be deleted")
	}
	if store.deletedChunkVersion != "version-1" {
		t.Fatalf("unexpected deleted version: %s", store.deletedChunkVersion)
	}
}
