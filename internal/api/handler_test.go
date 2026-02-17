package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestIsDevelopment(t *testing.T) {
	// Save original env
	origEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", origEnv)

	tests := []struct {
		name        string
		env         string
		frontendURL string
		want        bool
	}{
		{"EnvDev", "development", "", true},
		{"EnvProd", "production", "", false},
		{"URLMatchesLocalhost", "", "http://localhost:3000", true},
		{"URLMatches127", "", "http://127.0.0.1:3000", true},
		{"URLMatchesDash", "", "/dashboard", true},
		{"URLEmpty", "", "", true},
		{"URLProd", "", "https://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv("APP_ENV", tt.env)
			} else {
				os.Unsetenv("APP_ENV")
			}

			h := &Handler{frontendRedirectURL: tt.frontendURL}
			if got := h.isDevelopment(); got != tt.want {
				t.Errorf("Handler.isDevelopment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"foo": "bar"}

	JSON(w, http.StatusOK, data)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if got["foo"] != "bar" {
		t.Errorf("Expected foo=bar, got %v", got["foo"])
	}
}
