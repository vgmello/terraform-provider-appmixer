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

// --- Config tests ---

func TestConfig_GetAll_ReturnsSeedEntry(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/config", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) == 0 {
		t.Fatal("expected at least one seed config entry")
	}
}

func TestConfig_PostAndGet(t *testing.T) {
	base := startServer(t)
	entry := map[string]any{"key": "TEST_KEY", "value": "hello"}
	resp := authDo(t, "POST", base+"/config", entry)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST want 200, got %d", resp.StatusCode)
	}

	resp2 := authDo(t, "GET", base+"/config", nil)
	defer resp2.Body.Close()
	var all []map[string]any
	mustDecode(t, resp2.Body, &all)
	found := false
	for _, e := range all {
		if e["key"] == "TEST_KEY" {
			found = true
		}
	}
	if !found {
		t.Error("created entry not found in GET /config")
	}
}

func TestConfig_Delete_RemovesEntry(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/config", map[string]any{"key": "TO_DELETE", "value": "x"})

	resp := authDo(t, "DELETE", base+"/config/TO_DELETE", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	mustDecode(t, resp.Body, &result)
	if result["ok"] != true {
		t.Errorf("expected ok:true, got %v", result)
	}
}

func TestConfig_Delete_NotFound_Returns404(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "DELETE", base+"/config/DOES_NOT_EXIST", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

// --- Service Config tests ---

func TestServiceConfig_GetAll_ReturnsSeedEntry(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/service-config", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) == 0 {
		t.Fatal("expected at least one seed service config entry")
	}
}

func TestServiceConfig_GetByID(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/service-config/appmixer:google", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["serviceId"] != "appmixer:google" {
		t.Errorf("expected serviceId 'appmixer:google', got %v", got["serviceId"])
	}
}

func TestServiceConfig_GetByID_NotFound(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/service-config/appmixer:missing", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestServiceConfig_CreateAndUpdate(t *testing.T) {
	base := startServer(t)
	entry := map[string]any{"serviceId": "appmixer:test", "client_id": "cid"}
	resp := authDo(t, "POST", base+"/service-config", entry)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST want 200, got %d", resp.StatusCode)
	}

	updated := map[string]any{"serviceId": "appmixer:test", "client_id": "updated"}
	resp2 := authDo(t, "PUT", base+"/service-config/appmixer:test", updated)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp2.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp2.Body, &got)
	if got["client_id"] != "updated" {
		t.Errorf("expected client_id 'updated', got %v", got["client_id"])
	}
}

func TestServiceConfig_Delete(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/service-config", map[string]any{"serviceId": "appmixer:todel"})

	resp := authDo(t, "DELETE", base+"/service-config/appmixer:todel", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}

	resp2 := authDo(t, "GET", base+"/service-config/appmixer:todel", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("expect 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestServiceConfig_PatternFilter(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/service-config", map[string]any{"serviceId": "appmixer:filterme"})

	resp := authDo(t, "GET", base+"/service-config?pattern=filterme", nil)
	defer resp.Body.Close()
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 filtered result, got %d", len(got))
	}
}
