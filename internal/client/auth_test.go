package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin_ExchangesCredentialsForToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/auth" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["username"] != "u" || body["password"] != "p" {
			t.Errorf("unexpected creds: %v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"id":"x"},"token":"returned-token"}`))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	if err := c.Login(context.Background(), "u", "p"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if c.Token != "returned-token" {
		t.Fatalf("expected token on client, got %q", c.Token)
	}
}

func TestLogin_ReturnsAPIErrorOn401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer server.Close()

	c := &Client{BaseURL: server.URL, HTTP: server.Client()}
	err := c.Login(context.Background(), "u", "p")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 {
		t.Fatalf("expected status 401, got %d", apiErr.StatusCode)
	}
	if apiErr.Path != "/user/auth" {
		t.Fatalf("expected path /user/auth, got %q", apiErr.Path)
	}
	if c.Token != "" {
		t.Fatalf("expected Token to remain empty on failure, got %q", c.Token)
	}
}
