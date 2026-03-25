import axios from 'axios';
import apiClient from '@/playground/apiClient.js';
import type {
  AIGuardPolicy,
  AIPrompt,
  AIRetrievalConfig,
  Citation,
  CitationDetail,
  ChatFilters,
  ChatMessage,
  Conversation,
  DeleteByFilterRequest,
  DeleteByFilterResponse,
  DocType,
  DocTypeForm,
  DocumentItem,
  IngestJob,
  QdrantCollectionDetailResponse,
  QdrantCollectionsResponse,
  ReindexAcceptedResponse,
  ReindexAllRequest,
  ReindexDocumentRequest,
  SearchDebugRequest,
  SearchDebugResponse,
  VectorHealthResponse
} from './types';

const api = apiClient;

const adminApiKey = import.meta.env.VITE_ADMIN_API_KEY;
if (adminApiKey) {
  api.defaults.headers.common['X-Admin-Key'] = adminApiKey;
}
const adminBearerToken = import.meta.env.VITE_ADMIN_BEARER_TOKEN;
if (adminBearerToken) {
  api.defaults.headers.common.Authorization = `Bearer ${adminBearerToken}`;
}

export type ApiError = {
  code: string;
  message: string;
  details?: unknown;
};

export const unwrapError = (error: unknown): string => {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as { error?: ApiError } | undefined;
    if (data?.error?.message) return data.error.message;
    return error.message;
  }
  if (error instanceof Error) return error.message;
  return 'Unknown error';
};

export const answer = async (question: string, filters: ChatFilters) => {
  const { data } = await api.post('/answer', { question, filters });
  return data as { answer: string; citations?: unknown[] };
};

export const createConversation = async (title?: string) => {
  const { data } = await api.post('/conversations', title ? { title } : {});
  return data as Conversation;
};

export const listConversations = async () => {
  const { data } = await api.get('/conversations');
  return (data?.items || []) as Conversation[];
};

export const getConversation = async (id: string) => {
  const { data } = await api.get(`/conversations/${id}`);
  return data as Conversation;
};

export const deleteConversation = async (id: string) => {
  await api.delete(`/conversations/${id}`);
};

export const listMessages = async (conversationId: string) => {
  const { data } = await api.get('/messages', { params: { conversation_id: conversationId } });
  return (data?.items || []) as ChatMessage[];
};

export const createMessage = async (payload: { conversation_id?: string; content: string; filters: ChatFilters }) => {
  const { data } = await api.post('/messages', payload);
  return data as {
    conversation: Conversation;
    user_message: ChatMessage;
    message: ChatMessage;
    trace_id: string;
  };
};

export const getCitationDetail = async (payload: { chunk_id?: string; asset_id?: string }) => {
  const { data } = await api.get('/citations/detail', { params: payload });
  return data as CitationDetail;
};

const parseFileName = (contentDisposition?: string, fallback = 'citation.txt') => {
  if (!contentDisposition) return fallback;
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1]);
  }
  const plainMatch = contentDisposition.match(/filename="?([^"]+)"?/i);
  return plainMatch?.[1] || fallback;
};

const extractAssetId = (citation: Citation) => {
  if (citation.asset_id) return citation.asset_id;
  const matched = (citation.file_url || citation.url || '').match(/\/assets\/([^/]+)\/download/);
  return matched?.[1];
};

export const downloadCitationAsset = async (citation: Citation, fallbackFileName?: string) => {
  const assetId = extractAssetId(citation);
  if (assetId) {
    const response = await api.get(`/assets/${assetId}/download`, { responseType: 'blob' });
    const blobUrl = window.URL.createObjectURL(response.data as Blob);
    const anchor = document.createElement('a');
    anchor.href = blobUrl;
    anchor.download = parseFileName(
      typeof response.headers['content-disposition'] === 'string' ? response.headers['content-disposition'] : undefined,
      fallbackFileName || citation.document_title || 'citation'
    );
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    window.URL.revokeObjectURL(blobUrl);
    return;
  }

  const fallbackUrl = citation.file_url || citation.url;
  if (fallbackUrl) {
    window.open(fallbackUrl, '_blank', 'noopener,noreferrer');
    return;
  }

  throw new Error('Không tìm thấy tài liệu để tải xuống.');
};

export const search = async (query: string) => {
  const { data } = await api.post('/search', { query });
  return data as { results: unknown[] };
};

export const listDocTypes = async () => {
  const { data } = await api.get('/doc-types');
  return data as DocType[];
};

export const createDocType = async (payload: { code: string; name: string; form: DocTypeForm }) => {
  const { data } = await api.post('/doc-types', payload);
  return data as DocType;
};

export const updateDocType = async (id: string, payload: { form: DocTypeForm }) => {
  const { data } = await api.put(`/doc-types/${id}/form`, payload);
  return data as DocType;
};

export const deleteDocType = async (id: string) => {
  await api.delete(`/doc-types/${id}`);
};

export const listDocuments = async () => {
  const { data } = await api.get('/documents');
  return data as DocumentItem[];
};

export const createDocument = async (payload: { title: string; doc_type_code: string }) => {
  const { data } = await api.post('/documents', payload);
  return data as DocumentItem;
};

export const deleteDocument = async (id: string) => {
  await api.delete(`/documents/${id}`);
};

export const uploadDocumentAsset = async (id: string, file: File) => {
  const form = new FormData();
  form.append('file', file);
  const { data } = await api.post(`/documents/${id}/assets`, form);
  return data as { id: string };
};

export const createDocumentVersion = async (id: string, payload: { asset_id: string }) => {
  const { data } = await api.post(`/documents/${id}/versions`, payload);
  return data as { id: string };
};

export const enqueueIngestJob = async (documentVersionId: string) => {
  const { data } = await api.post(`/document-versions/${documentVersionId}/ingest`);
  return data as { id: string; status: string };
};

export const listIngestJobs = async () => {
  const { data } = await api.get('/ingest-jobs');
  return data as IngestJob[];
};

export const deleteIngestJob = async (id: string) => {
  await api.delete(`/ingest-jobs/${id}`);
};

export const listAIGuardPolicies = async () => {
  const { data } = await api.get('/admin/ai/guard-policies');
  return (data?.items || []) as AIGuardPolicy[];
};

export const getAIGuardPolicy = async (id: string) => {
  const { data } = await api.get(`/admin/ai/guard-policies/${id}`);
  return data as AIGuardPolicy;
};

export const createAIGuardPolicy = async (payload: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'>) => {
  const { data } = await api.post('/admin/ai/guard-policies', payload);
  return data as AIGuardPolicy;
};

export const updateAIGuardPolicy = async (
  id: string,
  payload: Omit<AIGuardPolicy, 'id' | 'created_at' | 'updated_at'>
) => {
  const { data } = await api.put(`/admin/ai/guard-policies/${id}`, payload);
  return data as AIGuardPolicy;
};

export const deleteAIGuardPolicy = async (id: string) => {
  await api.delete(`/admin/ai/guard-policies/${id}`);
};

export const listAIPrompts = async () => {
  const { data } = await api.get('/admin/ai/prompts');
  return (data?.items || []) as AIPrompt[];
};

export const getAIPrompt = async (id: string) => {
  const { data } = await api.get(`/admin/ai/prompts/${id}`);
  return data as AIPrompt;
};

export const createAIPrompt = async (payload: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'>) => {
  const { data } = await api.post('/admin/ai/prompts', payload);
  return data as AIPrompt;
};

export const updateAIPrompt = async (id: string, payload: Omit<AIPrompt, 'id' | 'created_at' | 'updated_at'>) => {
  const { data } = await api.put(`/admin/ai/prompts/${id}`, payload);
  return data as AIPrompt;
};

export const deleteAIPrompt = async (id: string) => {
  await api.delete(`/admin/ai/prompts/${id}`);
};

export const testAIPrompt = async (payload: { prompt_id: string; query: string; top_k?: number }) => {
  const { data } = await api.post('/admin/ai/prompts/test', payload);
  return data as { answer: string; citations?: unknown[] };
};

export const listAIRetrievalConfigs = async () => {
  const { data } = await api.get('/admin/ai/retrieval-configs');
  return (data?.items || []) as AIRetrievalConfig[];
};

export const getAIRetrievalConfig = async (id: string) => {
  const { data } = await api.get(`/admin/ai/retrieval-configs/${id}`);
  return data as AIRetrievalConfig;
};

export const createAIRetrievalConfig = async (payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'>) => {
  const { data } = await api.post('/admin/ai/retrieval-configs', payload);
  return data as AIRetrievalConfig;
};

export const updateAIRetrievalConfig = async (
  id: string,
  payload: Omit<AIRetrievalConfig, 'id' | 'created_at' | 'updated_at'>
) => {
  const { data } = await api.put(`/admin/ai/retrieval-configs/${id}`, payload);
  return data as AIRetrievalConfig;
};

export const enableAIRetrievalConfig = async (id: string) => {
  const { data } = await api.post(`/admin/ai/retrieval-configs/${id}/enable`);
  return data as AIRetrievalConfig;
};

export const disableAIRetrievalConfig = async (id: string) => {
  const { data } = await api.post(`/admin/ai/retrieval-configs/${id}/disable`);
  return data as AIRetrievalConfig;
};

export const deleteAIRetrievalConfig = async (id: string) => {
  await api.delete(`/admin/ai/retrieval-configs/${id}`);
};

export const listQdrantCollections = async () => {
  const { data } = await api.get('/admin/qdrant/collections');
  return data as QdrantCollectionsResponse;
};

export const getQdrantCollection = async (name: string) => {
  const { data } = await api.get(`/admin/qdrant/collections/${encodeURIComponent(name)}`);
  return data as QdrantCollectionDetailResponse;
};

export const searchDebugQdrant = async (payload: SearchDebugRequest) => {
  const { data } = await api.post('/admin/qdrant/search_debug', payload);
  return data as SearchDebugResponse;
};

export const getQdrantVectorHealth = async (params?: {
  mode?: 'quick' | 'full';
  full_scan?: boolean;
  batch_size?: number;
  chunk_batch_size?: number;
  vector_batch_size?: number;
  max_vectors_scanned?: number;
  max_chunks?: number;
  max_scan_duration_ms?: number;
}) => {
  const { data } = await api.get('/admin/qdrant/vector_health', { params });
  return data as VectorHealthResponse;
};

export const deleteByFilterQdrant = async (payload: DeleteByFilterRequest) => {
  const { data } = await api.post('/admin/qdrant/delete_by_filter', payload);
  return data as DeleteByFilterResponse;
};

export const reindexQdrantDocument = async (payload: ReindexDocumentRequest) => {
  const { data } = await api.post('/admin/qdrant/reindex_document', payload);
  return data as ReindexAcceptedResponse;
};

export const reindexQdrantAll = async (payload: ReindexAllRequest) => {
  const { data } = await api.post('/admin/qdrant/reindex_all', payload);
  return data as ReindexAcceptedResponse;
};

export default api;
