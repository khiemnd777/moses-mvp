import type { Citation } from '@/core/types';

export type CitationBadge = {
  label: string;
  tone?: 'active' | 'warning' | 'muted';
};

export type CitationGroup = {
  key: string;
  title: string;
  subtitle: string;
  citations: Citation[];
};

const compact = (value?: string | number) => String(value ?? '').trim();

export const getCitationKey = (citation: Citation, index = 0) => {
  return compact(citation.chunk_id) || compact(citation.id) || compact(citation.citation_label) || `citation-${index}`;
};

export const uniqueCitations = (citations: Citation[]) => {
  const seen = new Set<string>();
  const out: Citation[] = [];
  for (const citation of citations) {
    const key = getCitationKey(citation, out.length);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(citation);
  }
  return out;
};

export const getDocumentName = (citation: Citation) => {
  return (
    compact(citation.law_name) ||
    compact(citation.document_title) ||
    (compact(citation.document_number) ? `Văn bản ${citation.document_number}` : '')
  );
};

export const getArticleLabel = (citation: Citation) => {
  const article = compact(citation.article);
  const clause = compact(citation.clause);
  if (article && clause) return `Điều ${article}, khoản ${clause}`;
  if (article) return `Điều ${article}`;
  if (clause) return `Khoản ${clause}`;
  return '';
};

export const getCitationTitle = (citation: Citation, index = 0) => {
  const article = getArticleLabel(citation);
  const documentName = getDocumentName(citation);
  if (article && documentName) return `${article} - ${documentName}`;
  return compact(citation.citation_label) || article || documentName || `Nguồn ${index + 1}`;
};

export const getCitationChipLabel = (citation: Citation, index = 0) => {
  const article = getArticleLabel(citation);
  const documentNumber = compact(citation.document_number);
  if (article && documentNumber) return `${article} - ${documentNumber}`;
  return article || compact(citation.citation_label) || getDocumentName(citation) || `Nguồn ${index + 1}`;
};

export const getCitationSubtitle = (citation: Citation) => {
  const parts = [
    compact(citation.document_type).toUpperCase(),
    compact(citation.document_number),
    compact(citation.issuing_authority),
    citation.year > 0 ? String(citation.year) : ''
  ].filter(Boolean);
  return parts.join(' • ');
};

export const getStatusLabel = (status?: string) => {
  switch (compact(status).toLowerCase()) {
    case 'active':
      return 'Còn hiệu lực';
    case 'archived':
    case 'expired':
      return 'Hết hiệu lực';
    case 'pending':
      return 'Chưa có hiệu lực';
    default:
      return compact(status);
  }
};

export const getStatusTone = (status?: string): CitationBadge['tone'] => {
  switch (compact(status).toLowerCase()) {
    case 'active':
      return 'active';
    case 'archived':
    case 'expired':
    case 'pending':
      return 'warning';
    default:
      return 'muted';
  }
};

export const getScoreLabel = (score?: number) => {
  if (!Number.isFinite(score) || !score || score <= 0) return '';
  if (score <= 1) return `${Math.round(score * 100)}% khớp`;
  return `${score.toFixed(2)} điểm`;
};

export const getCitationBadges = (citation: Citation): CitationBadge[] => {
  const badges: CitationBadge[] = [];
  if (citation.source_rank) badges.push({ label: `#${citation.source_rank}`, tone: 'muted' });
  const status = getStatusLabel(citation.effective_status);
  if (status) badges.push({ label: status, tone: getStatusTone(citation.effective_status) });
  const score = getScoreLabel(citation.score);
  if (score) badges.push({ label: score, tone: 'active' });
  if (citation.document_number) badges.push({ label: `Số ${citation.document_number}`, tone: 'muted' });
  if (citation.chapter) badges.push({ label: `Chương ${citation.chapter}`, tone: 'muted' });
  const article = getArticleLabel(citation);
  if (article) badges.push({ label: article, tone: 'muted' });
  if (citation.issuing_authority) badges.push({ label: citation.issuing_authority, tone: 'muted' });
  return badges;
};

export const groupCitationsByDocument = (citations: Citation[]): CitationGroup[] => {
  const groups = new Map<string, CitationGroup>();
  for (const citation of uniqueCitations(citations)) {
    const documentName = getDocumentName(citation);
    const key = documentName || compact(citation.document_number) || compact(citation.asset_id) || 'unknown';
    const group = groups.get(key) || {
      key,
      title: documentName || 'Nguồn pháp lý',
      subtitle: getCitationSubtitle(citation),
      citations: []
    };
    group.citations.push(citation);
    if (!group.subtitle) group.subtitle = getCitationSubtitle(citation);
    groups.set(key, group);
  }
  return Array.from(groups.values());
};

export const trimExcerpt = (value?: string, limit = 520) => {
  const text = compact(value).replace(/\s+/g, ' ');
  if (!text || text.length <= limit) return text;
  return `${text.slice(0, limit - 3).trim()}...`;
};
