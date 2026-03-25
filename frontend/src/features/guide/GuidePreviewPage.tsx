import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import guideMarkdown from './how-to-rag.md?raw';

const GuidePreviewPage = () => {
  return (
    <section className="guide-page">
      <article className="card guide-markdown-shell">
        <div className="guide-markdown">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{guideMarkdown}</ReactMarkdown>
        </div>
      </article>
    </section>
  );
};

export default GuidePreviewPage;
