export type SseEvent = {
  event: string;
  data: string;
};

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
  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(body),
    signal
  });

  if (!response.ok || !response.body) {
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
