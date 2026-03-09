import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { ChatMessage as ChatMessageType, Citation } from '@/core/types';

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
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{message.content || '...'}</ReactMarkdown>
        {message.citations && message.citations.length > 0 && (
          <div style={{ marginTop: 8, display: 'flex', gap: 6, flexWrap: 'wrap' }}>
            {message.citations.map((citation, index) => (
              <button
                key={citation.id || index}
                className="button outline"
                onClick={() => onSelectCitations(message.citations || [])}
              >
                [{index + 1}]
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ChatMessage;
