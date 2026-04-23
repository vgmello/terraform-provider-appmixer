package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet_DecodesJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tkn" {
			t.Errorf("expected auth header 'Bearer tkn', got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"key": "k1", "value": "v1"}})
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, Token: "tkn", HTTP: server.Client()}

	got, err := Get[[]map[string]any](context.Background(), c, "/config")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if len(got) != 1 || got[0]["key"] != "k1" {
		t.Fatalf("unexpected response: %+v", got)
	}
}
