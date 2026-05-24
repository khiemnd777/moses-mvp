import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage as ChatMessageType, Citation } from '@/core/types';
import { getCitationChipLabel, getCitationKey, getCitationSubtitle, getCitationTitle, uniqueCitations } from './citationUtils';
import { ArticleIcon } from '@/shared/muiIcons';

const formatTimestamp = (value: string) =>
  new Intl.DateTimeFormat('vi-VN', {
    hour: '2-digit',
    minute: '2-digit',
    day: '2-digit',
    month: '2-digit'
  }).format(new Date(value));

const ChatMessage = ({
  message,
  onOpenCitation
}: {
  message: ChatMessageType;
  onOpenCitation: (citation: Citation, citations: Citation[]) => void;
}) => {
  const citations = uniqueCitations(message.citations);

  return (
    <div className={`message ${message.role}`}>
      <div className="bubble">
        <div className="message-meta">
          <span>{message.role === 'user' ? 'Bạn' : 'Trợ lý pháp lý'}</span>
          <span>{formatTimestamp(message.created_at)}</span>
        </div>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.content || '...'}</ReactMarkdown>
        {citations.length > 0 && (
          <div className="citation-chips">
            {citations.map((citation, index) => (
              <button
                key={getCitationKey(citation, index)}
                className="button outline citation-chip"
                onClick={() => onOpenCitation(citation, citations)}
                title={getCitationTitle(citation, index)}
                type="button"
              >
                <ArticleIcon aria-hidden="true" />
                <span className="citation-chip-index">[{index + 1}]</span>
                <span className="citation-chip-main">{getCitationChipLabel(citation, index)}</span>
                {getCitationSubtitle(citation) && <span className="citation-chip-subtitle">{getCitationSubtitle(citation)}</span>}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ChatMessage;
