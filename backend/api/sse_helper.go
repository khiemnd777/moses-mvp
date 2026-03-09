package api

import (
	"bufio"
	"encoding/json"
)

type sseWriter struct {
	w *bufio.Writer
}

func newSSEWriter(w *bufio.Writer) *sseWriter {
	return &sseWriter{w: w}
}

func (s *sseWriter) writeEvent(event string, data interface{}) error {
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
