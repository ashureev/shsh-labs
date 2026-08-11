package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAIProvider handles OpenAI, OpenRouter, and Ollama (OpenAI-compatible).
type OpenAIProvider struct {
	cfg        ProviderConfig
	httpClient *http.Client
}

// NewOpenAIProvider creates an OpenAI-compatible provider.
func NewOpenAIProvider(cfg ProviderConfig) *OpenAIProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OpenAIProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type openAIRequest struct {
	Model       string            `json:"model"`
	Messages    []Message         `json:"messages"`
	Tools       []openAITool      `json:"tools,omitempty"`
	Temperature float32           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type openAITool struct {
	Type     string            `json:"type"`
	Function openAIFunctionDef `json:"function"`
}

type openAIFunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *OpenAIProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	body := openAIRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}

	if len(req.Tools) > 0 {
		body.Tools = make([]openAITool, len(req.Tools))
		for i, t := range req.Tools {
			body.Tools[i] = openAITool{
				Type: "function",
				Function: openAIFunctionDef{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			}
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	url := p.cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute openai request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai api error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("openai error: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openai returned empty choices")
	}

	choice := parsed.Choices[0]
	return &CompletionResponse{
		Content:      choice.Message.Content,
		ToolCalls:    choice.Message.ToolCalls,
		FinishReason: choice.FinishReason,
		Usage:        parsed.Usage,
	}, nil
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	body := openAIRequest{
		Model:       model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal stream request: %w", err)
	}

	url := p.cfg.BaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}

	// Use transport directly without client timeout for unbounded SSE streaming
	resp, err := http.DefaultTransport.RoundTrip(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute stream request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("openai stream error (%d): %s", resp.StatusCode, string(respBody))
	}

	outCh := make(chan StreamChunk, 20)

	go func() {
		defer close(outCh)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" {
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}

			if len(chunk.Choices) > 0 {
				c := chunk.Choices[0]
				outCh <- StreamChunk{
					DeltaContent: c.Delta.Content,
					FinishReason: c.FinishReason,
				}
			}
		}

		if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
			outCh <- StreamChunk{Error: err}
		}
	}()

	return outCh, nil
}
