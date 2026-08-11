package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeminiProvider handles the Google Gemini API (REST/v1beta).
type GeminiProvider struct {
	cfg        ProviderConfig
	httpClient *http.Client
}

// NewGeminiProvider creates a Gemini provider instance.
func NewGeminiProvider(cfg ProviderConfig) *GeminiProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &GeminiProvider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	Contents          []geminiContent `json:"contents"`
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	GenerationConfig  *struct {
		Temperature     float32 `json:"temperature,omitempty"`
		MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	} `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Role  string       `json:"role"`
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (p *GeminiProvider) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.cfg.Model
	}

	var contents []geminiContent
	var systemInstruction *geminiContent

	for _, msg := range req.Messages {
		if msg.Role == RoleSystem {
			systemInstruction = &geminiContent{
				Parts: []geminiPart{{Text: msg.Content}},
			}
			continue
		}

		role := "user"
		if msg.Role == RoleAssistant {
			role = "model"
		}

		contents = append(contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	body := geminiRequest{
		Contents:          contents,
		SystemInstruction: systemInstruction,
	}

	if req.Temperature > 0 || req.MaxTokens > 0 {
		body.GenerationConfig = &struct {
			Temperature     float32 `json:"temperature,omitempty"`
			MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
		}{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.cfg.BaseURL, model, p.cfg.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create gemini http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read gemini response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini api error (%d): %s", resp.StatusCode, string(respBytes))
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("gemini error (%d): %s", parsed.Error.Code, parsed.Error.Message)
	}

	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned empty candidate")
	}

	cand := parsed.Candidates[0]
	var textBuilder strings.Builder
	for _, part := range cand.Content.Parts {
		textBuilder.WriteString(part.Text)
	}

	usage := Usage{}
	if parsed.UsageMetadata != nil {
		usage.PromptTokens = parsed.UsageMetadata.PromptTokenCount
		usage.CompletionTokens = parsed.UsageMetadata.CandidatesTokenCount
		usage.TotalTokens = parsed.UsageMetadata.TotalTokenCount
	}

	return &CompletionResponse{
		Content:      textBuilder.String(),
		FinishReason: cand.FinishReason,
		Usage:        usage,
	}, nil
}

func (p *GeminiProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	// Fallback to generate and single stream chunk for simplicity or streamGenerateContent
	outCh := make(chan StreamChunk, 1)
	go func() {
		defer close(outCh)
		resp, err := p.Generate(ctx, req)
		if err != nil {
			outCh <- StreamChunk{Error: err}
			return
		}
		outCh <- StreamChunk{
			DeltaContent: resp.Content,
			FinishReason: resp.FinishReason,
		}
	}()
	return outCh, nil
}
