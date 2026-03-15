import { useEffect, useMemo, useState } from 'react';
import Panel from '@/shared/Panel';
import ChatInput from './ChatInput';
import ChatMessage from './ChatMessage';
import ConversationSidebar from './ConversationSidebar';
import FiltersBar from './FiltersBar';
import SourcesPanel from './SourcesPanel';
import { useChatStore } from './chatStore';
import type { Citation } from '@/core/types';

const ChatPage = () => {
  const { hydrate, currentConversationId, messagesByConversation, isStreaming, isLoading, error } = useChatStore();
  const [selectedCitations, setSelectedCitations] = useState<Citation[]>([]);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  const messages = currentConversationId ? messagesByConversation[currentConversationId] || [] : [];

  const latestAssistantCitations = useMemo(() => {
    const lastAssistant = [...messages].reverse().find((message) => message.role === 'assistant');
    return lastAssistant?.citations || [];
  }, [messages]);

  return (
    <div className="chat-layout">
      <ConversationSidebar />
      <Panel bodyClassName="chat-main-panel-body" className="chat-main-panel" title="Trợ lý pháp lý">
        <div className="chat-main">
          <div className="chat-column">
            <FiltersBar />
            <div className="chat-log">
              {isLoading && <div className="badge">Đang tải hội thoại...</div>}
              {!isLoading && messages.length === 0 && (
                <div className="empty-chat-state">
                  <h3>Bắt đầu cuộc hội thoại pháp lý</h3>
                  <p>Đặt câu hỏi, hệ thống sẽ truy xuất nguồn luật, giữ lịch sử chat và hiển thị trích dẫn gốc.</p>
                </div>
              )}
              {messages.map((message) => (
                <ChatMessage
                  key={message.message_id}
                  message={message}
                  onSelectCitations={(citations) => setSelectedCitations(citations)}
                />
              ))}
              {error && <div className="badge">{error}</div>}
            </div>
            <ChatInput />
          </div>
          <Panel bodyClassName="source-panel-content" className="source-panel" title="Nguồn pháp lý">
            <div className="source-panel-body">
              <SourcesPanel citations={selectedCitations.length > 0 ? selectedCitations : latestAssistantCitations} />
            </div>
          </Panel>
        </div>
        {isStreaming && <div className="chat-stream-indicator">Đang nhận phản hồi trực tuyến...</div>}
      </Panel>
    </div>
  );
};

export default ChatPage;
