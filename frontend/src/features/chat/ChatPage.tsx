import { useEffect, useMemo, useRef, useState } from 'react';
import { downloadCitationAsset, getCitationDetail, unwrapError } from '@/core/api';
import Panel from '@/shared/Panel';
import ChatInput from './ChatInput';
import ChatMessage from './ChatMessage';
import CitationDetailModal from './CitationDetailModal';
import ConversationSidebar from './ConversationSidebar';
import FiltersBar from './FiltersBar';
import SourcesPanel from './SourcesPanel';
import { useChatStore } from './chatStore';
import type { Citation, CitationDetail } from '@/core/types';

const ChatPage = () => {
  const { hydrate, currentConversationId, messagesByConversation, isStreaming, isLoading, error } = useChatStore();
  const [selectedCitations, setSelectedCitations] = useState<Citation[]>([]);
  const [activeCitation, setActiveCitation] = useState<Citation>();
  const [activeCitationDetail, setActiveCitationDetail] = useState<CitationDetail>();
  const [citationDetailError, setCitationDetailError] = useState<string>();
  const [isCitationDetailLoading, setIsCitationDetailLoading] = useState(false);
  const [downloadNotice, setDownloadNotice] = useState<string>();
  const chatEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    void hydrate();
  }, [hydrate]);

  useEffect(() => {
    if (!activeCitation) {
      setActiveCitationDetail(undefined);
      setCitationDetailError(undefined);
      setIsCitationDetailLoading(false);
      return;
    }

    let cancelled = false;
    setIsCitationDetailLoading(true);
    setCitationDetailError(undefined);
    setActiveCitationDetail(undefined);

    void getCitationDetail({
      chunk_id: activeCitation.chunk_id,
      asset_id: activeCitation.asset_id
    })
      .then((detail) => {
        if (!cancelled) {
          setActiveCitationDetail(detail);
        }
      })
      .catch((fetchError) => {
        if (!cancelled) {
          setCitationDetailError(unwrapError(fetchError));
        }
      })
      .finally(() => {
        if (!cancelled) {
          setIsCitationDetailLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [activeCitation]);

  const messages = currentConversationId ? messagesByConversation[currentConversationId] || [] : [];

  useEffect(() => {
    if (!isStreaming && messages.length === 0) return;
    chatEndRef.current?.scrollIntoView({
      block: 'end',
      behavior: isStreaming ? 'auto' : 'smooth'
    });
  }, [isStreaming, messages]);

  const latestAssistantCitations = useMemo(() => {
    const lastAssistant = [...messages].reverse().find((message) => message.role === 'assistant');
    return lastAssistant?.citations || [];
  }, [messages]);

  const handleDownloadCitation = (citation: Citation, fallbackFileName?: string) => {
    setDownloadNotice('Tài liệu đang được tải về');
    window.setTimeout(() => {
      setDownloadNotice((current) => (current === 'Tài liệu đang được tải về' ? undefined : current));
    }, 2500);

    return downloadCitationAsset(citation, fallbackFileName).catch((downloadError) => {
      setDownloadNotice(undefined);
      throw downloadError;
    });
  };

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
                  onOpenCitation={(citation, citations) => {
                    setSelectedCitations(citations);
                    setActiveCitation(citation);
                  }}
                />
              ))}
              <div ref={chatEndRef} />
              {error && <div className="badge">{error}</div>}
            </div>
            <ChatInput />
          </div>
          <Panel bodyClassName="source-panel-content" className="source-panel" title="Nguồn pháp lý">
            <div className="source-panel-body">
              <SourcesPanel
                citations={selectedCitations.length > 0 ? selectedCitations : latestAssistantCitations}
                onDownload={(citation) =>
                  void handleDownloadCitation(citation).catch((downloadError) => setCitationDetailError(unwrapError(downloadError)))
                }
              />
            </div>
          </Panel>
        </div>
        {isStreaming && <div className="chat-stream-indicator">Đang nhận phản hồi trực tuyến...</div>}
      </Panel>
      {downloadNotice && <div className="download-notice-popup">{downloadNotice}</div>}
      {activeCitation && (
        <CitationDetailModal
          citation={activeCitation}
          detail={activeCitationDetail}
          error={citationDetailError}
          isLoading={isCitationDetailLoading}
          onClose={() => setActiveCitation(undefined)}
          onDownload={() =>
            void handleDownloadCitation(activeCitationDetail?.citation || activeCitation, activeCitationDetail?.file_name).catch(
              (downloadError) => setCitationDetailError(unwrapError(downloadError))
            )
          }
        />
      )}
    </div>
  );
};

export default ChatPage;
