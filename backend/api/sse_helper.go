package api

import (
	"bufio"
	"encoding/json"
	"sync"
)

type sseWriter struct {
	w  *bufio.Writer
	mu sync.Mutex
}

func newSSEWriter(w *bufio.Writer) *sseWriter {
	return &sseWriter{w: w}
}

func (s *sseWriter) writeEvent(event string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := s.w.WriteString("event: " + event + "\n"); err != nil {
		return err
	}
	if _, err := s.w.WriteString("data: " + string(payload) + "\n\n"); err != nil {
		return err
	}
	return s.w.Flush()
}

func (s *sseWriter) writeHeartbeat() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.w.WriteString(": heartbeat\n\n"); err != nil {
		return err
	}
	return s.w.Flush()
}
