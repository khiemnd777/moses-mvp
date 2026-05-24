import { useState } from 'react';
import Button from '@/shared/Button';
import { useChatStore } from './chatStore';

const suggestedQuestions = [
  'Tóm tắt căn cứ pháp lý và điều khoản áp dụng cho trường hợp này',
  'Điều kiện, hồ sơ và thủ tục cần thực hiện là gì?',
  'Văn bản nào đang còn hiệu lực điều chỉnh vấn đề này?'
];

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
        {!value.trim() &&
          suggestedQuestions.map((question) => (
            <button
              className="button outline chat-suggestion-button"
              disabled={isStreaming}
              key={question}
              onClick={() => setValue(question)}
              type="button"
            >
              {question}
            </button>
          ))}
      </div>
    </div>
  );
};

export default ChatInput;
