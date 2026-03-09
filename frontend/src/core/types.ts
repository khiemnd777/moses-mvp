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
