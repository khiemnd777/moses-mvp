package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
)

type downloadAssetStore struct {
	fakeStore
	asset domain.DocumentAsset
	err   error
}

func (s *downloadAssetStore) GetDocumentAsset(ctx context.Context, id string) (domain.DocumentAsset, error) {
	return s.asset, s.err
}

func TestDownloadAssetStreamsOriginalDocumentHeaders(t *testing.T) {
	root := t.TempDir()
	storagePath := filepath.Join("doc-1", "asset_report.docx")
	fullPath := filepath.Join(root, storagePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir temp asset dir: %v", err)
	}
	wantBody := []byte("fake-docx-binary")
	if err := os.WriteFile(fullPath, wantBody, 0o644); err != nil {
		t.Fatalf("write temp asset: %v", err)
	}

	h := &Handler{
		Store: &downloadAssetStore{
			asset: domain.DocumentAsset{
				ID:          "asset-1",
				FileName:    "Quyet-dinh.docx",
				ContentType: "application/zip",
				StoragePath: storagePath,
			},
		},
		Storage: infra.NewStorage(root),
	}

	app := fiber.New()
	app.Get("/assets/:id/download", h.DownloadAsset)

	req := httptest.NewRequest(http.MethodGet, "/assets/asset-1/download", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("content-type = %q", got)
	}
	disposition := resp.Header.Get("Content-Disposition")
	if !strings.Contains(disposition, `filename="Quyet-dinh.docx"`) {
		t.Fatalf("content-disposition = %q", disposition)
	}

	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatalf("body = %q, want %q", gotBody, wantBody)
	}
}
