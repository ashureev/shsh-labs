package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ashureev/shsh-labs/internal/llm"
)

func TestOpenAIProvider_Generate_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-openai-key" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		resp := map[string]interface{}{
			"id": "chatcmpl-123",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Check your file permissions with `ls -la`.",
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := llm.NewOpenAIProvider(llm.ProviderConfig{
		BaseURL: mockServer.URL + "/v1",
		APIKey:  "test-openai-key",
		Model:   "gpt-4o-mini",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Why did permission denied happen?"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Check your file permissions with `ls -la`." {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}

func TestOpenAIProvider_ToolCalling(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"id": "chatcmpl-456",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role": "assistant",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "check_permissions",
									"arguments": `{"path":"/var/log/syslog"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := llm.NewOpenAIProvider(llm.ProviderConfig{
		BaseURL: mockServer.URL + "/v1",
		APIKey:  "ollama-dummy-key",
		Model:   "qwen2.5-coder",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Why can't I read syslog?"},
		},
		Tools: []llm.ToolDefinition{
			{
				Name:        "check_permissions",
				Description: "Checks file permissions and ownership",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{"type": "string"},
					},
					"required": []string{"path"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Function.Name != "check_permissions" || tc.Function.Arguments != `{"path":"/var/log/syslog"}` {
		t.Errorf("unexpected tool call: %+v", tc)
	}
}

func TestGeminiProvider_Generate(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "test-gemini-key" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		resp := map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"role": "model",
						"parts": []map[string]interface{}{
							{
								"text": "Linux uses rwx permission bits. Let's inspect who owns the file.",
							},
						},
					},
					"finishReason": "STOP",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := llm.NewGeminiProvider(llm.ProviderConfig{
		BaseURL: mockServer.URL,
		APIKey:  "test-gemini-key",
		Model:   "gemini-2.5-flash",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := provider.Generate(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Explain permissions"},
		},
	})
	if err != nil {
		t.Fatalf("gemini error: %v", err)
	}

	if resp.Content != "Linux uses rwx permission bits. Let's inspect who owns the file." {
		t.Errorf("unexpected gemini content: %q", resp.Content)
	}
}

func TestOpenAIProvider_Streaming(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{
			`{"choices":[{"delta":{"content":"First "}}]}`,
			`{"choices":[{"delta":{"content":"hint "}}]}`,
			`{"choices":[{"delta":{"content":"chunk."}}]}`,
			`[DONE]`,
		}

		for _, chunk := range chunks {
			if chunk == "[DONE]" {
				fmt.Fprintf(w, "data: [DONE]\n\n")
			} else {
				fmt.Fprintf(w, "data: %s\n\n", chunk)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer mockServer.Close()

	provider := llm.NewOpenAIProvider(llm.ProviderConfig{
		BaseURL: mockServer.URL + "/v1",
		APIKey:  "test-key",
		Model:   "llama3.2",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	streamCh, err := provider.Stream(ctx, llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("stream start failed: %v", err)
	}

	var accumulated string
	for chunk := range streamCh {
		if chunk.Error != nil {
			t.Fatalf("chunk error: %v", chunk.Error)
		}
		accumulated += chunk.DeltaContent
	}

	if accumulated != "First hint chunk." {
		t.Errorf("expected %q, got %q", "First hint chunk.", accumulated)
	}
}
