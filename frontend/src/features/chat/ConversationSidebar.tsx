import Button from '@/shared/Button';
import { useChatStore } from './chatStore';

const formatTimestamp = (value?: string) => {
  if (!value) return '';
  const date = new Date(value);
  return new Intl.DateTimeFormat('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date);
};

const ConversationSidebar = ({
  isCollapsed = false,
  onToggleCollapse
}: {
  isCollapsed?: boolean;
  onToggleCollapse?: () => void;
}) => {
  const { conversations, currentConversationId, selectConversation, createConversation, deleteConversation, isLoading } =
    useChatStore();

  return (
    <aside className={`chat-sidebar card mobile-collapsible ${isCollapsed ? 'is-collapsed' : ''}`.trim()}>
      <div className="chat-sidebar-header">
        <div className="panel-title-with-action">
          <div>
            <div className="label">Chat History</div>
            <h2>Lịch sử hội thoại</h2>
          </div>
          <Button
            variant="outline"
            className="panel-toggle-button"
            onClick={() => onToggleCollapse?.()}
            type="button"
          >
            {isCollapsed ? 'Expand' : 'Collapse'}
          </Button>
        </div>
        <div className="chat-sidebar-actions">
          <Button variant="outline" onClick={() => void createConversation()} disabled={isLoading}>
            Cuộc trò chuyện mới
          </Button>
        </div>
      </div>
      <div className="conversation-list mobile-collapsible-content">
        {conversations.length === 0 && <div className="badge">Chưa có cuộc trò chuyện nào.</div>}
        {conversations.map((conversation) => (
          <div
            key={conversation.conversation_id}
            className={`conversation-item ${conversation.conversation_id === currentConversationId ? 'active' : ''}`.trim()}
          >
            <button
              className="conversation-item-main"
              onClick={() => void selectConversation(conversation.conversation_id)}
            >
              <div className="conversation-item-title">{conversation.title}</div>
              {conversation.last_message_preview && (
                <div className="conversation-item-preview">{conversation.last_message_preview}</div>
              )}
              <div className="conversation-item-meta">
                <span>{formatTimestamp(conversation.last_message_at || conversation.updated_at)}</span>
                <span>{conversation.message_count} tin nhắn</span>
              </div>
            </button>
            <Button
              variant="outline"
              className="conversation-delete-button"
              disabled={isLoading}
              onClick={() => void deleteConversation(conversation.conversation_id)}
            >
              Xóa
            </Button>
          </div>
        ))}
      </div>
    </aside>
  );
};

export default ConversationSidebar;
