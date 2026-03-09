import type { Citation } from '@/core/types';

const SourcesPanel = ({ citations }: { citations: Citation[] }) => {
  if (!citations.length) {
    return <div className="badge">Không có trích dẫn nào.</div>;
  }

  return (
    <div className="grid">
      {citations.map((citation, index) => (
        <div className="source-item" key={citation.id || index}>
          <div style={{ fontWeight: 600 }}>{citation.title || `Source ${index + 1}`}</div>
          {citation.url && (
            <a href={citation.url} target="_blank" rel="noreferrer">
              {citation.url}
            </a>
          )}
          {citation.excerpt && <p>{citation.excerpt}</p>}
          {citation.score !== undefined && <div className="badge">Score: {citation.score}</div>}
        </div>
      ))}
    </div>
  );
};

export default SourcesPanel;
