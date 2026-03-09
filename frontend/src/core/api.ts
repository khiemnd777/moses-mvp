import axios from 'axios';
import type {
  AIGuardPolicy,
  AIPrompt,
  ChatFilters,
  DocType,
  DocTypeForm,
  DocumentItem,
  IngestJob
} from './types';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL
});

const adminApiKey = import.meta.env.VITE_ADMIN_API_KEY;
if (adminApiKey) {
  api.defaults.headers.common['X-Admin-Key'] = adminApiKey;
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

export default api;
