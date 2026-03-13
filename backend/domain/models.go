package domain

import "time"

type DocType struct {
	ID        string
	Code      string
	Name      string
	FormJSON  []byte
	FormHash  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Document struct {
	ID          string
	DocTypeID   string
	DocTypeCode string
	Title       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DocumentAsset struct {
	ID          string
	DocumentID  string
	FileName    string
	ContentType string
	StoragePath string
	CreatedAt   time.Time
}

type DocumentAssetWithVersions struct {
	ID          string
	DocumentID  string
	FileName    string
	ContentType string
	StoragePath string
	CreatedAt   time.Time
	Versions    []int
}

type DocumentVersion struct {
	ID         string
	DocumentID string
	AssetID    string
	Version    int
	CreatedAt  time.Time
}

type Chunk struct {
	ID                string
	DocumentVersionID string
	Index             int
	Text              string
	MetadataJSON      []byte
	EmbeddingJSON     []byte
	CreatedAt         time.Time
}

type IngestJob struct {
	ID                string
	DocumentVersionID string
	Status            string
	ErrorMessage      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type QueryLog struct {
	ID        string
	Query     string
	CreatedAt time.Time
}

type AnswerLog struct {
	ID        string
	Query     string
	Answer    string
	CreatedAt time.Time
}

type Conversation struct {
	ID            string
	Title         string
	UserID        *string
	LastMessage   *string
	LastMessageAt *time.Time
	MessageCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Message struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	CitationsJSON  []byte
	TraceID        *string
	CreatedAt      time.Time
}

type User struct {
	ID                 string
	Username           string
	PasswordHash       string
	Role               string
	MustChangePassword bool
	PasswordChangedAt  *time.Time
	CreatedAt          time.Time
}

type AIGuardPolicy struct {
	ID                 string
	Name               string
	Enabled            bool
	MinRetrievedChunks int
	MinSimilarityScore float64
	OnEmptyRetrieval   string
	OnLowConfidence    string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type AIPrompt struct {
	ID           string
	Name         string
	PromptType   string
	SystemPrompt string
	Temperature  float64
	MaxTokens    int
	Retry        int
	Enabled      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type AIRetrievalConfig struct {
	ID                      string                 `json:"id"`
	Name                    string                 `json:"name"`
	Enabled                 bool                   `json:"enabled"`
	DefaultTopK             int                    `json:"default_top_k"`
	RerankEnabled           bool                   `json:"rerank_enabled"`
	RerankVectorWeight      float64                `json:"rerank_vector_weight"`
	RerankKeywordWeight     float64                `json:"rerank_keyword_weight"`
	RerankMetadataWeight    float64                `json:"rerank_metadata_weight"`
	RerankArticleWeight     float64                `json:"rerank_article_weight"`
	AdjacentChunkEnabled    bool                   `json:"adjacent_chunk_enabled"`
	AdjacentChunkWindow     int                    `json:"adjacent_chunk_window"`
	MaxContextChunks        int                    `json:"max_context_chunks"`
	MaxContextChars         int                    `json:"max_context_chars"`
	DefaultEffectiveStatus  string                 `json:"default_effective_status"`
	PreferredDocTypes       []string               `json:"preferred_doc_types_json"`
	LegalDomainDefaultsJSON map[string]interface{} `json:"legal_domain_defaults_json"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}
