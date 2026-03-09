package embedding

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
)

type Client struct {
	APIKey string
	Model  string
	HTTP   *http.Client
	BaseURL string
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		APIKey: apiKey,
		Model:  model,
		BaseURL: "https://api.openai.com",
		HTTP: &http.Client{Timeout: 30 * time.Second},
	}
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float64, error) {
	if c.APIKey == "" {
		return nil, errors.New("openai api key is required")
	}
	payload := embeddingRequest{Model: c.Model, Input: inputs}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, formatOpenAIError("openai embeddings request failed", resp.StatusCode, body)
	}
	var out embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vectors := make([][]float64, 0, len(out.Data))
	for _, item := range out.Data {
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
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
