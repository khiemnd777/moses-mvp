import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import guideMarkdown from './how-to-rag.md?raw';

const GuidePreviewPage = () => {
  return (
    <section className="guide-page">
      <div className="guide-hero card">
        <div className="panel-body guide-hero-body">
          <div className="badge">Markdown Preview</div>
          <h1>Hướng dẫn RAG</h1>
          <p>
            Nội dung được đọc trực tiếp từ file <code>how-to-rag.md</code> trong frontend workspace.
          </p>
        </div>
      </div>

      <article className="card guide-markdown-shell">
        <div className="guide-markdown">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{guideMarkdown}</ReactMarkdown>
        </div>
      </article>
    </section>
  );
};

export default GuidePreviewPage;
