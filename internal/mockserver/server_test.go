package mockserver_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ellosoft/terraform-provider-appmixer/internal/mockserver"
)

// startServer starts the mock and registers cleanup with t.
func startServer(t *testing.T) string {
	t.Helper()
	addr, stop := mockserver.Start()
	t.Cleanup(stop)
	return addr
}

// authDo makes an authenticated request.
func authDo(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		r = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, url, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer mock-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, url, err)
	}
	return resp
}

// mustDecode decodes JSON from r into out.
func mustDecode(t *testing.T, r io.Reader, out any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(out); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// --- Auth tests ---

func TestAuth_ValidCredentials(t *testing.T) {
	base := startServer(t)
	body := map[string]string{"username": "admin@test.com", "password": "test123"}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+"/user/auth", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got map[string]string
	mustDecode(t, resp.Body, &got)
	if got["token"] != "mock-token" {
		t.Errorf("want token 'mock-token', got %q", got["token"])
	}
}

func TestAuth_InvalidCredentials_Returns401(t *testing.T) {
	base := startServer(t)
	body := map[string]string{"username": "wrong@test.com", "password": "badpass"}
	data, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+"/user/auth", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestAuth_MissingBearer_Returns401(t *testing.T) {
	base := startServer(t)
	req, _ := http.NewRequest("GET", base+"/config", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}
