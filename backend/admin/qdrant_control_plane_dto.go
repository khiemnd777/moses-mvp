package admin

import "github.com/khiemnd777/legal_api/infra"

type QdrantCollectionSummary struct {
	CollectionName       string                     `json:"collection_name"`
	Status               string                     `json:"status"`
	PointsCount          *int64                     `json:"points_count,omitempty"`
	VectorCount          *int64                     `json:"vector_count,omitempty"`
	IndexedVectorsCount  *int64                     `json:"indexed_vectors_count,omitempty"`
	VectorDimension      int                        `json:"vector_dimension,omitempty"`
	DistanceMetric       string                     `json:"distance_metric,omitempty"`
	Validation           QdrantValidationSummary    `json:"validation"`
	PayloadSchemaSummary []QdrantPayloadSchemaField `json:"payload_schema_summary,omitempty"`
}

type QdrantValidationSummary struct {
	Available         bool   `json:"available"`
	ExpectedDimension int    `json:"expected_dimension,omitempty"`
	Passed            bool   `json:"passed,omitempty"`
	Message           string `json:"message,omitempty"`
}

type QdrantPayloadSchemaField struct {
	Key  string `json:"key"`
	Type string `json:"type,omitempty"`
}

type ListQdrantCollectionsResponse struct {
	Status      string                    `json:"status"`
	Summary     string                    `json:"summary"`
	Collections []QdrantCollectionSummary `json:"collections"`
}

type GetQdrantCollectionResponse struct {
	Status     string                   `json:"status"`
	Summary    string                   `json:"summary"`
	Found      bool                     `json:"found"`
	Collection *QdrantCollectionSummary `json:"collection,omitempty"`
}

type SearchDebugMetadataFilter struct {
	LegalDomain     []string `json:"legal_domain,omitempty"`
	DocumentType    []string `json:"document_type,omitempty"`
	EffectiveStatus []string `json:"effective_status,omitempty"`
	DocumentNumber  []string `json:"document_number,omitempty"`
	ArticleNumber   []string `json:"article_number,omitempty"`
}

type SearchDebugRequest struct {
	QueryText           string                     `json:"query_text"`
	TopK                int                        `json:"top_k"`
	MetadataFilters     *SearchDebugMetadataFilter `json:"metadata_filters,omitempty"`
	Collection          string                     `json:"collection,omitempty"`
	IncludePayload      bool                       `json:"include_payload"`
	IncludeChunkPreview bool                       `json:"include_chunk_preview"`
}

type SearchDebugChunk struct {
	ChunkID           string `json:"chunk_id"`
	DocumentVersionID string `json:"document_version_id,omitempty"`
	ChunkIndex        int    `json:"chunk_index,omitempty"`
	Preview           string `json:"preview,omitempty"`
	Citation          string `json:"citation,omitempty"`
}

type SearchDebugHit struct {
	Rank    int                    `json:"rank"`
	PointID string                 `json:"point_id"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload,omitempty"`
	Chunk   *SearchDebugChunk      `json:"chunk,omitempty"`
}

type SearchDebugResponse struct {
	Status        string           `json:"status"`
	Summary       string           `json:"summary"`
	QueryHash     string           `json:"query_hash"`
	TopK          int              `json:"top_k"`
	FilterSummary string           `json:"filter_summary"`
	Collection    string           `json:"collection"`
	DurationMS    int64            `json:"duration_ms"`
	HitCount      int              `json:"hit_count"`
	Hits          []SearchDebugHit `json:"hits"`
}

type VectorHealthResponse struct {
	Status                    string   `json:"status"`
	Summary                   string   `json:"summary"`
	ScanMode                  string   `json:"scan_mode"`
	ScannedBatches            int      `json:"scanned_batches"`
	ScannedVectors            int      `json:"scanned_vectors"`
	ScannedChunks             int      `json:"scanned_chunks"`
	DurationMS                int64    `json:"duration_ms"`
	Bounded                   bool     `json:"bounded"`
	OrphanVectorsCount        int      `json:"orphan_vectors_count"`
	MissingVectorsCount       int      `json:"missing_vectors_count"`
	ChunkVectorCountMismatch  bool     `json:"chunk_vector_count_mismatch"`
	DimensionMismatchDetected bool     `json:"dimension_mismatch_detected"`
	RepairableIssuesDetected  bool     `json:"repairable_issues_detected"`
	RepairRecommendation      string   `json:"repair_recommendation"`
	Samples                   []string `json:"samples,omitempty"`
}

type DeleteByFilterRequest struct {
	Collection string       `json:"collection"`
	Filter     infra.Filter `json:"filter"`
	Confirm    bool         `json:"confirm"`
	DryRun     bool         `json:"dry_run"`
	Reason     string       `json:"reason,omitempty"`
}

type DeleteByFilterResponse struct {
	Status         string `json:"status"`
	Summary        string `json:"summary"`
	Collection     string `json:"collection"`
	DryRun         bool   `json:"dry_run"`
	Confirmed      bool   `json:"confirmed"`
	FilterSummary  string `json:"filter_summary"`
	EstimatedScope *int64 `json:"estimated_scope,omitempty"`
	ScopeEstimated bool   `json:"scope_estimated"`
}

type ReindexDocumentRequest struct {
	DocumentID        string `json:"document_id,omitempty"`
	DocumentVersionID string `json:"document_version_id,omitempty"`
	Force             bool   `json:"force"`
	Reason            string `json:"reason,omitempty"`
}

type ReindexAllRequest struct {
	Confirm     bool   `json:"confirm"`
	Force       bool   `json:"force"`
	DocTypeCode string `json:"doc_type_code,omitempty"`
	Collection  string `json:"collection,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type ReindexEnqueueItem struct {
	DocumentVersionID string `json:"document_version_id"`
	JobID             string `json:"job_id"`
	JobStatus         string `json:"job_status"`
	Created           bool   `json:"created"`
}

type ReindexAcceptedResponse struct {
	Status        string               `json:"status"`
	Summary       string               `json:"summary"`
	Scope         map[string]string    `json:"scope,omitempty"`
	AcceptedCount int                  `json:"accepted_count"`
	CreatedCount  int                  `json:"created_count"`
	SkippedCount  int                  `json:"skipped_count"`
	Items         []ReindexEnqueueItem `json:"items,omitempty"`
}
