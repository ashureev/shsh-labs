package api

import (
	"encoding/json"
	"net/http"

	"github.com/ashureev/shsh-labs/internal/domain"
	"github.com/ashureev/shsh-labs/internal/llm"
	"github.com/ashureev/shsh-labs/internal/store"
	"github.com/ashureev/shsh-labs/internal/tutor"
	"github.com/go-chi/chi/v5"
)

// SettingsHandler manages user configuration for LLM providers.
type SettingsHandler struct {
	repo        store.Repository
	tutorEngine *tutor.Engine
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(repo store.Repository, engine *tutor.Engine) *SettingsHandler {
	return &SettingsHandler{
		repo:        repo,
		tutorEngine: engine,
	}
}

// RegisterRoutes registers setting endpoints on the API subrouter.
func (h *SettingsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/settings", h.GetSettings)
	r.Post("/settings", h.UpdateSettings)
}

// GetSettings retrieves active LLM settings.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider, _ := h.repo.GetSetting(ctx, "llm_provider")
	model, _ := h.repo.GetSetting(ctx, "llm_model")
	apiKey, _ := h.repo.GetSetting(ctx, "llm_api_key")
	baseURL, _ := h.repo.GetSetting(ctx, "llm_base_url")

	if provider == "" {
		provider = "ollama"
	}
	if model == "" {
		if provider == "gemini" {
			model = "gemini-2.5-flash"
		} else {
			model = "llama3.2"
		}
	}

	// Mask API key for security
	maskedKey := ""
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	} else if len(apiKey) > 0 {
		maskedKey = "****"
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"provider":       provider,
		"model":          model,
		"base_url":       baseURL,
		"has_api_key":    apiKey != "",
		"masked_api_key": maskedKey,
		"enable_ambient": true,
	})
}

// UpdateSettings updates the active LLM provider and reloads the engine provider.
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req domain.UserSettings
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid settings payload")
		return
	}

	if req.Provider != "" {
		_ = h.repo.SetSetting(ctx, "llm_provider", req.Provider)
	}
	if req.Model != "" {
		_ = h.repo.SetSetting(ctx, "llm_model", req.Model)
	}
	if req.APIKey != "" {
		_ = h.repo.SetSetting(ctx, "llm_api_key", req.APIKey)
	}
	if req.BaseURL != "" {
		_ = h.repo.SetSetting(ctx, "llm_base_url", req.BaseURL)
	}

	// Reinitialize tutor engine provider
	apiKey, _ := h.repo.GetSetting(ctx, "llm_api_key")
	if req.APIKey != "" {
		apiKey = req.APIKey
	}
	providerName, _ := h.repo.GetSetting(ctx, "llm_provider")
	modelName, _ := h.repo.GetSetting(ctx, "llm_model")
	baseURL, _ := h.repo.GetSetting(ctx, "llm_base_url")

	newProvider, err := llm.NewProvider(llm.ProviderConfig{
		Provider: providerName,
		Model:    modelName,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	})
	if err == nil && h.tutorEngine != nil {
		h.tutorEngine.SetProvider(newProvider)
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"status": "updated",
	})
}
