import { type ReactNode, useEffect, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import Button from '@/shared/Button';
import chatgptSetupMarkdown from './chatgpt-setup.md?raw';
import ragGuideMarkdown from './how-to-rag.md?raw';
import chatgptLegalRagInstructionMarkdown from '../../../../docs/chatgpt_legal_rag_instruction.md?raw';
import doctypeGeneratorSpecMarkdown from '../../../../docs/doctype_generator_spec.md?raw';

type GuideItem = {
  id: string;
  title: string;
  markdown?: string;
  content?: ReactNode;
};

type GuideSource = {
  id: string;
  title: string;
  markdown: string;
};

const DOCUMENT_INGEST_STEPS = [
  { step: '1', lines: ['Upload tài liệu `.docx` lên ChatGPT, và prompt `Phân tích file thành DocType và Document`'] },
  { step: '2', lines: ['Mở tab `Tuning`'] },
  { step: '3', lines: ['Mở `Doc Types`'] },
  {
    step: '4',
    lines: [
      'Copy giá trị `code` vào `Doc Type Code`',
      'Copy giá trị `name` vào `Doc Type Name`',
      'Sau đó bấm `Create` để tạo mới Doc Type',
    ],
  },
  {
    step: '5',
    lines: [
      'Copy toàn bộ giá trị JSON của Doc Type mà ChatGPT đã phân tích',
      'Paste vào field `Doc Type Form (JSON)`',
    ],
  },
  { step: '6', lines: ['Bấm `Save` để hoàn thành việc tạo `Doc Type`'] },
  { step: '7', lines: ['Mở `Documents`'] },
  { step: '8', lines: ['Copy giá trị `title` vào `Title`', 'Copy giá trị `doc_type_code` vào `Doc Type Code`'] },
  { step: '9', lines: ['Bấm `Create Document`'] },
  { step: '10', lines: ['Chọn file `.docx` tương ứng, là file đã upload bên ChatGPT, để attach vào `Document Actions`'] },
  { step: '11', lines: ['Bấm `Upload Asset`'] },
  { step: '12', lines: ['Bấm `Create Version` để tạo phiên bản cho chỉ mục (`index`)'] },
  { step: '13', lines: ['Bấm `Enqueue Ingest Job` để nạp dữ liệu vào cơ chế Ingest'] },
  { step: '14', lines: ['Theo dõi trạng thái tại `Ingest Jobs`. Nếu thấy `completed` thì quy trình nạp dữ liệu thành công'] },
];

const DocumentIngestStepsContent = () => (
  <div className="guide-markdown">
    <h1>Các bước nạp tài liệu</h1>
    <p>
      Sau khi đã cài đặt xong ChatGPT đóng vai trò <code>Phân tích</code> và RAG Tool đóng vai trò <code>Ingest</code>
    </p>
    <table>
      <thead>
        <tr>
          <th>Bước</th>
          <th>Thao tác</th>
        </tr>
      </thead>
      <tbody>
        {DOCUMENT_INGEST_STEPS.map((item) => (
          <tr key={item.step}>
            <td>{item.step}</td>
            <td>
              <div className="guide-step-lines">
                {item.lines.map((line, index) => (
                  <ReactMarkdown key={`${item.step}-${index}`} remarkPlugins={[remarkGfm]}>
                    {line}
                  </ReactMarkdown>
                ))}
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  </div>
);

const GuidePreviewPage = () => {
  const guideItems = useMemo<GuideItem[]>(
    () => [
      {
        id: 'rag-guide',
        title: 'Tổng quan về luồng RAG',
        markdown: ragGuideMarkdown,
      },
      {
        id: 'chatgpt-setup',
        title: 'Hướng dẫn cài đặt ChatGPT',
        markdown: chatgptSetupMarkdown,
      },
      {
        id: 'document-ingest-steps',
        title: 'Các bước nạp tài liệu',
        content: <DocumentIngestStepsContent />,
      },
    ],
    [],
  );
  const guideSources = useMemo<Record<string, GuideSource>>(
    () => ({
      chatgpt_legal_rag_instruction: {
        id: 'chatgpt_legal_rag_instruction',
        title: 'chatgpt_legal_rag_instruction.md',
        markdown: chatgptLegalRagInstructionMarkdown,
      },
      doctype_generator_spec: {
        id: 'doctype_generator_spec',
        title: 'doctype_generator_spec.md',
        markdown: doctypeGeneratorSpecMarkdown,
      },
    }),
    [],
  );
  const [selectedGuideId, setSelectedGuideId] = useState(guideItems[0]?.id ?? '');
  const [activeSourceId, setActiveSourceId] = useState<string | null>(null);
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'error'>('idle');
  const selectedGuide = guideItems.find((item) => item.id === selectedGuideId) ?? guideItems[0];
  const activeSource = activeSourceId ? guideSources[activeSourceId] : null;

  useEffect(() => {
    if (!activeSourceId) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setActiveSourceId(null);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [activeSourceId]);

  useEffect(() => {
    if (copyState !== 'copied') {
      return;
    }

    const timeoutId = window.setTimeout(() => setCopyState('idle'), 1800);
    return () => window.clearTimeout(timeoutId);
  }, [copyState]);

  const handleCopySource = async () => {
    if (!activeSource) {
      return;
    }

    try {
      await navigator.clipboard.writeText(activeSource.markdown);
      setCopyState('copied');
    } catch {
      setCopyState('error');
    }
  };

  return (
    <section className="guide-page">
      <aside className="card guide-sidebar">
        <div className="guide-sidebar-header">
          <h2>Tài liệu</h2>
        </div>
        <nav className="guide-sidebar-nav" aria-label="Document navigation">
          {guideItems.map((item) => {
            const isActive = item.id === selectedGuide?.id;

            return (
              <button
                key={item.id}
                type="button"
                className={`guide-sidebar-item${isActive ? ' active' : ''}`}
                onClick={() => setSelectedGuideId(item.id)}
              >
                {item.title}
              </button>
            );
          })}
        </nav>
      </aside>
      <article className="card guide-markdown-shell">
        {selectedGuide?.content ?? (
          <div className="guide-markdown">
            <ReactMarkdown
              components={{
                a: ({ href, children, ...props }) => {
                  if (href?.startsWith('guide-source://')) {
                    const sourceId = href.replace('guide-source://', '');
                    const source = guideSources[sourceId];

                    if (source) {
                      return (
                        <button
                          type="button"
                          className="guide-inline-link"
                          onClick={() => {
                            setCopyState('idle');
                            setActiveSourceId(source.id);
                          }}
                        >
                          {children}
                        </button>
                      );
                    }
                  }

                  return (
                    <a href={href} {...props}>
                      {children}
                    </a>
                  );
                },
              }}
              remarkPlugins={[remarkGfm]}
              urlTransform={(url) => url}
            >
              {selectedGuide?.markdown ?? ''}
            </ReactMarkdown>
          </div>
        )}
      </article>
      {activeSource && (
        <div
          aria-modal="true"
          className="citation-modal-overlay"
          role="dialog"
          onClick={() => setActiveSourceId(null)}
        >
          <div className="citation-modal card guide-source-dialog" onClick={(event) => event.stopPropagation()}>
            <div className="citation-modal-header">
              <div>
                <div className="citation-modal-eyebrow">Source Markdown</div>
                <h3>{activeSource.title}</h3>
              </div>
              <button
                aria-label="Đóng popup"
                className="button outline citation-modal-close"
                onClick={() => setActiveSourceId(null)}
                type="button"
              >
                Đóng
              </button>
            </div>
            <div className="citation-modal-actions">
              <Button onClick={handleCopySource} type="button" variant="secondary">
                {copyState === 'copied' ? 'Đã copy' : 'Copy'}
              </Button>
              {copyState === 'error' && <span className="badge">Không thể copy nội dung.</span>}
            </div>
            <div className="citation-modal-content guide-source-dialog-content">
              <div className="guide-markdown guide-source-markdown">
                <ReactMarkdown remarkPlugins={[remarkGfm]} urlTransform={(url) => url}>
                  {activeSource.markdown}
                </ReactMarkdown>
              </div>
            </div>
          </div>
        </div>
      )}
    </section>
  );
};

export default GuidePreviewPage;
