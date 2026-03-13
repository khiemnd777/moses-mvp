export type SseEvent = {
  event: string;
  data: string;
};

import { AUTH_TOKEN_KEY, CHANGE_PASSWORD_PATH, PLAYGROUND_LOGIN_PATH } from '@/playground/apiClient.js';

type StreamHandlers = {
  onEvent: (evt: SseEvent) => void;
  onError?: (error: Error) => void;
  onDone?: () => void;
};

export const streamSse = async (
  url: string,
  body: Record<string, unknown>,
  signal: AbortSignal,
  handlers: StreamHandlers
) => {
  const token = window.localStorage.getItem(AUTH_TOKEN_KEY);
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'text/event-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {})
    },
    body: JSON.stringify(body),
    signal
  });

  if (!response.ok || !response.body) {
    if (response.status === 401) {
      window.localStorage.removeItem(AUTH_TOKEN_KEY);
      if (window.location.pathname !== PLAYGROUND_LOGIN_PATH) {
        window.location.assign(PLAYGROUND_LOGIN_PATH);
      }
    }
    if (response.status === 403) {
      const text = await response.text();
      if (text.includes('password_change_required') && window.location.pathname !== CHANGE_PASSWORD_PATH) {
        window.location.assign(CHANGE_PASSWORD_PATH);
      }
    }
    throw new Error(`SSE request failed (${response.status})`);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let boundary = buffer.indexOf('\n\n');
    while (boundary !== -1) {
      const chunk = buffer.slice(0, boundary).trim();
      buffer = buffer.slice(boundary + 2);
      if (chunk) {
        const lines = chunk.split('\n');
        let event = 'message';
        let data = '';
        for (const line of lines) {
          if (line.startsWith('event:')) {
            event = line.replace('event:', '').trim();
          }
          if (line.startsWith('data:')) {
            data += line.replace('data:', '').trim();
          }
        }
        handlers.onEvent({ event, data });
      }
      boundary = buffer.indexOf('\n\n');
    }
  }

  handlers.onDone?.();
};
