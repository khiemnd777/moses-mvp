package api

import (
	"context"

	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
)

type handlerStore interface {
	CreateDocType(ctx context.Context, code, name string, formJSON []byte, formHash string) (string, error)
	ListDocTypes(ctx context.Context) ([]domain.DocType, error)
	UpdateDocTypeForm(ctx context.Context, id string, formJSON []byte, formHash string) error
	CountDocumentsByDocType(ctx context.Context, docTypeID string) (int, error)
	DeleteDocType(ctx context.Context, id string) (bool, error)
	GetDocType(ctx context.Context, id string) (domain.DocType, error)
	GetDocTypeByCode(ctx context.Context, code string) (domain.DocType, error)
	CreateDocument(ctx context.Context, docTypeID, title string) (string, error)
	GetDocument(ctx context.Context, id string) (domain.Document, error)
	ListDocuments(ctx context.Context) ([]domain.Document, error)
	DeleteDocument(ctx context.Context, id string) (bool, error)
	ListDocumentAssetPaths(ctx context.Context, documentID string) ([]string, error)
	ListDocumentAssets(ctx context.Context, documentID string) ([]domain.DocumentAssetWithVersions, error)
	CreateDocumentAsset(ctx context.Context, documentID, fileName, contentType, storagePath string) (string, error)
	GetDocumentAsset(ctx context.Context, id string) (domain.DocumentAsset, error)
	CreateDocumentVersion(ctx context.Context, documentID, assetID string) (string, error)
	GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error)
	ListIngestJobs(ctx context.Context) ([]domain.IngestJob, error)
	DeleteIngestJob(ctx context.Context, id string) (bool, error)
	EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error)
	LogQuery(ctx context.Context, q string) error
	LogAnswer(ctx context.Context, q, a string) error
	GetActiveAIGuardPolicy(ctx context.Context) (domain.AIGuardPolicy, error)
	GetActiveAIPromptByType(ctx context.Context, promptType string) (domain.AIPrompt, error)
}

type retriever interface {
	Search(ctx context.Context, query string, opts retrieval.SearchOptions) ([]retrieval.Result, error)
}
