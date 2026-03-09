package answer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/observability"
)

type chatStreamRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

func (c *Client) StreamAnswer(ctx context.Context, messages []message, cfg CompletionConfig, onDelta func(string) error) error {
	if c.APIKey == "" {
		return errors.New("openai api key is required")
	}
	started := time.Now()
	payload := chatStreamRequest{Model: c.Model, Messages: messages, Stream: true}
	if cfg.Temperature >= 0 {
		t := cfg.Temperature
		payload.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		payload.MaxTokens = cfg.MaxTokens
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		observability.LogError(ctx, nil, "openai", "openai stream failed", map[string]interface{}{
			"model":      c.Model,
			"latency_ms": time.Since(started).Milliseconds(),
			"error":      err.Error(),
		})
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		err := errors.New("openai chat stream request failed")
		observability.LogError(ctx, nil, "openai", "openai stream failed", map[string]interface{}{
			"model":       c.Model,
			"status_code": resp.StatusCode,
			"latency_ms":  time.Since(started).Milliseconds(),
			"error":       err.Error(),
		})
		return err
	}
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			observability.LogInfo(ctx, nil, "openai", "openai stream succeeded", map[string]interface{}{
				"model":      c.Model,
				"latency_ms": time.Since(started).Milliseconds(),
			})
			return nil
		}
		var chunk chatStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return err
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}
