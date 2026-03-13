import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage as ChatMessageType, Citation } from '@/core/types';

const formatTimestamp = (value: string) =>
  new Intl.DateTimeFormat('vi-VN', {
    hour: '2-digit',
    minute: '2-digit',
    day: '2-digit',
    month: '2-digit'
  }).format(new Date(value));

const ChatMessage = ({
  message,
  onSelectCitations
}: {
  message: ChatMessageType;
  onSelectCitations: (citations: Citation[]) => void;
}) => {
  return (
    <div className={`message ${message.role}`}>
      <div className="bubble">
        <div className="message-meta">
          <span>{message.role === 'user' ? 'Bạn' : 'Trợ lý pháp lý'}</span>
          <span>{formatTimestamp(message.created_at)}</span>
        </div>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.content || '...'}</ReactMarkdown>
        {message.citations.length > 0 && (
          <div className="citation-chips">
            {message.citations.map((citation, index) => (
              <button
                key={citation.id || citation.chunk_id || index}
                className="button outline"
                onClick={() => onSelectCitations(message.citations)}
              >
                [{index + 1}] {citation.article ? `Điều ${citation.article}` : citation.document_title}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ChatMessage;
