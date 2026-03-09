import { useState } from 'react';
import Button from '@/shared/Button';
import { useChatStore } from './chatStore';

const ChatInput = () => {
  const [value, setValue] = useState('');
  const { sendMessage, stopStreaming, retryLast, resetChat, isStreaming } = useChatStore();

  const handleSend = async () => {
    await sendMessage(value);
    setValue('');
  };

  return (
    <div className="chat-input">
      <textarea
        className="textarea"
        rows={3}
        placeholder="Hỏi bất kỳ điều gì về luật Việt Nam..."
        value={value}
        onChange={(event) => setValue(event.target.value)}
      />
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <Button onClick={handleSend} disabled={isStreaming}>
          Send
        </Button>
        <Button variant="secondary" onClick={stopStreaming} disabled={!isStreaming}>
          Stop
        </Button>
        <Button variant="outline" onClick={retryLast}>
          Retry last
        </Button>
        <Button variant="outline" onClick={resetChat}>
          Reset
        </Button>
      </div>
    </div>
  );
};

export default ChatInput;
