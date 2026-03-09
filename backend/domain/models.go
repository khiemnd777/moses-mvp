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
