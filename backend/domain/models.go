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

type DocumentUpload struct {
	ID                string
	Title             string
	FileName          string
	ContentType       string
	StoragePath       string
	FileSizeBytes     int64
	SHA256            string
	Status            string
	AnalysisJSON      []byte
	ErrorMessage      *string
	DocumentID        *string
	DocumentAssetID   *string
	DocumentVersionID *string
	Events            []DocumentUploadEvent
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DocumentUploadInput struct {
	Title         string
	FileName      string
	ContentType   string
	StoragePath   string
	FileSizeBytes int64
	SHA256        string
	Status        string
	AnalysisJSON  []byte
	Events        []DocumentUploadEventInput
}

type DocumentUploadEvent struct {
	ID            string
	UploadID      *string
	EventType     string
	Stage         string
	Status        string
	Message       string
	EvidenceJSON  []byte
	Actor         string
	FileName      string
	ContentType   string
	FileSizeBytes int64
	SHA256        string
	CreatedAt     time.Time
}

type DocumentUploadEventInput struct {
	UploadID      *string
	EventType     string
	Stage         string
	Status        string
	Message       string
	EvidenceJSON  []byte
	Actor         string
	FileName      string
	ContentType   string
	FileSizeBytes int64
	SHA256        string
}

type DocumentUploadPromotion struct {
	DocumentID        string
	DocumentAssetID   string
	DocumentVersionID string
	IngestJobID       string
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

type PipelineHealthOptions struct {
	RecentSince time.Time
	StaleBefore time.Time
	Limit       int
}

type PipelineHealth struct {
	GeneratedAt       time.Time
	RecentSince       time.Time
	StaleBefore       time.Time
	Severity          string
	Alerts            []PipelineHealthAlert
	Summary           PipelineHealthSummary
	UploadStatusCount []PipelineStatusCount
	JobStatusCount    []PipelineStatusCount
	StageStatusCount  []PipelineStageStatusCount
	Security          PipelineSecurityStats
	Latency           PipelineLatencyStats
	StaleUploads      []PipelineHealthIssue
	RecentIssues      []PipelineHealthIssue
}

type PipelineHealthAlert struct {
	Code      string
	Severity  string
	Message   string
	Value     float64
	Threshold float64
}

type PipelineHealthSummary struct {
	TotalUploads        int
	ProcessingUploads   int
	ReviewUploads       int
	FailedUploads       int
	PublishedUploads    int
	ActiveJobs          int
	FailedJobs          int
	StaleUploads        int
	RecentIssues        int
	SecurityBlocked     int
	SecurityUnavailable int
}

type PipelineStatusCount struct {
	Status string
	Count  int
}

type PipelineStageStatusCount struct {
	Stage  string
	Status string
	Count  int
}

type PipelineSecurityStats struct {
	Passed      int
	Blocked     int
	Unavailable int
}

type PipelineLatencyStats struct {
	CompletedCount int
	AverageSeconds float64
	P50Seconds     float64
	P95Seconds     float64
	MaxSeconds     float64
}

type PipelineHealthIssue struct {
	UploadID       *string
	Title          string
	FileName       string
	Status         string
	Stage          string
	EventStatus    string
	Message        string
	ErrorMessage   string
	AgeSeconds     int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	EventCreatedAt *time.Time
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

const (
	TelegramBotStatusRunning = "running"
	TelegramBotStatusStopped = "stopped"
	TelegramBotStatusError   = "error"
)

type TelegramBot struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	Token                  string     `json:"-"`
	TokenHint              string     `json:"token_hint"`
	BotUsername            string     `json:"bot_username,omitempty"`
	Status                 string     `json:"status"`
	DefaultTone            string     `json:"default_tone"`
	DefaultTopK            int        `json:"default_top_k"`
	DefaultEffectiveStatus string     `json:"default_effective_status"`
	DefaultDomain          string     `json:"default_domain"`
	DefaultDocType         string     `json:"default_doc_type"`
	AllowedChatIDs         []int64    `json:"allowed_chat_ids"`
	WelcomeMessage         string     `json:"welcome_message"`
	LastUpdateID           int64      `json:"last_update_id"`
	LastError              *string    `json:"last_error,omitempty"`
	StartedAt              *time.Time `json:"started_at,omitempty"`
	StoppedAt              *time.Time `json:"stopped_at,omitempty"`
	ChatCount              int        `json:"chat_count"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type TelegramChatLink struct {
	ID             string
	BotID          string
	ChatID         int64
	ChatType       string
	ChatTitle      string
	ConversationID string
	LastMessageAt  *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

type RefreshSession struct {
	ID             string
	UserID         string
	TokenHash      string
	ExpiresAt      time.Time
	RevokedAt      *time.Time
	ReplacedByHash *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AIGuardPolicy struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Enabled            bool      `json:"enabled"`
	MinRetrievedChunks int       `json:"min_retrieved_chunks"`
	MinSimilarityScore float64   `json:"min_similarity_score"`
	OnEmptyRetrieval   string    `json:"on_empty_retrieval"`
	OnLowConfidence    string    `json:"on_low_confidence"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AIPrompt struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PromptType   string    `json:"prompt_type"`
	SystemPrompt string    `json:"system_prompt"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"max_tokens"`
	Retry        int       `json:"retry"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
