package api

import (
	"context"

	"github.com/khiemnd777/legal_api/core/retrieval"
	"github.com/khiemnd777/legal_api/domain"
	"github.com/khiemnd777/legal_api/infra"
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
	ListDocumentVersionIDsByDocument(ctx context.Context, documentID string) ([]string, error)
	ListChunkIDsByVersion(ctx context.Context, documentVersionID string) ([]string, error)
	EnqueueDeleteVectorsRepair(ctx context.Context, collection, documentID, documentVersionID string, filter infra.Filter) (bool, error)
	EnqueueRebuildVectorsRepair(ctx context.Context, collection, documentVersionID string) (bool, error)
	ListDocumentAssetPaths(ctx context.Context, documentID string) ([]string, error)
	ListDocumentAssets(ctx context.Context, documentID string) ([]domain.DocumentAssetWithVersions, error)
	CreateDocumentAsset(ctx context.Context, documentID, fileName, contentType, storagePath string) (string, error)
	GetDocumentAsset(ctx context.Context, id string) (domain.DocumentAsset, error)
	CreateDocumentVersion(ctx context.Context, documentID, assetID string) (string, error)
	GetDocumentVersion(ctx context.Context, id string) (domain.DocumentVersion, error)
	GetDocumentVersionBundle(ctx context.Context, id string) (domain.DocumentVersion, domain.Document, domain.DocumentAsset, domain.DocType, error)
	DeleteDocumentVersion(ctx context.Context, id string) (bool, error)
	ListIngestJobs(ctx context.Context) ([]domain.IngestJob, error)
	DeleteIngestJob(ctx context.Context, id string) (bool, error)
	EnqueueIngestJob(ctx context.Context, documentVersionID string) (domain.IngestJob, bool, error)
	LogQuery(ctx context.Context, q string) error
	LogAnswer(ctx context.Context, q, a string) error
	CreateConversation(ctx context.Context, title string, userID *string) (domain.Conversation, error)
	ListConversations(ctx context.Context, userID *string) ([]domain.Conversation, error)
	GetConversation(ctx context.Context, id string) (domain.Conversation, error)
	DeleteConversation(ctx context.Context, id string) (bool, error)
	UpdateConversationTitle(ctx context.Context, id, title string) error
	CreateMessage(ctx context.Context, conversationID, role, content string, citationsJSON []byte, traceID *string) (domain.Message, error)
	UpdateMessage(ctx context.Context, id, content string, citationsJSON []byte, traceID *string) error
	ListMessagesByConversation(ctx context.Context, conversationID string) ([]domain.Message, error)
	GetActiveAIGuardPolicy(ctx context.Context) (domain.AIGuardPolicy, error)
	GetActiveAIPromptByType(ctx context.Context, promptType string) (domain.AIPrompt, error)
	ListEnabledAIPrompts(ctx context.Context) ([]domain.AIPrompt, error)
}

type retriever interface {
	Search(ctx context.Context, query string, opts retrieval.SearchOptions) ([]retrieval.Result, error)
}
