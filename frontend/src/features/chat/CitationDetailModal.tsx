import { useEffect } from 'react';
import Button from '@/shared/Button';
import type { Citation, CitationDetail } from '@/core/types';
import { getCitationBadges, getCitationSubtitle, getCitationTitle } from './citationUtils';

const CitationDetailModal = ({
  citation,
  detail,
  isLoading,
  error,
  onClose,
  onDownload
}: {
  citation: Citation;
  detail?: CitationDetail;
  isLoading: boolean;
  error?: string;
  onClose: () => void;
  onDownload: () => void;
}) => {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const resolved = detail?.citation || citation;
  const title = getCitationTitle(resolved);
  const subtitle = getCitationSubtitle(resolved);

  return (
    <div
      aria-modal="true"
      className="citation-modal-overlay"
      role="dialog"
      onClick={onClose}
    >
      <div className="citation-modal card" onClick={(event) => event.stopPropagation()}>
        <div className="citation-modal-header">
          <div>
            <div className="citation-modal-eyebrow">Chi tiết dẫn chứng</div>
            <h3>{title}</h3>
            {subtitle && <div className="citation-modal-subtitle">{subtitle}</div>}
          </div>
          <button aria-label="Đóng popup" className="button outline citation-modal-close" onClick={onClose} type="button">
            Đóng
          </button>
        </div>
        <div className="citation-modal-meta">
          {getCitationBadges(resolved).map((badge) => (
            <span className={`badge citation-badge ${badge.tone ? `citation-badge-${badge.tone}` : ''}`.trim()} key={badge.label}>
              {badge.label}
            </span>
          ))}
          {detail?.file_name && <span className="badge">{detail.file_name}</span>}
        </div>
        <div className="citation-modal-actions">
          <Button onClick={onDownload} type="button" variant="secondary">
            Tải tài liệu
          </Button>
          <Button onClick={onClose} type="button" variant="outline">
            Đóng
          </Button>
        </div>
        <div className="citation-modal-content">
          {isLoading && <div className="badge">Đang tải nội dung dẫn chứng...</div>}
          {!isLoading && error && <div className="badge">{error}</div>}
          {!isLoading && !error && (
            <pre className="citation-detail-content">{detail?.content || citation.excerpt || 'Không có nội dung chi tiết.'}</pre>
          )}
        </div>
      </div>
    </div>
  );
};

export default CitationDetailModal;
