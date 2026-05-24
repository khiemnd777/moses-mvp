import type { Citation } from '@/core/types';
import {
  getCitationBadges,
  getCitationKey,
  getCitationTitle,
  groupCitationsByDocument,
  trimExcerpt,
  uniqueCitations
} from './citationUtils';

const SourcesPanel = ({
  citations,
  onDownload,
  onOpen
}: {
  citations: Citation[];
  onDownload: (citation: Citation) => void;
  onOpen: (citation: Citation, citations: Citation[]) => void;
}) => {
  const normalizedCitations = uniqueCitations(citations);
  const groups = groupCitationsByDocument(normalizedCitations);

  if (!normalizedCitations.length) {
    return <div className="badge">Không có trích dẫn nào.</div>;
  }

  return (
    <div className="source-groups">
      {groups.map((group) => (
        <div className="source-group" key={group.key}>
          <div className="source-group-header">
            <div>
              <div className="source-item-title">{group.title}</div>
              {group.subtitle && <div className="source-item-subtitle">{group.subtitle}</div>}
            </div>
            <span className="badge">{group.citations.length} dẫn chứng</span>
          </div>
          <div className="grid">
            {group.citations.map((citation, index) => (
              <div className="source-item" key={getCitationKey(citation, index)}>
                <div className="source-item-title">{getCitationTitle(citation, index)}</div>
                <div className="citation-badge-row">
                  {getCitationBadges(citation).map((badge) => (
                    <span className={`badge citation-badge ${badge.tone ? `citation-badge-${badge.tone}` : ''}`.trim()} key={badge.label}>
                      {badge.label}
                    </span>
                  ))}
                </div>
                <div className="source-actions">
                  <button className="button secondary" onClick={() => onOpen(citation, normalizedCitations)} type="button">
                    Xem nội dung
                  </button>
                  {(citation.file_url || citation.url) && (
                    <button className="button outline" onClick={() => onDownload(citation)} type="button">
                      Tải văn bản
                    </button>
                  )}
                </div>
                {citation.excerpt && <p className="source-excerpt">{trimExcerpt(citation.excerpt)}</p>}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
};

export default SourcesPanel;
