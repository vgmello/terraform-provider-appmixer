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

// --- ACL tests ---

func TestACL_GetReturnsEmptyArrayForSeededType(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/acl/components", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []any
	mustDecode(t, resp.Body, &got)
	if got == nil {
		t.Error("expected non-nil (empty) array")
	}
}

func TestACL_GetReturnsEmptyArrayForUnknownType(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/acl/unknown", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200 (empty array), got %d", resp.StatusCode)
	}
	var got []any
	mustDecode(t, resp.Body, &got)
	if len(got) != 0 {
		t.Errorf("expected empty array, got %v", got)
	}
}

func TestACL_PostReplacesRules(t *testing.T) {
	base := startServer(t)
	rules := []map[string]any{{"role": "admin", "action": []string{"*"}}}
	resp := authDo(t, "POST", base+"/acl/components", rules)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST want 200, got %d", resp.StatusCode)
	}
	var got []any
	mustDecode(t, resp.Body, &got)
	if len(got) != 1 {
		t.Errorf("expected 1 rule, got %d", len(got))
	}

	resp2 := authDo(t, "GET", base+"/acl/components", nil)
	defer resp2.Body.Close()
	var got2 []any
	mustDecode(t, resp2.Body, &got2)
	if len(got2) != 1 {
		t.Errorf("GET after POST: expected 1 rule, got %d", len(got2))
	}
}

// --- Modifiers tests ---

func TestModifiers_GetWhenNotSet_Returns404(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/modifiers", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404 when unset, got %d", resp.StatusCode)
	}
}

func TestModifiers_PutAndGet(t *testing.T) {
	base := startServer(t)
	doc := map[string]any{"prefix": "test-", "enabled": true}
	resp := authDo(t, "PUT", base+"/modifiers", doc)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp.StatusCode)
	}

	resp2 := authDo(t, "GET", base+"/modifiers", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("GET after PUT want 200, got %d", resp2.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp2.Body, &got)
	if got["prefix"] != "test-" {
		t.Errorf("expected prefix 'test-', got %v", got["prefix"])
	}
}

func TestModifiers_Delete_Returns404OnGet(t *testing.T) {
	base := startServer(t)
	authDo(t, "PUT", base+"/modifiers", map[string]any{"x": 1})

	resp := authDo(t, "DELETE", base+"/modifiers", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}

	resp2 := authDo(t, "GET", base+"/modifiers", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("want 404 after delete, got %d", resp2.StatusCode)
	}
}

// --- Flows tests ---

func TestFlows_GetAll_ReturnsSeedFlows(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/flows", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 seed flows, got %d", len(got))
	}
}

func TestFlows_GetByID(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/flows/flow-1", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["flowId"] != "flow-1" {
		t.Errorf("expected flowId 'flow-1', got %v", got["flowId"])
	}
}

func TestFlows_GetByID_NotFound(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/flows/no-such-flow", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestFlows_CreateReturnFlowIDOnly(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/flows", map[string]any{"name": "My Flow"})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if _, ok := got["flowId"]; !ok {
		t.Error("expected flowId in POST response")
	}
	if _, hasStage := got["stage"]; hasStage {
		t.Error("POST /flows should return only {flowId}, not full document")
	}
}

func TestFlows_CreateAssignsGeneratedID(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/flows", map[string]any{"name": "Flow"})
	defer resp.Body.Close()
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	flowID, _ := got["flowId"].(string)
	if flowID == "" || flowID == "flow-1" || flowID == "flow-2" {
		t.Errorf("expected generated flow ID starting at flow-1000, got %q", flowID)
	}
}

func TestFlows_PutUpdatesFlow(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "PUT", base+"/flows/flow-1", map[string]any{"flowId": "flow-1", "stage": "running"})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp.StatusCode)
	}
}

func TestFlows_DeleteRemovesFlow(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/flows", map[string]any{"name": "Temp"})

	resp := authDo(t, "GET", base+"/flows", nil)
	defer resp.Body.Close()
	var all []map[string]any
	mustDecode(t, resp.Body, &all)
	lastID, _ := all[len(all)-1]["flowId"].(string)

	resp2 := authDo(t, "DELETE", base+"/flows/"+lastID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp2.StatusCode)
	}
}

// --- Accounts tests ---

func TestAccounts_GetAll_ReturnsSeedAccount(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/accounts", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) == 0 {
		t.Fatal("expected at least one seed account")
	}
}

func TestAccounts_CreateAndGetByID(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/accounts", map[string]any{
		"service": "appmixer:github",
		"name":    "github-account",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST want 200, got %d", resp.StatusCode)
	}
	var created map[string]any
	mustDecode(t, resp.Body, &created)
	accountID, _ := created["accountId"].(string)
	if accountID == "" {
		t.Fatal("expected accountId in POST response")
	}

	resp2 := authDo(t, "GET", base+"/accounts/"+accountID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("GET by ID want 200, got %d", resp2.StatusCode)
	}
}

func TestAccounts_Post_UpsertsOnMatchingServiceAndName(t *testing.T) {
	base := startServer(t)
	first := authDo(t, "POST", base+"/accounts", map[string]any{
		"service": "appmixer:upsert",
		"name":    "upsert-test",
		"token":   "tok-1",
	})
	defer first.Body.Close()
	if first.StatusCode != 200 {
		t.Fatalf("first POST want 200, got %d", first.StatusCode)
	}
	var created map[string]any
	mustDecode(t, first.Body, &created)
	firstID, _ := created["accountId"].(string)
	if firstID == "" {
		t.Fatal("expected accountId in first POST response")
	}

	second := authDo(t, "POST", base+"/accounts", map[string]any{
		"service": "appmixer:upsert",
		"name":    "upsert-test",
		"token":   "tok-2",
	})
	defer second.Body.Close()
	if second.StatusCode != 200 {
		t.Fatalf("second POST want 200, got %d", second.StatusCode)
	}
	var upserted map[string]any
	mustDecode(t, second.Body, &upserted)
	secondID, _ := upserted["accountId"].(string)
	if secondID != firstID {
		t.Fatalf("expected upsert to reuse accountId %q, got %q", firstID, secondID)
	}
	if upserted["token"] != "tok-2" {
		t.Errorf("expected token to be updated to 'tok-2', got %v", upserted["token"])
	}
}

func TestAccounts_Put_UpdatesDisplayName(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "PUT", base+"/accounts/acc-1", map[string]any{"displayName": "Updated"})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["displayName"] != "Updated" {
		t.Errorf("expected displayName 'Updated', got %v", got["displayName"])
	}
}

func TestAccounts_Delete(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "DELETE", base+"/accounts/acc-1", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}

	resp2 := authDo(t, "GET", base+"/accounts/acc-1", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("want 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestAccounts_TestEndpoint_ReturnsRevoked(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/accounts/acc-1/test", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["revoked"] != false {
		t.Errorf("expected revoked:false, got %v", got["revoked"])
	}
}

// --- Users tests ---

func TestUsers_GetAll_ReturnsSeedUser(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/users", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) == 0 {
		t.Fatal("expected at least one seed user")
	}
}

func TestUsers_CreateAndGetByID(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/user", map[string]any{
		"email": "new@test.com",
		"scope": []string{"user"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST /user want 200, got %d", resp.StatusCode)
	}
	var created map[string]any
	mustDecode(t, resp.Body, &created)
	userID, _ := created["userId"].(string)
	if userID == "" {
		t.Fatal("expected userId in POST /user response")
	}

	resp2 := authDo(t, "GET", base+"/users/"+userID, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("GET /users/:id want 200, got %d", resp2.StatusCode)
	}
}

func TestUsers_Put_ReplacesUser(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "PUT", base+"/users/user-1", map[string]any{
		"userId": "user-1",
		"email":  "updated@test.com",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["email"] != "updated@test.com" {
		t.Errorf("expected email 'updated@test.com', got %v", got["email"])
	}
}

func TestUsers_Delete_ReturnsTicketAndRemovesUser(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "DELETE", base+"/users/user-1", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	ticket, _ := got["ticket"].(string)
	if ticket == "" {
		t.Fatal("expected ticket in DELETE response")
	}

	resp2 := authDo(t, "GET", base+"/users/user-1", nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 404 {
		t.Fatalf("want 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestUsers_DeleteStatus_ReturnsCompleted(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/user", map[string]any{"email": "tmp@test.com"})

	resp := authDo(t, "GET", base+"/users", nil)
	defer resp.Body.Close()
	var all []map[string]any
	mustDecode(t, resp.Body, &all)
	userID, _ := all[len(all)-1]["userId"].(string)

	delResp := authDo(t, "DELETE", base+"/users/"+userID, nil)
	defer delResp.Body.Close()
	var delGot map[string]any
	mustDecode(t, delResp.Body, &delGot)
	ticket, _ := delGot["ticket"].(string)

	resp2 := authDo(t, "GET", base+"/users/"+userID+"/delete-status/"+ticket, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("delete-status want 200, got %d", resp2.StatusCode)
	}
	var status map[string]any
	mustDecode(t, resp2.Body, &status)
	if status["status"] != "completed" {
		t.Errorf("expected status 'completed', got %v", status["status"])
	}
}

func TestUsers_ResetPassword_Succeeds(t *testing.T) {
	base := startServer(t)
	authDo(t, "POST", base+"/user", map[string]any{
		"username": "pw@test.com",
		"email":    "pw@test.com",
		"password": "old",
	}).Body.Close()

	resp := authDo(t, "POST", base+"/user/reset-password", map[string]any{
		"email":    "pw@test.com",
		"password": "new",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("reset-password want 200, got %d", resp.StatusCode)
	}
}

func TestUsers_ResetPassword_UnknownEmail_Returns404(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "POST", base+"/user/reset-password", map[string]any{
		"email":    "ghost@test.com",
		"password": "whatever",
	})
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404 for unknown email, got %d", resp.StatusCode)
	}
}

// --- Quota tests ---

func TestQuota_ListReturnsSeed(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "GET", base+"/quota", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	if len(got) < 1 {
		t.Fatalf("expected at least 1 seed quota, got %d", len(got))
	}
}

func TestQuota_PutCreatesCustom(t *testing.T) {
	base := startServer(t)
	body := map[string]any{"source": "module.exports = { rules: [] };"}
	resp := authDo(t, "PUT", base+"/quota/tenant:newq", body)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["name"] != "tenant:newq" {
		t.Errorf("expected name 'tenant:newq', got %v", got["name"])
	}
	if got["isCustom"] != true {
		t.Errorf("expected isCustom=true after PUT, got %v", got["isCustom"])
	}
	if got["id"] == nil || got["id"] == "" {
		t.Errorf("expected id to be assigned, got %v", got["id"])
	}
}

func TestQuota_PutUpdatesExisting(t *testing.T) {
	base := startServer(t)
	authDo(t, "PUT", base+"/quota/tenant:upd", map[string]any{"source": "v1"}).Body.Close()
	resp := authDo(t, "PUT", base+"/quota/tenant:upd", map[string]any{"source": "v2"})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PUT (update) want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["source"] != "v2" {
		t.Errorf("expected source 'v2', got %v", got["source"])
	}
}

func TestQuota_PutThenAppearsInList(t *testing.T) {
	base := startServer(t)
	authDo(t, "PUT", base+"/quota/tenant:inlist", map[string]any{"source": "x"}).Body.Close()
	resp := authDo(t, "GET", base+"/quota", nil)
	defer resp.Body.Close()
	var got []map[string]any
	mustDecode(t, resp.Body, &got)
	found := false
	for _, q := range got {
		if q["name"] == "tenant:inlist" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'tenant:inlist' in quota list")
	}
}

func TestQuota_Delete_Returns200WithCount(t *testing.T) {
	base := startServer(t)
	authDo(t, "PUT", base+"/quota/tenant:dq", map[string]any{"source": "x"}).Body.Close()

	resp := authDo(t, "DELETE", base+"/quota/tenant:dq", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE want 200, got %d", resp.StatusCode)
	}
	var got map[string]any
	mustDecode(t, resp.Body, &got)
	if got["acknowledged"] != true {
		t.Errorf("expected acknowledged=true, got %v", got["acknowledged"])
	}
	if n, ok := got["deletedCount"].(float64); !ok || n != 1 {
		t.Errorf("expected deletedCount=1, got %v", got["deletedCount"])
	}
}

func TestQuota_DeleteMissing_Returns404(t *testing.T) {
	base := startServer(t)
	resp := authDo(t, "DELETE", base+"/quota/tenant:does-not-exist", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}
