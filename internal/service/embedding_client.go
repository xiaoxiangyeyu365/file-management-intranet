package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// EmbeddingClient handles text embedding via OpenAI-compatible /embeddings API.
type EmbeddingClient struct {
	baseURL string
	apiKey  string
	model   string
	timeout int
	client  *http.Client

	dimOnce sync.Once
	dims    int
}

// NewEmbeddingClient creates a new EmbeddingClient.
func NewEmbeddingClient(baseURL, apiKey, model string, timeout, dims int) *EmbeddingClient {
	return &EmbeddingClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
		dims:    dims,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

// Embed sends texts to the embedding API and returns float64 vectors.
func (c *EmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Model: c.model,
		Input: texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	url := c.baseURL + "/embeddings"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Sort by index to match input order
	result := make([][]float64, len(embResp.Data))
	for _, d := range embResp.Data {
		if d.Index >= 0 && d.Index < len(result) {
			result[d.Index] = d.Embedding
		}
	}

	// Validate dimension and cache
	c.dimOnce.Do(func() {
		actualDims := len(result[0])
		if actualDims != c.dims {
			log.Printf("[RAG] Embedding dimension mismatch: configured %d, API returned %d. Using actual.", c.dims, actualDims)
			c.dims = actualDims
		}
	})

	// Validate all embeddings have consistent dimensions
	for i, emb := range result {
		if len(emb) != c.dims {
			return nil, fmt.Errorf("embedding dimension mismatch at index %d: expected %d, got %d", i, c.dims, len(emb))
		}
	}

	return result, nil
}

// Dimensions returns the cached embedding dimension.
func (c *EmbeddingClient) Dimensions() int {
	return c.dims
}
