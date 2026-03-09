import { create } from 'zustand';
import { answer } from '@/core/api';
import { streamSse } from '@/core/sse';
import { readLocal, writeLocal } from '@/core/utils';
import type { ChatFilters, ChatMessage, Citation } from '@/core/types';

const STORAGE_KEY = 'legal-chat-state';

type ChatState = {
  messages: ChatMessage[];
  filters: ChatFilters;
  isStreaming: boolean;
  error?: string;
  abortController?: AbortController;
  sendMessage: (content: string) => Promise<void>;
  stopStreaming: () => void;
  retryLast: () => Promise<void>;
  resetChat: () => void;
  updateFilters: (filters: Partial<ChatFilters>) => void;
};

const initialFilters: ChatFilters = {
  tone: 'default',
  topK: 5,
  effectiveStatus: 'active',
  domain: '',
  docType: ''
};

const loadState = () => {
  return readLocal<{ messages: ChatMessage[]; filters: ChatFilters }>(STORAGE_KEY, {
    messages: [],
    filters: initialFilters
  });
};

const persistState = (messages: ChatMessage[], filters: ChatFilters) => {
  writeLocal(STORAGE_KEY, { messages, filters });
};

const uid = () => `${Date.now()}-${Math.random().toString(16).slice(2)}`;

export const useChatStore = create<ChatState>((set, get) => {
  const stored = loadState();

  return {
    messages: stored.messages,
    filters: stored.filters,
    isStreaming: false,
    error: undefined,
    abortController: undefined,
    updateFilters: (filters) => {
      set((state) => {
        const next = { ...state.filters, ...filters };
        persistState(state.messages, next);
        return { filters: next };
      });
    },
    sendMessage: async (content) => {
      if (!content.trim()) return;
      const userMessage: ChatMessage = {
        id: uid(),
        role: 'user',
        content,
        createdAt: Date.now()
      };
      const assistantMessage: ChatMessage = {
        id: uid(),
        role: 'assistant',
        content: '',
        createdAt: Date.now()
      };
      set((state) => {
        const nextMessages = [...state.messages, userMessage, assistantMessage];
        persistState(nextMessages, state.filters);
        return { messages: nextMessages, isStreaming: true, error: undefined };
      });

      const baseUrl = import.meta.env.VITE_API_BASE_URL || '';
      const abortController = new AbortController();
      set({ abortController });

      try {
        await streamSse(
          `${baseUrl}/answer/stream`,
          { question: content, filters: get().filters },
          abortController.signal,
          {
            onEvent: (evt) => {
              if (evt.event === 'token') {
                set((state) => {
                  const updated = state.messages.map((msg) =>
                    msg.id === assistantMessage.id
                      ? { ...msg, content: msg.content + evt.data }
                      : msg
                  );
                  persistState(updated, state.filters);
                  return { messages: updated };
                });
              }
              if (evt.event === 'citations') {
                let citations: Citation[] | undefined;
                try {
                  citations = JSON.parse(evt.data) as Citation[];
                } catch {
                  citations = undefined;
                }
                set((state) => {
                  const updated = state.messages.map((msg) =>
                    msg.id === assistantMessage.id ? { ...msg, citations } : msg
                  );
                  persistState(updated, state.filters);
                  return { messages: updated };
                });
              }
              if (evt.event === 'error') {
                set({ error: evt.data, isStreaming: false });
              }
              if (evt.event === 'done') {
                set({ isStreaming: false });
              }
            }
          }
        );
      } catch (error) {
        if (abortController.signal.aborted) {
          set({ isStreaming: false });
          return;
        }
        try {
          const result = await answer(content, get().filters);
          set((state) => {
            const updated = state.messages.map((msg) =>
              msg.id === assistantMessage.id
                ? {
                    ...msg,
                    content: result.answer,
                    citations: (result.citations || []) as Citation[]
                  }
                : msg
            );
            persistState(updated, state.filters);
            return { messages: updated, isStreaming: false };
          });
        } catch (fallbackError) {
          set({ error: (fallbackError as Error).message, isStreaming: false });
        }
      } finally {
        set({ abortController: undefined, isStreaming: false });
      }
    },
    stopStreaming: () => {
      const controller = get().abortController;
      if (controller) controller.abort();
      set({ isStreaming: false, abortController: undefined });
    },
    retryLast: async () => {
      const { messages } = get();
      const lastUser = [...messages].reverse().find((msg) => msg.role === 'user');
      if (!lastUser) return;
      set((state) => {
        const trimmed = state.messages.filter((msg) => msg.createdAt <= lastUser.createdAt);
        persistState(trimmed, state.filters);
        return { messages: trimmed };
      });
      await get().sendMessage(lastUser.content);
    },
    resetChat: () => {
      set({ messages: [], error: undefined, isStreaming: false });
      persistState([], get().filters);
    }
  };
});
