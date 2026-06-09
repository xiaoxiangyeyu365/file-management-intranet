package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAIClient_ChatCompletion_Text(t *testing.T) {
	wantContent := "摘要：测试文档\n标签：测试,单元测试"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer test-key, got %s", r.Header.Get("Authorization"))
		}

		// Verify request body
		var reqBody chatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if reqBody.Model != "test-model" {
			t.Errorf("expected model test-model, got %s", reqBody.Model)
		}
		if len(reqBody.Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(reqBody.Messages))
		}
		if reqBody.Messages[0].Role != "user" {
			t.Errorf("expected role user, got %s", reqBody.Messages[0].Role)
		}
		// For non-vision, content should be a string
		contentStr, ok := reqBody.Messages[0].Content.(string)
		if !ok {
			t.Errorf("expected content to be string, got %T", reqBody.Messages[0].Content)
		} else if contentStr != "hello" {
			t.Errorf("expected content 'hello', got %s", contentStr)
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: wantContent}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(AIClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10,
	})

	got, err := client.ChatCompletion(context.Background(), "hello", false)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if got != wantContent {
		t.Errorf("ChatCompletion() = %q, want %q", got, wantContent)
	}
}

func TestAIClient_ChatCompletion_RetryOn5xx(t *testing.T) {
	var calls atomic.Int32

	wantContent := "recovered"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount := calls.Add(1)
		if callCount == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{Message: struct {
					Content string `json:"content"`
				}{Content: wantContent}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(AIClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10,
	})

	got, err := client.ChatCompletion(context.Background(), "hello", false)
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if got != wantContent {
		t.Errorf("ChatCompletion() = %q, want %q", got, wantContent)
	}

	totalCalls := calls.Load()
	if totalCalls != 2 {
		t.Errorf("expected 2 calls, got %d", totalCalls)
	}
}
