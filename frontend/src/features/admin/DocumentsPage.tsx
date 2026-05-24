import { useEffect, useMemo, useState, type ChangeEvent } from 'react';
import Panel from '@/shared/Panel';
import Button from '@/shared/Button';
import Input from '@/shared/Input';
import {
  getPipelineHealth,
  isEndpointUnavailableError,
  listDocumentUploads,
  listDocuments,
  listIngestJobs,
  runDocumentAdminAction,
  unwrapError,
  uploadDocumentOnly
} from '@/core/api';
import type {
  DocumentAdminActionDescriptor,
  DocumentAdminActionName,
  DocumentItem,
  DocumentLifecycleStatus,
  DocumentUploadEvent,
  DocumentUploadItem,
  IngestJob,
  PipelineHealthIssue,
  PipelineHealthResponse
} from '@/core/types';
import {
  ArticleIcon,
  DeleteIcon,
  FactCheckIcon,
  HealthIcon,
  QueueIcon,
  ScienceIcon,
  StorageIcon,
  SyncIcon,
  UploadFileIcon,
  VisibilityIcon
} from '@/shared/muiIcons';

type StageState = 'done' | 'active' | 'pending' | 'warning' | 'blocked';

type PipelineStageKey =
  | 'upload'
  | 'security'
  | 'extract'
  | 'normalize_vi'
  | 'classify'
  | 'profile'
  | 'document_version'
  | 'chunk'
  | 'embed_index'
  | 'validate'
  | 'publish';

type PipelineStage = {
  key: PipelineStageKey;
  label: string;
  state: StageState;
  detail?: string;
  event?: DocumentUploadEvent;
};

type ApprovalCandidate = {
  code: string;
  name?: string;
  score?: number;
};

const DOCUMENT_ACTIONS: Array<{ name: DocumentAdminActionName; label: string }> = [
  { name: 'approve', label: 'Duyệt & publish' },
  { name: 'reindex', label: 'Reindex' },
  { name: 'archive', label: 'Lưu trữ' },
  { name: 'reject', label: 'Từ chối' }
];

const TERMINAL_FAILED = new Set(['extract_failed', 'validation_failed', 'rejected', 'security_failed']);
const REVIEW_STATUSES = new Set(['classification_low_confidence', 'needs_review', 'validation_failed']);
const PROCESSING_STATUSES = new Set(['uploaded', 'extracting', 'classified', 'profile_resolved', 'indexing', 'validating']);

const STAGE_LABELS: Record<PipelineStageKey, string> = {
  upload: 'Upload',
  security: 'Quét an toàn',
  extract: 'Trích xuất',
  normalize_vi: 'Chuẩn hóa tiếng Việt',
  classify: 'Phân loại',
  profile: 'Hồ sơ ingest',
  document_version: 'Document/version',
  chunk: 'Chunk',
  embed_index: 'Embed/Index',
  validate: 'Validate',
  publish: 'Publish'
};

const STATUS_LABELS: Record<string, string> = {
  uploaded: 'Đã nhận file',
  security_scanning: 'Đang quét an toàn',
  security_failed: 'Không đạt quét an toàn',
  extracting: 'Đang trích xuất',
  classified: 'Đã phân loại',
  normalizing_vi: 'Đang chuẩn hóa tiếng Việt',
  profile_resolved: 'Đã resolve hồ sơ ingest',
  indexing: 'Đang chunk/embed/index',
  validating: 'Đang validate',
  ready: 'Đã publish',
  published: 'Đã publish',
  extract_failed: 'Lỗi trích xuất',
  classification_low_confidence: 'Cần review phân loại',
  needs_review: 'Cần review',
  validation_failed: 'Không đạt validate',
  rejected: 'Đã từ chối',
  archived: 'Đã lưu trữ',
  queued: 'Đang chờ',
  pending: 'Đang chờ',
  running: 'Đang chạy',
  processing: 'Đang xử lý',
  failed: 'Thất bại',
  completed: 'Hoàn tất'
};

const STATE_LABELS: Record<StageState, string> = {
  done: 'Xong',
  active: 'Đang chạy',
  pending: 'Chờ',
  warning: 'Cần review',
  blocked: 'Lỗi'
};

const PIPELINE_SEVERITY_LABELS: Record<string, string> = {
  ok: 'OK',
  degraded: 'Degraded',
  critical: 'Critical'
};

const deriveTitleFromFileName = (fileName: string) =>
  fileName
    .replace(/\.[^/.]+$/, '')
    .replace(/[_-]+/g, ' ')
    .trim();

const formatCreatedAt = (value?: string) => {
  if (!value) return '-';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  return d.toLocaleString('vi-VN');
};

const formatFileSize = (bytes?: number) => {
  if (!bytes || bytes <= 0) return '-';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 102.4) / 10} KB`;
  return `${Math.round(bytes / 104857.6) / 10} MB`;
};

const formatConfidence = (value?: number) => {
  if (typeof value !== 'number') return '-';
  if (value >= 0 && value <= 1) return `${Math.round(value * 100)}%`;
  return value.toFixed(2);
};

const formatDurationSeconds = (seconds?: number) => {
  if (!seconds || seconds <= 0) return '-';
  const rounded = Math.round(seconds);
  const minutes = Math.floor(rounded / 60);
  const remainSeconds = rounded % 60;
  if (minutes < 1) return `${remainSeconds}s`;
  if (minutes < 60) return `${minutes}m ${remainSeconds}s`;
  const hours = Math.floor(minutes / 60);
  const remainMinutes = minutes % 60;
  return `${hours}h ${remainMinutes}m`;
};

const formatAgeSeconds = (seconds?: number) => {
  if (!seconds || seconds <= 0) return 'vừa xong';
  return `${formatDurationSeconds(seconds)} trước`;
};

const statusLabel = (status?: string) => (status ? STATUS_LABELS[status] || status : '-');

const stageLabel = (stage?: string) => {
  if (!stage) return '-';
  return STAGE_LABELS[stage as PipelineStageKey] || stage;
};

const stageStateClass = (state: StageState) => {
  if (state === 'done') return 'status-ok';
  if (state === 'active') return 'status-processing';
  if (state === 'warning') return 'status-warn';
  if (state === 'blocked') return 'status-warn pipeline-stage-blocked';
  return 'status-muted';
};

const statusClass = (status: string) => {
  if (status === 'ready' || status === 'published' || status === 'completed') return 'status-ok';
  if (TERMINAL_FAILED.has(status) || REVIEW_STATUSES.has(status)) return 'status-warn';
  if (PROCESSING_STATUSES.has(status) || status === 'queued' || status === 'running' || status === 'processing') {
    return 'status-processing';
  }
  if (status === 'archived') return 'status-muted';
  return 'status-info';
};

const pipelineSeverityClass = (severity?: string) => {
  if (severity === 'ok') return 'status-ok';
  if (severity === 'critical') return 'status-critical';
  if (severity === 'degraded') return 'status-warn';
  return 'status-muted';
};

const getAnalysisObject = (upload: DocumentUploadItem | undefined): Record<string, unknown> => {
  if (upload?.analysis && typeof upload.analysis === 'object' && !Array.isArray(upload.analysis)) return upload.analysis;
  return {};
};

const getUploadAnalysisValue = (upload: DocumentUploadItem | undefined, key: string) => {
  const analysisValue = getAnalysisObject(upload)[key];
  if (typeof analysisValue === 'string' || typeof analysisValue === 'number') return String(analysisValue);
  const metadataValue = upload?.metadata?.[key];
  if (typeof metadataValue === 'string' || typeof metadataValue === 'number') return String(metadataValue);
  return undefined;
};

const getUploadAnalysisNumber = (upload: DocumentUploadItem | undefined, key: string) => {
  const value = getUploadAnalysisValue(upload, key);
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isNaN(parsed) ? undefined : parsed;
};

const resolveUploadConfidence = (upload: DocumentUploadItem | undefined) =>
  upload?.confidence ?? getUploadAnalysisNumber(upload, 'confidence');

const resolveUploadDetectedType = (upload: DocumentUploadItem | undefined) =>
  upload?.detected_type ||
  upload?.detected_doc_type ||
  upload?.detected_doc_type_code ||
  getUploadAnalysisValue(upload, 'detected_type') ||
  getUploadAnalysisValue(upload, 'detected_doc_type') ||
  getUploadAnalysisValue(upload, 'detected_doc_type_code') ||
  '';

const resolveUploadDocumentNumber = (upload: DocumentUploadItem | undefined) =>
  getUploadAnalysisValue(upload, 'document_number') || getUploadAnalysisValue(upload, 'document_no') || '';

const getApprovalCandidates = (upload: DocumentUploadItem | undefined): ApprovalCandidate[] => {
  const candidates = getAnalysisObject(upload).candidates;
  if (!Array.isArray(candidates)) return [];
  return candidates
    .map((candidate): ApprovalCandidate | null => {
      if (!candidate || typeof candidate !== 'object') return null;
      const raw = candidate as Record<string, unknown>;
      const code = typeof raw.code === 'string' ? raw.code.trim() : '';
      if (!code) return null;
      const name = typeof raw.name === 'string' ? raw.name : undefined;
      const score = typeof raw.score === 'number' ? raw.score : undefined;
      return { code, name, score };
    })
    .filter((candidate): candidate is ApprovalCandidate => Boolean(candidate));
};

const formatUploadAnalysis = (upload: DocumentUploadItem | undefined) => {
  if (!upload?.analysis) return '';
  if (typeof upload.analysis === 'string') return upload.analysis;
  return JSON.stringify(upload.analysis, null, 2);
};

const getDocumentStatus = (doc: DocumentItem | undefined): DocumentLifecycleStatus =>
  doc?.status ||
  doc?.lifecycle_status ||
  doc?.review_status ||
  doc?.validation_status ||
  doc?.ingest_status ||
  doc?.latest_asset?.status ||
  (doc?.assets?.length ? 'ready' : 'needs_review');

const getDocumentDetectedType = (doc: DocumentItem | undefined) =>
  doc?.detected_type || doc?.detected_doc_type || doc?.detected_doc_type_code || doc?.doc_type_code || '';

const getActionEndpoint = (action: DocumentAdminActionDescriptor | undefined) => action?.href || action?.url;

const getDocumentAction = (doc: DocumentItem | undefined, name: DocumentAdminActionName) => {
  if (!doc) return undefined;
  const raw = doc.admin_actions?.[name] ?? doc.actions?.[name];
  if (raw && typeof raw === 'object') return raw;
  if (raw === true || doc.available_actions?.includes(name)) return { enabled: true };
  return undefined;
};

const getUploadAction = (upload: DocumentUploadItem | undefined, name: DocumentAdminActionName) => {
  if (!upload) return undefined;
  const raw = upload.admin_actions?.[name] ?? upload.actions?.[name];
  if (raw && typeof raw === 'object') return raw;
  if (raw === true || upload.available_actions?.includes(name)) return { enabled: true };
  return undefined;
};

const actionState = (action: DocumentAdminActionDescriptor | undefined) => {
  if (!action) return 'pending';
  if (action.enabled === false) return 'disabled';
  return getActionEndpoint(action) ? 'wired' : 'reported';
};

const renderActionIcon = (name: DocumentAdminActionName) => {
  if (name === 'approve') return <FactCheckIcon aria-hidden="true" />;
  if (name === 'reindex') return <SyncIcon aria-hidden="true" />;
  if (name === 'archive') return <StorageIcon aria-hidden="true" />;
  return <DeleteIcon aria-hidden="true" />;
};

const normalizePipelineEventStage = (event: DocumentUploadEvent): PipelineStageKey | undefined => {
  const stage = String(event.stage || '').trim().toLowerCase();
  if (stage === 'upload') return 'upload';
  if (stage === 'security' || stage === 'security_scan' || stage === 'av_scan') return 'security';
  if (stage === 'extract' || stage === 'extraction') return 'extract';
  if (stage === 'normalize_vi' || stage === 'vietnamese_normalize' || stage === 'normalizing_vi') return 'normalize_vi';
  if (stage === 'classify' || stage === 'classification') return 'classify';
  if (stage === 'profile' || stage === 'ingestion_profile') return 'profile';
  if (stage === 'document_version' || stage === 'document' || stage === 'version') return 'document_version';
  if (stage === 'chunk' || stage === 'chunking') return 'chunk';
  if (stage === 'embed_index' || stage === 'indexing' || stage === 'embedding') return 'embed_index';
  if (stage === 'validate' || stage === 'validation') return 'validate';
  if (stage === 'publish' || stage === 'ready') return 'publish';
  return undefined;
};

const pipelineEventState = (event: DocumentUploadEvent): StageState => {
  const status = String(event.status || '').trim().toLowerCase();
  if (['done', 'passed', 'completed', 'ready', 'published'].includes(status)) return 'done';
  if (['active', 'running', 'processing', 'queued', 'pending'].includes(status)) return 'active';
  if (['warning', 'needs_review', 'low_confidence', 'skipped'].includes(status)) return 'warning';
  if (['blocked', 'failed', 'error', 'unavailable', 'rejected'].includes(status)) return 'blocked';
  return 'pending';
};

const eventCreatedAtMs = (event: DocumentUploadEvent) => {
  if (!event.created_at) return 0;
  const parsed = new Date(event.created_at).getTime();
  return Number.isNaN(parsed) ? 0 : parsed;
};

const hasStageEventState = (upload: DocumentUploadItem, stage: PipelineStageKey, state: StageState) =>
  Boolean(upload.events?.some((event) => normalizePipelineEventStage(event) === stage && pipelineEventState(event) === state));

const applyPipelineEvents = (stages: PipelineStage[], upload: DocumentUploadItem | undefined) => {
  if (!upload?.events?.length) return stages;
  const next = stages.map((stage) => ({ ...stage }));
  const indexByStage = new Map<PipelineStageKey, number>();
  next.forEach((stage, index) => indexByStage.set(stage.key, index));

  [...upload.events]
    .sort((a, b) => eventCreatedAtMs(a) - eventCreatedAtMs(b))
    .forEach((event) => {
      const stageKey = normalizePipelineEventStage(event);
      if (!stageKey) return;
      const index = indexByStage.get(stageKey);
      if (index === undefined) return;
      next[index] = {
        ...next[index],
        state: pipelineEventState(event),
        detail: event.message || next[index].detail,
        event
      };
    });
  return next;
};

const buildPipelineStages = (upload: DocumentUploadItem | undefined, isUploading = false): PipelineStage[] => {
  const status = upload?.status || (isUploading ? 'security_scanning' : 'uploaded');
  const hasUploadRecord = Boolean(upload);
  const linkedDocument = Boolean(upload?.document_id && upload?.document_version_id);
  const failed = TERMINAL_FAILED.has(status);
  const review = REVIEW_STATUSES.has(status);
  const ready = status === 'ready' || status === 'published';

  const stage = (key: PipelineStageKey, state: StageState, detail?: string): PipelineStage => ({
    key,
    label: STAGE_LABELS[key],
    state,
    detail
  });

  if (isUploading && !hasUploadRecord) {
    return [
      stage('upload', 'done'),
      stage('security', 'active', 'Đang kiểm tra trước khi lưu'),
      stage('extract', 'pending'),
      stage('normalize_vi', 'pending'),
      stage('classify', 'pending'),
      stage('profile', 'pending'),
      stage('document_version', 'pending'),
      stage('chunk', 'pending'),
      stage('embed_index', 'pending'),
      stage('validate', 'pending'),
      stage('publish', 'pending')
    ];
  }

  const inferredStages = [
    stage('upload', hasUploadRecord ? 'done' : 'pending', hasUploadRecord ? 'File đã được nhận' : undefined),
    stage(
      'security',
      hasUploadRecord ? 'done' : status === 'security_failed' ? 'blocked' : 'pending',
      hasUploadRecord ? 'Đã qua AV scan trước khi lưu' : undefined
    ),
    stage(
      'extract',
      status === 'extracting'
        ? 'active'
        : status === 'extract_failed'
          ? 'blocked'
          : ['classification_low_confidence', 'classified', 'profile_resolved', 'indexing', 'validating', 'ready'].includes(status)
            ? 'done'
            : 'pending'
    ),
    stage(
      'normalize_vi',
      ['classification_low_confidence', 'classified', 'profile_resolved', 'indexing', 'validating', 'ready'].includes(status)
        ? 'done'
        : status === 'extract_failed'
          ? 'pending'
          : status === 'extracting'
            ? 'pending'
            : 'pending',
      ['classification_low_confidence', 'classified', 'profile_resolved', 'indexing', 'validating', 'ready'].includes(status)
        ? 'Đã chuẩn hóa tiếng Việt cho phân loại/tìm kiếm'
        : undefined
    ),
    stage(
      'classify',
      status === 'classification_low_confidence'
        ? 'warning'
        : ['classified', 'profile_resolved', 'indexing', 'validating', 'ready'].includes(status)
          ? 'done'
          : failed
            ? 'pending'
            : 'pending',
      review ? 'Confidence thấp, cần người duyệt' : undefined
    ),
    stage(
      'profile',
      linkedDocument || ['profile_resolved', 'indexing', 'validating', 'ready'].includes(status)
        ? 'done'
        : status === 'classification_low_confidence'
          ? 'warning'
          : 'pending',
      linkedDocument ? 'Đã resolve/create ingestion profile' : undefined
    ),
    stage('document_version', linkedDocument ? 'done' : review ? 'pending' : 'pending', upload?.document_version_id),
    stage(
      'chunk',
      ready || status === 'validating' ? 'done' : status === 'indexing' ? 'active' : failed ? 'pending' : 'pending'
    ),
    stage(
      'embed_index',
      ready || status === 'validating' ? 'done' : status === 'indexing' ? 'active' : failed ? 'pending' : 'pending'
    ),
    stage(
      'validate',
      ready ? 'done' : status === 'validating' ? 'active' : status === 'validation_failed' ? 'blocked' : 'pending'
    ),
    stage(
      'publish',
      ready ? 'done' : review ? 'warning' : status === 'validation_failed' ? 'blocked' : 'pending',
      ready ? 'Đã publish vào knowledge base' : review ? 'Chờ review trước khi publish' : undefined
    )
  ];
  return applyPipelineEvents(inferredStages, upload);
};

const DocumentsPage = () => {
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [uploads, setUploads] = useState<DocumentUploadItem[]>([]);
  const [jobs, setJobs] = useState<IngestJob[]>([]);
  const [pipelineHealth, setPipelineHealth] = useState<PipelineHealthResponse | null>(null);
  const [title, setTitle] = useState('');
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [fileInputKey, setFileInputKey] = useState(0);
  const [selectedUploadId, setSelectedUploadId] = useState<string | null>(null);
  const [selectedDocumentId, setSelectedDocumentId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [runningAction, setRunningAction] = useState<DocumentAdminActionName | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [approvalDocTypeCode, setApprovalDocTypeCode] = useState('');
  const [error, setError] = useState<string | null>(null);

  const fetchOperations = async (preferredUploadId?: string, preferredDocumentId?: string) => {
    setLoading(true);
    try {
      const [uploadData, documentData, jobData] = await Promise.all([
        listDocumentUploads(100),
        listDocuments(),
        listIngestJobs()
      ]);
      const healthData = await getPipelineHealth({ recent_hours: 24, stale_minutes: 15, limit: 20 }).catch((err) => {
        if (isEndpointUnavailableError(err)) return null;
        throw err;
      });
      setUploads(uploadData);
      setDocuments(documentData);
      setJobs(jobData);
      setPipelineHealth(healthData);
      setSelectedUploadId((prev) => {
        const candidate = preferredUploadId || prev;
        if (candidate && uploadData.some((item) => item.id === candidate)) return candidate;
        return uploadData[0]?.id || null;
      });
      setSelectedDocumentId((prev) => {
        const candidate = preferredDocumentId || prev;
        if (candidate && documentData.some((item) => item.id === candidate)) return candidate;
        return documentData[0]?.id || null;
      });
      setError(null);
    } catch (err) {
      if (isEndpointUnavailableError(err)) {
        setError('Luồng upload tự động chưa sẵn sàng trên backend hiện tại.');
      } else {
        setError(unwrapError(err));
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchOperations();
  }, []);

  const selectedUpload = uploads.find((item) => item.id === selectedUploadId);
  const selectedDocument =
    documents.find((item) => item.id === selectedUpload?.document_id) ||
    documents.find((item) => item.id === selectedDocumentId);
  const selectedStages = buildPipelineStages(selectedUpload, uploading);
  const selectedEvents = [...(selectedUpload?.events || [])].sort((a, b) => eventCreatedAtMs(a) - eventCreatedAtMs(b));
  const approvalCandidates = getApprovalCandidates(selectedUpload);

  useEffect(() => {
    setApprovalDocTypeCode(approvalCandidates[0]?.code || '');
  }, [selectedUploadId]);

  const counters = useMemo(() => {
    const failedUploads = uploads.filter((item) => TERMINAL_FAILED.has(item.status)).length;
    const reviewUploads = uploads.filter((item) => REVIEW_STATUSES.has(item.status)).length;
    const indexingUploads = uploads.filter((item) => item.status === 'indexing' || item.status === 'validating').length;
    const extractingUploads = uploads.filter((item) => item.status === 'extracting').length;
    const securityPassed = uploads.filter((item) => hasStageEventState(item, 'security', 'done')).length;
    const published = uploads.filter((item) => item.status === 'ready' || item.status === 'published' || hasStageEventState(item, 'publish', 'done')).length;

    return [
      { label: 'Đã upload', value: uploads.length, icon: UploadFileIcon, tone: 'info' },
      { label: 'Qua security', value: securityPassed, icon: HealthIcon, tone: 'ok' },
      { label: 'Đang extract', value: extractingUploads, icon: ScienceIcon, tone: 'processing' },
      { label: 'Đang index', value: indexingUploads, icon: SyncIcon, tone: 'processing' },
      { label: 'Cần review', value: reviewUploads, icon: VisibilityIcon, tone: 'warn' },
      { label: 'Đã publish', value: published, icon: FactCheckIcon, tone: 'ok' },
      { label: 'Failed', value: failedUploads, icon: DeleteIcon, tone: 'warn' },
      { label: 'Jobs', value: jobs.length, icon: QueueIcon, tone: 'info' }
    ];
  }, [jobs.length, uploads]);

  const healthCards = useMemo(() => {
    if (!pipelineHealth) return [];
    const summary = pipelineHealth.summary;
    return [
      { label: 'Stuck', value: summary.stale_uploads, icon: VisibilityIcon, tone: summary.stale_uploads > 0 ? 'warn' : 'ok' },
      { label: 'Review backlog', value: summary.review_uploads, icon: FactCheckIcon, tone: summary.review_uploads > 0 ? 'warn' : 'ok' },
      { label: 'Failed', value: summary.failed_uploads, icon: DeleteIcon, tone: summary.failed_uploads > 0 ? 'warn' : 'ok' },
      {
        label: 'Security',
        value: summary.security_blocked + summary.security_unavailable,
        icon: HealthIcon,
        tone: summary.security_blocked + summary.security_unavailable > 0 ? 'warn' : 'ok'
      },
      { label: 'Index backlog', value: summary.active_jobs, icon: QueueIcon, tone: summary.active_jobs > 0 ? 'processing' : 'ok' },
      {
        label: 'Avg publish',
        value: formatDurationSeconds(pipelineHealth.latency.average_seconds),
        icon: SyncIcon,
        tone: 'info'
      }
    ];
  }, [pipelineHealth]);

  const healthIssues = useMemo(() => {
    if (!pipelineHealth) return [];
    const byKey = new Map<string, PipelineHealthIssue>();
    [...pipelineHealth.stale_uploads, ...pipelineHealth.recent_issues].forEach((issue, index) => {
      const key = issue.upload_id || `${issue.file_name}:${issue.stage}:${issue.event_created_at || index}`;
      if (!byKey.has(key)) byKey.set(key, issue);
    });
    return Array.from(byKey.values()).slice(0, 6);
  }, [pipelineHealth]);

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const nextFile = event.target.files?.[0] || null;
    setUploadFile(nextFile);
    if (nextFile && !title.trim()) setTitle(deriveTitleFromFileName(nextFile.name));
  };

  const handleUpload = async () => {
    if (!uploadFile) return;
    setUploading(true);
    setActionMessage(null);
    try {
      const upload = await uploadDocumentOnly({ file: uploadFile, title: title.trim() || deriveTitleFromFileName(uploadFile.name) });
      setUploads((prev) => [upload, ...prev.filter((item) => item.id !== upload.id)]);
      setSelectedUploadId(upload.id);
      setUploadFile(null);
      setTitle('');
      setFileInputKey((prev) => prev + 1);
      setActionMessage('upload_created');
      setError(null);
      await fetchOperations(upload.id, upload.document_id);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setUploading(false);
    }
  };

  const runAction = async (
    source: 'upload' | 'document',
    name: DocumentAdminActionName,
    label: string,
    action?: DocumentAdminActionDescriptor
  ) => {
    const endpoint = getActionEndpoint(action);
    if (!action || !endpoint || action.enabled === false) return;
    if ((name === 'archive' || name === 'reject') && !window.confirm(`${label}?`)) return;
    setRunningAction(name);
    setActionMessage(null);
    try {
      await runDocumentAdminAction(action);
      await fetchOperations(selectedUpload?.id, selectedDocument?.id);
      setActionMessage(`${source}_${name}_submitted`);
      setError(null);
    } catch (err) {
      setError(unwrapError(err));
    } finally {
      setRunningAction(null);
    }
  };

  const renderStatusBadge = (status: string) => (
    <span className={`badge ${statusClass(status)}`.trim()}>{statusLabel(status)}</span>
  );

  return (
    <div className="pipeline-operations">
      <Panel
        title={
          <div className="pipeline-page-title">
            <span>Intake Pipeline</span>
            <span className="badge status-info">Upload-first</span>
          </div>
        }
      >
        <div className="pipeline-summary-grid">
          {counters.map((item) => {
            const Icon = item.icon;
            return (
              <div className={`pipeline-stat status-${item.tone}`} key={item.label}>
                <Icon aria-hidden="true" />
                <div>
                  <div className="pipeline-stat-value">{item.value}</div>
                  <div className="pipeline-stat-label">{item.label}</div>
                </div>
              </div>
            );
          })}
        </div>

        <div className="pipeline-health-panel">
          <div className="pipeline-section-header">
            <div>
              <div className="admin-list-item-title">Pipeline Health</div>
              <div className="admin-list-item-subtitle">
                Snapshot 24h gần nhất, stale threshold 15 phút.
              </div>
            </div>
            {pipelineHealth ? (
              <div className="pipeline-health-header-actions">
                <span className={`badge ${pipelineSeverityClass(pipelineHealth.severity)}`.trim()}>
                  {PIPELINE_SEVERITY_LABELS[pipelineHealth.severity] || pipelineHealth.severity}
                </span>
                <span className="badge">Cập nhật: {formatCreatedAt(pipelineHealth.generated_at)}</span>
              </div>
            ) : (
              <span className="badge status-muted">Chưa có snapshot</span>
            )}
          </div>
          {pipelineHealth ? (
            <>
              <div className="pipeline-health-grid">
                {healthCards.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div className={`pipeline-health-card status-${item.tone}`} key={item.label}>
                      <Icon aria-hidden="true" />
                      <div>
                        <div className="pipeline-health-value">{item.value}</div>
                        <div className="pipeline-health-label">{item.label}</div>
                      </div>
                    </div>
                  );
                })}
              </div>
              {pipelineHealth.alerts.length > 0 && (
                <div className="pipeline-alert-list">
                  {pipelineHealth.alerts.map((alert) => (
                    <div className="pipeline-alert-row" key={alert.code}>
                      <span className={`badge ${pipelineSeverityClass(alert.severity)}`.trim()}>
                        {PIPELINE_SEVERITY_LABELS[alert.severity] || alert.severity}
                      </span>
                      <div className="pipeline-issue-copy">
                        <div className="pipeline-event-message">{alert.message}</div>
                        <div className="pipeline-event-meta">
                          <span>{alert.code}</span>
                          <span>value: {alert.value}</span>
                          <span>threshold: {alert.threshold}</span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
              <div className="pipeline-health-issues">
                <div className="admin-list-item-title">Recent pipeline issues</div>
                {healthIssues.length === 0 ? (
                  <div className="badge status-ok">Không có issue trong window hiện tại.</div>
                ) : (
                  <div className="pipeline-issue-list">
                    {healthIssues.map((issue) => (
                      <button
                        className="pipeline-issue-row"
                        key={issue.upload_id || `${issue.file_name}:${issue.stage}:${issue.event_created_at || issue.age_seconds}`}
                        onClick={() => issue.upload_id && setSelectedUploadId(issue.upload_id)}
                        disabled={!issue.upload_id}
                      >
                        <span className={`badge ${statusClass(issue.status || issue.event_status)}`.trim()}>
                          {stageLabel(issue.stage)}
                        </span>
                        <div className="pipeline-issue-copy">
                          <div className="pipeline-event-message">{issue.title || issue.file_name || '-'}</div>
                          <div className="pipeline-event-meta">
                            <span>{statusLabel(issue.status || issue.event_status)}</span>
                            <span>{formatAgeSeconds(issue.age_seconds)}</span>
                            {issue.message && <span>{issue.message}</span>}
                          </div>
                        </div>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="badge status-muted">Backend health endpoint chưa trả dữ liệu.</div>
          )}
        </div>

        <div className="pipeline-upload-zone">
          <div className="pipeline-upload-copy">
            <div className="admin-list-item-title">Upload tài liệu</div>
            <div className="admin-list-item-subtitle">File sẽ đi qua security scan trước khi vào pipeline xử lý.</div>
          </div>
          <div className="pipeline-upload-controls">
            <input key={fileInputKey} type="file" onChange={handleFileChange} />
            <Input label="Tiêu đề" value={title} onChange={(event) => setTitle(event.target.value)} />
            <Button onClick={handleUpload} disabled={!uploadFile || uploading}>
              <UploadFileIcon aria-hidden="true" />
              {uploading ? 'Đang kiểm tra...' : 'Upload'}
            </Button>
          </div>
          {uploadFile && (
            <div className="documents-document-meta">
              <span className="badge">{uploadFile.name}</span>
              <span className="badge">{formatFileSize(uploadFile.size)}</span>
            </div>
          )}
        </div>

        {actionMessage === 'upload_created' && <div className="badge status-ok">Đã đưa file vào pipeline.</div>}
        {error && <div className="badge status-warn">Lỗi: {error}</div>}
        {loading && <div className="badge">Đang tải Operations...</div>}

        <div className="pipeline-main-grid">
          <section className="pipeline-queue-panel">
            <div className="pipeline-section-header">
              <div>
                <div className="admin-list-item-title">Pipeline queue</div>
                <div className="admin-list-item-subtitle">Theo dõi từng file từ upload tới publish.</div>
              </div>
              <Button variant="outline" onClick={() => void fetchOperations()} disabled={loading}>
                <SyncIcon aria-hidden="true" />
                Refresh
              </Button>
            </div>

            <div className="pipeline-upload-table">
              {uploads.length === 0 && !loading && <div className="badge status-muted">Chưa có file trong pipeline.</div>}
              {uploads.map((upload) => {
                const stages = buildPipelineStages(upload);
                const activeStage = stages.find((item) => item.state === 'active') || stages.find((item) => item.state === 'warning');
                const detectedType = resolveUploadDetectedType(upload);
                const documentNumber = resolveUploadDocumentNumber(upload);
                const confidence = resolveUploadConfidence(upload);
                return (
                  <button
                    className={`pipeline-row ${selectedUploadId === upload.id ? 'selected' : ''}`}
                    key={upload.id}
                    onClick={() => setSelectedUploadId(upload.id)}
                  >
                    <div className="pipeline-row-main">
                      <div className="pipeline-row-title">{upload.title || upload.file_name}</div>
                      <div className="pipeline-row-meta">
                        {renderStatusBadge(upload.status)}
                        <span className="badge">{activeStage ? activeStage.label : 'Hoàn tất'}</span>
                        <span className="badge">{formatFileSize(upload.file_size_bytes)}</span>
                        {detectedType && <span className="badge">Loại: {detectedType}</span>}
                        {documentNumber && <span className="badge">Số: {documentNumber}</span>}
                        {typeof confidence === 'number' && <span className="badge">Tin cậy: {formatConfidence(confidence)}</span>}
                      </div>
                    </div>
                    <div className="pipeline-mini-stages" aria-label="Pipeline stages">
                      {stages.map((stage) => (
                        <span className={`pipeline-mini-stage ${stage.state}`} key={stage.key} title={`${stage.label}: ${STATE_LABELS[stage.state]}`} />
                      ))}
                    </div>
                  </button>
                );
              })}
            </div>
          </section>

          <aside className="pipeline-detail-panel">
            <div className="pipeline-section-header">
              <div>
                <div className="admin-list-item-title">Pipeline detail</div>
                <div className="admin-list-item-subtitle">
                  {selectedUpload ? selectedUpload.file_name : 'Chọn một file để xem chi tiết.'}
                </div>
              </div>
              {selectedUpload && renderStatusBadge(selectedUpload.status)}
            </div>

            {selectedUpload ? (
              <div className="pipeline-detail-body">
                <div className="pipeline-stage-list">
                  {selectedStages.map((stage) => (
                    <div className={`pipeline-stage-card ${stage.state}`} key={stage.key}>
                      <div className="pipeline-stage-dot" aria-hidden="true" />
                      <div>
                        <div className="pipeline-stage-title">{stage.label}</div>
                        <div className={`badge ${stageStateClass(stage.state)}`.trim()}>{STATE_LABELS[stage.state]}</div>
                        {stage.detail && <div className="pipeline-stage-detail">{stage.detail}</div>}
                      </div>
                    </div>
                  ))}
                </div>

                <div className="pipeline-evidence-grid">
                  <span className="badge">SHA-256: {selectedUpload.sha256?.slice(0, 16) || '-'}</span>
                  <span className="badge">Content-Type: {selectedUpload.content_type || '-'}</span>
                  <span className="badge">Tạo lúc: {formatCreatedAt(selectedUpload.created_at)}</span>
                  <span className="badge">Cập nhật: {formatCreatedAt(selectedUpload.updated_at)}</span>
                  {selectedUpload.document_id && <span className="badge status-ok">Document: {selectedUpload.document_id}</span>}
                  {selectedUpload.document_version_id && (
                    <span className="badge status-ok">Version: {selectedUpload.document_version_id}</span>
                  )}
                  {selectedUpload.error_message && <span className="badge status-warn">{selectedUpload.error_message}</span>}
                </div>

                <div className="panel-section">
                  <div className="label">Audit timeline</div>
                  {selectedEvents.length ? (
                    <div className="pipeline-event-list">
                      {selectedEvents.map((event) => {
                        const state = pipelineEventState(event);
                        const stageKey = normalizePipelineEventStage(event);
                        return (
                          <div className={`pipeline-event-row ${state}`} key={event.id}>
                            <span className={`badge ${stageStateClass(state)}`.trim()}>
                              {stageKey ? STAGE_LABELS[stageKey] : event.stage || '-'}
                            </span>
                            <div className="pipeline-event-copy">
                              <div className="pipeline-event-message">{event.message || statusLabel(event.status)}</div>
                              <div className="pipeline-event-meta">
                                <span>{formatCreatedAt(event.created_at)}</span>
                                <span>{event.actor || 'system'}</span>
                                <span>{event.event_type}</span>
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <div className="badge status-muted">Backend chưa trả audit events cho upload này.</div>
                  )}
                </div>

                <div className="panel-section">
                  <div className="label">Phân tích pipeline</div>
                  {formatUploadAnalysis(selectedUpload) ? (
                    <pre className="documents-analysis">{formatUploadAnalysis(selectedUpload)}</pre>
                  ) : (
                    <div className="badge status-muted">Chưa có analysis.</div>
                  )}
                </div>

                <div className="panel-section">
                  <div className="label">Review actions</div>
                  {approvalCandidates.length > 0 && (
                    <label className="documents-review-select">
                      <span>Ingestion profile</span>
                      <select value={approvalDocTypeCode} onChange={(event) => setApprovalDocTypeCode(event.target.value)}>
                        {approvalCandidates.map((candidate) => (
                          <option key={candidate.code} value={candidate.code}>
                            {candidate.name ? `${candidate.name} (${candidate.code})` : candidate.code}
                            {typeof candidate.score === 'number' ? ` - ${candidate.score.toFixed(1)}` : ''}
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                  <div className="documents-action-grid">
                    {DOCUMENT_ACTIONS.map((item) => {
                      const action = getUploadAction(selectedUpload, item.name) || getDocumentAction(selectedDocument, item.name);
                      const actionForRun =
                        item.name === 'approve' && action
                          ? {
                              ...action,
                              payload: {
                                ...(action.payload || {}),
                                doc_type_code: approvalDocTypeCode
                              }
                            }
                          : action;
                      const endpoint = getActionEndpoint(action);
                      const isRunning = runningAction === item.name;
                      const missingApproveProfile = item.name === 'approve' && !approvalDocTypeCode;
                      return (
                        <Button
                          key={item.name}
                          variant="outline"
                          className="document-action-button"
                          onClick={() =>
                            void runAction(getUploadAction(selectedUpload, item.name) ? 'upload' : 'document', item.name, item.label, actionForRun)
                          }
                          disabled={!endpoint || action?.enabled === false || missingApproveProfile || runningAction !== null}
                          title={endpoint || action?.reason || (missingApproveProfile ? 'Chọn ingestion profile trước' : 'Backend chưa trả action endpoint')}
                        >
                          {renderActionIcon(item.name)}
                          {isRunning ? 'Đang gửi...' : item.label}
                        </Button>
                      );
                    })}
                  </div>
                  <div className="documents-action-hints">
                    {DOCUMENT_ACTIONS.map((item) => {
                      const action = getUploadAction(selectedUpload, item.name) || getDocumentAction(selectedDocument, item.name);
                      const state = actionState(action);
                      return (
                        <span className={`badge ${state === 'wired' ? 'status-info' : 'status-muted'}`.trim()} key={item.name}>
                          {item.name}: {state}
                        </span>
                      );
                    })}
                  </div>
                  {actionMessage?.endsWith('_submitted') && <div className="badge status-ok">{actionMessage}</div>}
                </div>
              </div>
            ) : (
              <div className="badge status-muted">Chưa có file được chọn.</div>
            )}
          </aside>
        </div>
      </Panel>

      <Panel title="Published Document Registry">
        <div className="pipeline-registry-grid">
          {documents.length === 0 && !loading && <div className="badge status-muted">Chưa có document đã tạo.</div>}
          {documents.map((doc) => {
            const status = getDocumentStatus(doc);
            return (
              <button
                className={`pipeline-document-card ${selectedDocument?.id === doc.id ? 'selected' : ''}`}
                key={doc.id}
                onClick={() => setSelectedDocumentId(doc.id)}
              >
                <div className="pipeline-document-icon">
                  <ArticleIcon aria-hidden="true" />
                </div>
                <div>
                  <div className="pipeline-row-title">{doc.title}</div>
                  <div className="pipeline-row-meta">
                    {renderStatusBadge(status)}
                    <span className="badge">Profile: {doc.doc_type_code || '-'}</span>
                    <span className="badge">Loại: {getDocumentDetectedType(doc) || '-'}</span>
                    <span className="badge">Assets: {doc.assets?.length || 0}</span>
                  </div>
                </div>
              </button>
            );
          })}
        </div>
      </Panel>

      <Panel title="Ingest Job Monitor">
        <div className="pipeline-jobs-grid">
          {jobs.length === 0 && !loading && <div className="badge status-muted">Không có ingest job.</div>}
          {jobs.slice(0, 8).map((job) => (
            <div className="pipeline-job-row" key={job.id}>
              <span className={`badge ${statusClass(job.status)}`.trim()}>{statusLabel(job.status)}</span>
              <span className="pipeline-job-id">{job.id}</span>
              <span className="badge">Version: {job.document_version_id}</span>
              {job.error_message && <span className="badge status-warn">{job.error_message}</span>}
            </div>
          ))}
        </div>
      </Panel>
    </div>
  );
};

export default DocumentsPage;
