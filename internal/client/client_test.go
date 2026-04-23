package client

import (
	"context"
	"encoding/json"
	"errors"
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

func TestPost_SendsBodyAndDecodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["key"] != "k1" {
			t.Errorf("expected body key=k1, got %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	got, err := Post[map[string]any](context.Background(), c, "/config", map[string]any{"key": "k1", "value": "v1"})
	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if got["key"] != "k1" {
		t.Fatalf("unexpected response: %v", got)
	}
}

func TestPut_SendsBody(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["a"] != float64(1) {
			t.Errorf("expected body a=1, got %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := Put[map[string]any](context.Background(), c, "/svc/1", map[string]any{"a": 1})
	if err != nil || !called {
		t.Fatalf("Put error=%v called=%v", err, called)
	}
}

func TestDelete_NoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.ContentLength > 0 {
			t.Errorf("expected no request body, got Content-Length=%d", r.ContentLength)
		}
		if ct := r.Header.Get("Content-Type"); ct != "" {
			t.Errorf("expected no Content-Type header, got %q", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := Delete[map[string]any](context.Background(), c, "/config/k1")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}
}

func TestError_Non2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := Get[map[string]any](context.Background(), c, "/missing")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Fatalf("expected status 404, got %d", apiErr.StatusCode)
	}
	if apiErr.Method != http.MethodGet || apiErr.Path != "/missing" {
		t.Fatalf("unexpected method/path: %s %s", apiErr.Method, apiErr.Path)
	}
}
