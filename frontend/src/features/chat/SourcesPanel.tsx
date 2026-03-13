import type { Citation } from '@/core/types';

const SourcesPanel = ({ citations }: { citations: Citation[] }) => {
  if (!citations.length) {
    return <div className="badge">Không có trích dẫn nào.</div>;
  }

  return (
    <div className="grid">
      {citations.map((citation, index) => (
        <div className="source-item" key={citation.id || citation.chunk_id || index}>
          <div className="source-item-title">
            {citation.citation_label || citation.document_title || `Nguồn ${index + 1}`}
          </div>
          {citation.law_name && <div className="badge">{citation.law_name}</div>}
          {citation.chapter && <div className="badge">Chương: {citation.chapter}</div>}
          {citation.document_number && <div className="badge">So: {citation.document_number}</div>}
          {citation.clause && <div className="badge">Khoan: {citation.clause}</div>}
          {citation.chunk_id && <div className="badge">Chunk: {citation.chunk_id}</div>}
          {(citation.file_url || citation.url) && (
            <a href={citation.file_url || citation.url} target="_blank" rel="noreferrer">
              Download original document
            </a>
          )}
          {citation.excerpt && <p>{citation.excerpt}</p>}
        </div>
      ))}
    </div>
  );
};

export default SourcesPanel;
