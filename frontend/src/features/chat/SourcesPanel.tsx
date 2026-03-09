import type { Citation } from '@/core/types';

const SourcesPanel = ({ citations }: { citations: Citation[] }) => {
  if (!citations.length) {
    return <div className="badge">Không có trích dẫn nào.</div>;
  }

  return (
    <div className="grid">
      {citations.map((citation, index) => (
        <div className="source-item" key={citation.id || index}>
          <div style={{ fontWeight: 600 }}>
            {citation.article
              ? `Dieu ${citation.article} ${citation.document_title} ${citation.year || ''}`.trim()
              : citation.document_title || `Source ${index + 1}`}
          </div>
          {citation.document_number && <div className="badge">So: {citation.document_number}</div>}
          {citation.clause && <div className="badge">Khoan: {citation.clause}</div>}
          {citation.url && (
            <a href={citation.url} target="_blank" rel="noreferrer">
              {citation.url}
            </a>
          )}
          {citation.excerpt && <p>{citation.excerpt}</p>}
        </div>
      ))}
    </div>
  );
};

export default SourcesPanel;
