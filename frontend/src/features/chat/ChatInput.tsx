import { useState } from 'react';
import Button from '@/shared/Button';
import { useChatStore } from './chatStore';

const ChatInput = () => {
  const [value, setValue] = useState('');
  const { sendMessage, isStreaming } = useChatStore();

  const handleSend = async () => {
    const trimmed = value.trim();
    if (!trimmed || isStreaming) return;
    await sendMessage(trimmed);
    setValue('');
  };

  const canSend = value.trim().length > 0 && !isStreaming;

  return (
    <div className="chat-input">
      <textarea
        className="textarea"
        rows={3}
        placeholder="Hỏi bất kỳ điều gì về luật Việt Nam..."
        value={value}
        disabled={isStreaming}
        onChange={(event) => setValue(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            void handleSend();
          }
        }}
      />
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <Button onClick={() => void handleSend()} disabled={!canSend}>
          Gửi câu hỏi
        </Button>
      </div>
    </div>
  );
};

export default ChatInput;
