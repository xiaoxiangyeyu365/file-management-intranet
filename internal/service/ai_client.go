package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AIClientConfig holds configuration for the OpenAI-compatible AI client.
type AIClientConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout int // seconds
}

// AIClient is an OpenAI-compatible chat completion HTTP client.
type AIClient struct {
	config     AIClientConfig
	httpClient *http.Client
}

// NewAIClient creates a new AIClient with the given configuration.
func NewAIClient(cfg AIClientConfig) *AIClient {
	return &AIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ChatCompletion sends a text chat completion request.
// If isVision is true, content is sent as an array of content parts;
// otherwise content is sent as a plain string.
func (c *AIClient) ChatCompletion(ctx context.Context, prompt string, isVision bool) (string, error) {
	var content interface{}
	if isVision {
		content = []map[string]interface{}{
			{"type": "text", "text": prompt},
		}
	} else {
		content = prompt
	}

	return c.doChatCompletion(ctx, content)
}

// ChatCompletionWithImage sends a vision request with a base64-encoded image.
func (c *AIClient) ChatCompletionWithImage(ctx context.Context, prompt string, imageBase64 string, mimeType string) (string, error) {
	content := []map[string]interface{}{
		{"type": "text", "text": prompt},
		{"type": "image_url", "image_url": map[string]string{
			"url": fmt.Sprintf("data:%s;base64,%s", mimeType, imageBase64),
		}},
	}

	return c.doChatCompletion(ctx, content)
}

func (c *AIClient) doChatCompletion(ctx context.Context, content interface{}) (string, error) {
	reqBody := chatRequest{
		Model: c.config.Model,
		Messages: []chatMessage{
			{Role: "user", Content: content},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.config.BaseURL + "/chat/completions"

	var resp *http.Response
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

		resp, err = c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("send request: %w", err)
		}

		if resp.StatusCode >= 500 && attempt == 0 {
			resp.Body.Close()
			continue
		}
		break
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
