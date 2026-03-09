package answer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/khiemnd777/legal_api/observability"
)

type Client struct {
	APIKey  string
	Model   string
	HTTP    *http.Client
	BaseURL string
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: "https://api.openai.com",
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionConfig struct {
	Temperature float64
	MaxTokens   int
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *Client) Answer(ctx context.Context, messages []message, cfg CompletionConfig) (string, error) {
	if c.APIKey == "" {
		return "", errors.New("openai api key is required")
	}
	started := time.Now()
	payload := chatRequest{Model: c.Model, Messages: messages}
	if cfg.Temperature >= 0 {
		t := cfg.Temperature
		payload.Temperature = &t
	}
	if cfg.MaxTokens > 0 {
		payload.MaxTokens = cfg.MaxTokens
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		observability.LogError(ctx, nil, "openai", "openai completion failed", map[string]interface{}{
			"model":      c.Model,
			"latency_ms": time.Since(started).Milliseconds(),
			"error":      err.Error(),
		})
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err := formatOpenAIError("openai chat request failed", resp.StatusCode, body)
		observability.LogError(ctx, nil, "openai", "openai completion failed", map[string]interface{}{
			"model":       c.Model,
			"status_code": resp.StatusCode,
			"latency_ms":  time.Since(started).Milliseconds(),
			"error":       err.Error(),
		})
		return "", err
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("no chat choices")
	}
	observability.LogInfo(ctx, nil, "openai", "openai completion succeeded", map[string]interface{}{
		"model":      c.Model,
		"latency_ms": time.Since(started).Milliseconds(),
	})
	return out.Choices[0].Message.Content, nil
}

func (c *Client) HealthCheck(ctx context.Context) error {
	if c.APIKey == "" {
		return errors.New("openai api key is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return formatOpenAIError("openai health check failed", resp.StatusCode, body)
	}
	return nil
}

func formatOpenAIError(prefix string, status int, body []byte) error {
	var oe openaiError
	if err := json.Unmarshal(body, &oe); err == nil && oe.Error.Message != "" {
		return fmt.Errorf("%s: status=%d type=%s code=%s message=%s", prefix, status, oe.Error.Type, oe.Error.Code, oe.Error.Message)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return fmt.Errorf("%s: status=%d", prefix, status)
	}
	return fmt.Errorf("%s: status=%d body=%s", prefix, status, trimmed)
}
