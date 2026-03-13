import { create } from 'zustand';
import { createConversation, deleteConversation, getConversation, listConversations } from '@/core/api';
import { streamSse } from '@/core/sse';
import { readLocal, writeLocal } from '@/core/utils';
import type { ChatFilters, ChatMessage, Citation, Conversation } from '@/core/types';

const STORAGE_KEY = 'legal-chat-ui-state';

type PersistedState = {
  filters: ChatFilters;
  currentConversationId?: string;
};

type ChatState = {
  conversations: Conversation[];
  currentConversationId?: string;
  messagesByConversation: Record<string, ChatMessage[]>;
  filters: ChatFilters;
  isStreaming: boolean;
  isLoading: boolean;
  error?: string;
  abortController?: AbortController;
  hydrate: () => Promise<void>;
  selectConversation: (conversationId: string) => Promise<void>;
  createConversation: () => Promise<void>;
  deleteConversation: (conversationId: string) => Promise<void>;
  sendMessage: (content: string) => Promise<void>;
  retryLast: () => Promise<void>;
  stopStreaming: () => void;
  updateFilters: (filters: Partial<ChatFilters>) => void;
};

const initialFilters: ChatFilters = {
  tone: 'default',
  topK: 5,
  effectiveStatus: 'active',
  domain: '',
  docType: '',
  documentNumber: '',
  articleNumber: ''
};

const loadState = (): PersistedState =>
  readLocal<PersistedState>(STORAGE_KEY, {
    filters: initialFilters
  });

const persistState = (filters: ChatFilters, currentConversationId?: string) => {
  writeLocal(STORAGE_KEY, { filters, currentConversationId });
};

const sortConversations = (items: Conversation[]) =>
  [...items].sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime());

const upsertConversation = (items: Conversation[], next: Conversation) => {
  const filtered = items.filter((item) => item.conversation_id !== next.conversation_id);
  return sortConversations([next, ...filtered]);
};

const parseStreamPayload = <T>(value: string): T | undefined => {
  try {
    return JSON.parse(value) as T;
  } catch {
    return undefined;
  }
};

export const useChatStore = create<ChatState>((set, get) => {
  const stored = loadState();

  return {
    conversations: [],
    currentConversationId: stored.currentConversationId,
    messagesByConversation: {},
    filters: stored.filters,
    isStreaming: false,
    isLoading: false,
    error: undefined,
    abortController: undefined,

    hydrate: async () => {
      set({ isLoading: true, error: undefined });
      try {
        const conversations = sortConversations(await listConversations());
        let currentConversationId = get().currentConversationId;
        if (!currentConversationId && conversations.length > 0) {
          currentConversationId = conversations[0].conversation_id;
        }

        const messagesByConversation = { ...get().messagesByConversation };
        if (currentConversationId) {
          const conversation = await getConversation(currentConversationId);
          messagesByConversation[currentConversationId] = conversation.messages || [];
          const merged = upsertConversation(conversations, {
            ...conversation,
            messages: undefined
          });
          set({
            conversations: merged,
            currentConversationId,
            messagesByConversation,
            isLoading: false
          });
          persistState(get().filters, currentConversationId);
          return;
        }

        set({ conversations, currentConversationId, messagesByConversation, isLoading: false });
        persistState(get().filters, currentConversationId);
      } catch (error) {
        set({ error: (error as Error).message, isLoading: false });
      }
    },

    selectConversation: async (conversationId) => {
      set({ isLoading: true, error: undefined });
      try {
        const conversation = await getConversation(conversationId);
        set((state) => ({
          currentConversationId: conversationId,
          messagesByConversation: {
            ...state.messagesByConversation,
            [conversationId]: conversation.messages || []
          },
          conversations: upsertConversation(state.conversations, { ...conversation, messages: undefined }),
          isLoading: false
        }));
        persistState(get().filters, conversationId);
      } catch (error) {
        set({ error: (error as Error).message, isLoading: false });
      }
    },

    createConversation: async () => {
      set({ isLoading: true, error: undefined });
      try {
        const conversation = await createConversation();
        set((state) => ({
          conversations: upsertConversation(state.conversations, conversation),
          currentConversationId: conversation.conversation_id,
          messagesByConversation: {
            ...state.messagesByConversation,
            [conversation.conversation_id]: []
          },
          isLoading: false
        }));
        persistState(get().filters, conversation.conversation_id);
      } catch (error) {
        set({ error: (error as Error).message, isLoading: false });
      }
    },

    deleteConversation: async (conversationId) => {
      set({ isLoading: true, error: undefined });
      try {
        await deleteConversation(conversationId);
        set((state) => {
          const conversations = state.conversations.filter((item) => item.conversation_id !== conversationId);
          const messagesByConversation = { ...state.messagesByConversation };
          delete messagesByConversation[conversationId];

          let nextConversationId = state.currentConversationId;
          if (state.currentConversationId === conversationId) {
            nextConversationId = conversations[0]?.conversation_id;
          }

          return {
            conversations,
            currentConversationId: nextConversationId,
            messagesByConversation,
            isLoading: false
          };
        });

        const nextConversationId = get().currentConversationId;
        if (nextConversationId) {
          const conversation = await getConversation(nextConversationId);
          set((state) => ({
            conversations: upsertConversation(state.conversations, { ...conversation, messages: undefined }),
            messagesByConversation: {
              ...state.messagesByConversation,
              [nextConversationId]: conversation.messages || []
            }
          }));
        }
        persistState(get().filters, get().currentConversationId);
      } catch (error) {
        set({ error: (error as Error).message, isLoading: false });
      }
    },

    sendMessage: async (content) => {
      const trimmed = content.trim();
      if (!trimmed) return;

      let conversationId = get().currentConversationId;
      if (!conversationId) {
        try {
          const conversation = await createConversation();
          set((state) => ({
            conversations: upsertConversation(state.conversations, conversation),
            currentConversationId: conversation.conversation_id,
            messagesByConversation: {
              ...state.messagesByConversation,
              [conversation.conversation_id]: []
            }
          }));
          conversationId = conversation.conversation_id;
          persistState(get().filters, conversationId);
        } catch (error) {
          set({ error: (error as Error).message });
          return;
        }
      }

      const optimisticUser: ChatMessage = {
        message_id: `temp-user-${Date.now()}`,
        conversation_id: conversationId,
        role: 'user',
        content: trimmed,
        citations: [],
        created_at: new Date().toISOString()
      };
      const optimisticAssistant: ChatMessage = {
        message_id: `temp-assistant-${Date.now()}`,
        conversation_id: conversationId,
        role: 'assistant',
        content: '',
        citations: [],
        created_at: new Date().toISOString()
      };

      set((state) => ({
        isStreaming: true,
        error: undefined,
        messagesByConversation: {
          ...state.messagesByConversation,
          [conversationId!]: [...(state.messagesByConversation[conversationId!] || []), optimisticUser, optimisticAssistant]
        }
      }));

      const abortController = new AbortController();
      set({ abortController });
      let activeAssistantId = optimisticAssistant.message_id;
      let activeUserId = optimisticUser.message_id;

      const baseUrl = import.meta.env.VITE_API_BASE_URL || '';
      try {
        await streamSse(
          `${baseUrl}/messages/stream`,
          { conversation_id: conversationId, content: trimmed, filters: get().filters },
          abortController.signal,
          {
            onEvent: (evt) => {
              if (evt.event === 'meta') {
                const payload = parseStreamPayload<{
                  conversation_id: string;
                  user_message_id: string;
                  assistant_message_id: string;
                  title?: string;
                }>(evt.data);
                if (!payload) return;
                activeUserId = payload.user_message_id;
                activeAssistantId = payload.assistant_message_id;
                set((state) => {
                  const current = state.messagesByConversation[payload.conversation_id] || [];
                  const nextMessages = current.map((item) => {
                    if (item.message_id === optimisticUser.message_id || item.message_id === activeUserId) {
                      return { ...item, message_id: payload.user_message_id };
                    }
                    if (item.message_id === optimisticAssistant.message_id || item.message_id === activeAssistantId) {
                      return { ...item, message_id: payload.assistant_message_id };
                    }
                    return item;
                  });
                  const nextConversation = state.conversations.find((item) => item.conversation_id === payload.conversation_id);
                  return {
                    currentConversationId: payload.conversation_id,
                    messagesByConversation: {
                      ...state.messagesByConversation,
                      [payload.conversation_id]: nextMessages
                    },
                    conversations: nextConversation
                      ? upsertConversation(state.conversations, {
                          ...nextConversation,
                          title: payload.title || nextConversation.title,
                          last_message_preview: trimmed,
                          updated_at: new Date().toISOString()
                        })
                      : state.conversations
                  };
                });
              }

              if (evt.event === 'token') {
                const payload = parseStreamPayload<{ delta?: string }>(evt.data);
                const delta = payload?.delta || '';
                if (!delta) return;
                set((state) => ({
                  messagesByConversation: {
                    ...state.messagesByConversation,
                    [conversationId!]: (state.messagesByConversation[conversationId!] || []).map((item) =>
                      item.message_id === activeAssistantId
                        ? { ...item, content: item.content + delta }
                        : item
                    )
                  }
                }));
              }

              if (evt.event === 'citations') {
                const payload = parseStreamPayload<Citation[] | { citations?: Citation[] }>(evt.data);
                const citations = Array.isArray(payload) ? payload : payload?.citations || [];
                set((state) => ({
                  messagesByConversation: {
                    ...state.messagesByConversation,
                    [conversationId!]: (state.messagesByConversation[conversationId!] || []).map((item) =>
                      item.message_id === activeAssistantId
                        ? { ...item, citations }
                        : item
                    )
                  }
                }));
              }

              if (evt.event === 'error') {
                const payload = parseStreamPayload<{ message?: string }>(evt.data);
                set({ error: payload?.message || 'Streaming failed', isStreaming: false });
              }

              if (evt.event === 'done') {
                set({ isStreaming: false });
              }
            }
          }
        );

        const refreshed = await getConversation(conversationId);
        set((state) => ({
          conversations: upsertConversation(state.conversations, { ...refreshed, messages: undefined }),
          messagesByConversation: {
            ...state.messagesByConversation,
            [conversationId!]: refreshed.messages || []
          },
          currentConversationId: conversationId,
          isStreaming: false,
          abortController: undefined
        }));
        persistState(get().filters, conversationId);
      } catch (error) {
        if (abortController.signal.aborted) {
          set({ isStreaming: false, abortController: undefined });
          const refreshed = await getConversation(conversationId);
          set((state) => ({
            conversations: upsertConversation(state.conversations, { ...refreshed, messages: undefined }),
            messagesByConversation: {
              ...state.messagesByConversation,
              [conversationId!]: refreshed.messages || []
            }
          }));
          return;
        }

        try {
          const refreshed = await getConversation(conversationId);
          set((state) => ({
            conversations: upsertConversation(state.conversations, { ...refreshed, messages: undefined }),
            messagesByConversation: {
              ...state.messagesByConversation,
              [conversationId!]: refreshed.messages || []
            },
            error: (error as Error).message,
            isStreaming: false,
            abortController: undefined
          }));
        } catch (refreshError) {
          set({
            error: (refreshError as Error).message || (error as Error).message,
            isStreaming: false,
            abortController: undefined
          });
        }
      }
    },

    retryLast: async () => {
      const conversationId = get().currentConversationId;
      if (!conversationId) return;
      const messages = get().messagesByConversation[conversationId] || [];
      const lastUser = [...messages].reverse().find((message) => message.role === 'user');
      if (!lastUser) return;
      await get().sendMessage(lastUser.content);
    },

    stopStreaming: () => {
      const controller = get().abortController;
      if (controller) controller.abort();
      set({ isStreaming: false, abortController: undefined });
    },

    updateFilters: (filters) => {
      set((state) => {
        const next = { ...state.filters, ...filters };
        persistState(next, state.currentConversationId);
        return { filters: next };
      });
    }
  };
});
