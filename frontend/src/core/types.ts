export type Role = 'user' | 'assistant' | 'system';

export type Citation = {
  id: string;
  document_title: string;
  document_number: string;
  article: string;
  clause: string;
  year: number;
  excerpt: string;
  url: string;
};

export type ChatMessage = {
  id: string;
  role: Role;
  content: string;
  citations?: Citation[];
  createdAt: number;
};

export type ChatFilters = {
  tone: string;
  topK: number;
  effectiveStatus: string;
  domain: string;
  docType: string;
};

export type DocTypeForm = {
  version: number;
  doc_type: {
    code: string;
    name: string;
  };
  segment_rules: {
    strategy: string;
    hierarchy: string;
    normalization: string;
  };
  metadata_schema: {
    fields: Array<{
      name: string;
      type: string;
    }>;
  };
  mapping_rules: Array<{
    field: string;
    regex: string;
    group?: number;
    default?: string;
    value_map?: Record<string, string>;
  }>;
  reindex_policy: {
    on_content_change: boolean;
    on_form_change: boolean;
  };
};

export type DocType = {
  id: string;
  code: string;
  name: string;
  form: DocTypeForm;
  form_hash?: string;
  created_at?: string;
  updated_at?: string;
};

export type DocumentItem = {
  id: string;
  title: string;
  doc_type_code: string;
  assets?: Array<{
    file_name: string;
    content_type: string;
    created_at?: string;
    versions?: number[];
  }>;
  created_at?: string;
  updated_at?: string;
};

export type IngestJob = {
  id: string;
  document_version_id: string;
  status: string;
  error_message?: string;
  created_at?: string;
  updated_at?: string;
};

export type AIGuardPolicy = {
  id: string;
  name: string;
  enabled: boolean;
  min_retrieved_chunks: number;
  min_similarity_score: number;
  on_empty_retrieval: 'refuse' | 'fallback_llm' | 'ask_clarification';
  on_low_confidence: 'refuse' | 'fallback_llm' | 'ask_clarification';
  created_at?: string;
  updated_at?: string;
};

export type AIPrompt = {
  id: string;
  name: string;
  prompt_type: string;
  system_prompt: string;
  temperature: number;
  max_tokens: number;
  retry: number;
  enabled: boolean;
  created_at?: string;
  updated_at?: string;
};

export type AIRetrievalConfig = {
  id: string;
  name: string;
  enabled: boolean;
  default_top_k: number;
  rerank_enabled: boolean;
  rerank_vector_weight: number;
  rerank_keyword_weight: number;
  rerank_metadata_weight: number;
  rerank_article_weight: number;
  adjacent_chunk_enabled: boolean;
  adjacent_chunk_window: number;
  max_context_chunks: number;
  max_context_chars: number;
  default_effective_status: string;
  preferred_doc_types_json: string[];
  legal_domain_defaults_json: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
};

export type QdrantCollectionValidation = {
  available: boolean;
  expected_dimension?: number;
  passed?: boolean;
  message?: string;
};

export type QdrantPayloadSchemaField = {
  key: string;
  type?: string;
};

export type QdrantCollectionSummary = {
  collection_name: string;
  status: string;
  points_count?: number;
  vector_count?: number;
  indexed_vectors_count?: number;
  vector_dimension?: number;
  distance_metric?: string;
  validation: QdrantCollectionValidation;
  payload_schema_summary?: QdrantPayloadSchemaField[];
};

export type QdrantCollectionsResponse = {
  status: string;
  summary: string;
  collections: QdrantCollectionSummary[];
};

export type QdrantCollectionDetailResponse = {
  status: string;
  summary: string;
  found: boolean;
  collection?: QdrantCollectionSummary;
};

export type SearchDebugMetadataFilters = {
  legal_domain?: string[];
  document_type?: string[];
  effective_status?: string[];
  document_number?: string[];
  article_number?: string[];
};

export type SearchDebugRequest = {
  query_text: string;
  top_k?: number;
  metadata_filters?: SearchDebugMetadataFilters;
  collection?: string;
  include_payload?: boolean;
  include_chunk_preview?: boolean;
};

export type SearchDebugChunk = {
  chunk_id: string;
  document_version_id?: string;
  chunk_index?: number;
  preview?: string;
  citation?: string;
};

export type SearchDebugHit = {
  rank: number;
  point_id: string;
  score: number;
  payload?: Record<string, unknown>;
  chunk?: SearchDebugChunk;
};

export type SearchDebugResponse = {
  status: string;
  summary: string;
  query_hash: string;
  top_k: number;
  filter_summary: string;
  collection: string;
  duration_ms: number;
  hit_count: number;
  hits: SearchDebugHit[];
};

export type VectorHealthResponse = {
  status: string;
  summary: string;
  scan_mode: string;
  scanned_batches: number;
  scanned_vectors: number;
  scanned_chunks: number;
  duration_ms: number;
  bounded: boolean;
  orphan_vectors_count: number;
  missing_vectors_count: number;
  chunk_vector_count_mismatch: boolean;
  dimension_mismatch_detected: boolean;
  repairable_issues_detected: boolean;
  repair_recommendation: string;
  samples?: string[];
};

export type DeleteByFilterFilter = {
  must: Array<{
    key: string;
    match: {
      value?: unknown;
      any?: string[];
    };
  }>;
};

export type DeleteByFilterRequest = {
  collection?: string;
  filter: DeleteByFilterFilter;
  confirm: boolean;
  dry_run: boolean;
  reason?: string;
};

export type DeleteByFilterResponse = {
  status: string;
  summary: string;
  collection: string;
  dry_run: boolean;
  confirmed: boolean;
  filter_summary: string;
  estimated_scope?: number;
  scope_estimated: boolean;
};

export type ReindexDocumentRequest = {
  document_id?: string;
  document_version_id?: string;
  force?: boolean;
  reason?: string;
};

export type ReindexAllRequest = {
  confirm: boolean;
  force?: boolean;
  doc_type_code?: string;
  collection?: string;
  status?: string;
  limit?: number;
  reason: string;
};

export type ReindexEnqueueItem = {
  document_version_id: string;
  job_id: string;
  job_status: string;
  created: boolean;
};

export type ReindexAcceptedResponse = {
  status: string;
  summary: string;
  scope?: Record<string, string>;
  accepted_count: number;
  created_count: number;
  skipped_count: number;
  items?: ReindexEnqueueItem[];
};
