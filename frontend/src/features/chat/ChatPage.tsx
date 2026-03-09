import { useMemo, useState } from 'react';
import { useChatStore } from './chatStore';
import ChatMessage from './ChatMessage';
import ChatInput from './ChatInput';
import FiltersBar from './FiltersBar';
import SourcesPanel from './SourcesPanel';
import Panel from '@/shared/Panel';
import type { Citation } from '@/core/types';

const ChatPage = () => {
  const { messages, isStreaming, error } = useChatStore();
  const [showSources, setShowSources] = useState(true);
  const [selectedCitations, setSelectedCitations] = useState<Citation[]>([]);

  const latestAssistantCitations = useMemo(() => {
    const last = [...messages].reverse().find((msg) => msg.role === 'assistant');
    return last?.citations || [];
  }, [messages]);

  return (
    <div className="chat-shell">
      <Panel className="chat-panel" title="Playground">
        <FiltersBar />
        <div className="chat-log">
          {messages.map((message) => (
            <ChatMessage
              key={message.id}
              message={message}
              onSelectCitations={(citations) => setSelectedCitations(citations)}
            />
          ))}
          {error && <div className="badge">{error}</div>}
        </div>
        <ChatInput />
      </Panel>
      <Panel
        className="source-panel"
        title={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>Trích dẫn nguồn</span>
            <button className="button outline" onClick={() => setShowSources((prev) => !prev)}>
              {showSources ? 'Thu nhỏ' : 'Mở rộng'}
            </button>
          </div>
        }
      >
        {showSources ? (
          <SourcesPanel citations={selectedCitations.length ? selectedCitations : latestAssistantCitations} />
        ) : (
          <div className="badge">Trích dẫn nguồn [thu nhỏ]</div>
        )}
      </Panel>
      {isStreaming && <div className="badge">Streaming response…</div>}
    </div>
  );
};

export default ChatPage;
