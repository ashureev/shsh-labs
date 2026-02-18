//nolint:revive // "api" package name is intentionally concise for this layer.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
