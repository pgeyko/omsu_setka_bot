package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"omsu_bot/internal/db"
	"omsu_bot/internal/llm"
	"omsu_bot/internal/persona"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()

	database, err := db.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}

	if err := database.Migrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	_, err = database.Exec(`INSERT INTO groups (chat_id, title, api_token) VALUES (0, 'Test Group', 'test-api-token')`)
	if err != nil {
		t.Fatalf("failed to seed test group: %v", err)
	}

	tmpDir := t.TempDir()
	personaPath := tmpDir + "/persona.md"
	err = os.WriteFile(personaPath, []byte("# name\nTestBot\n# system_prompt\nYou are a test bot\n# signature\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write mock persona.md: %v", err)
	}

	personaStore := persona.NewStore(database.DB)
	if err := personaStore.Load(context.Background(), personaPath); err != nil {
		t.Fatalf("failed to load mock persona: %v", err)
	}

	auth := NewAuthMiddleware("test-admin-secret", "test-jwt-secret")

	promptsDir := t.TempDir()
	os.WriteFile(promptsDir+"/classify.txt", []byte("test prompt"), 0644)
	prompts, err := llm.NewPromptRegistry(promptsDir)
	if err != nil {
		t.Fatalf("failed to create prompts: %v", err)
	}

	s := NewServer(database.DB, personaStore, prompts, auth, false, "test", "*", nil, nil, nil, 0, false, 9999, 9999, 60)
	return s
}

func performRequest(s *Server, method, path, body string, token string) *http.Response {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, _ := s.App.Test(req)
	return resp
}

func getAuthToken(s *Server) string {
	resp := performRequest(s, "POST", "/api/auth/token", `{"admin_secret":"test-admin-secret"}`, "")
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	data := body["data"].(map[string]interface{})
	return data["token"].(string)
}

func TestAuthLogin_Success(t *testing.T) {
	s := setupTestServer(t)
	resp := performRequest(s, "POST", "/api/auth/token", `{"admin_secret":"test-admin-secret"}`, "")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	if body["success"] != true {
		t.Error("expected success true")
	}
}

func TestAuthLogin_WrongSecret(t *testing.T) {
	s := setupTestServer(t)
	resp := performRequest(s, "POST", "/api/auth/token", `{"admin_secret":"wrong"}`, "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_NoToken(t *testing.T) {
	s := setupTestServer(t)
	resp := performRequest(s, "GET", "/api/persona", "", "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestGetPersona(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)
	resp := performRequest(s, "GET", "/api/persona", "", token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUpdatePersona(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)
	resp := performRequest(s, "PUT", "/api/persona", `{"name":"TestBot"}`, token)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	data := body["data"].(map[string]interface{})
	if data["name"] != "TestBot" {
		t.Errorf("expected name 'TestBot', got '%v'", data["name"])
	}
}

func TestCrudTopics(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)

	// Create
	resp := performRequest(s, "POST", "/api/topics",
		`{"tg_thread_id":100,"name":"Test Topic","slug":"test-topic"}`, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("create expected 200, got %d", resp.StatusCode)
	}

	// List
	resp = performRequest(s, "GET", "/api/topics", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("list expected 200, got %d", resp.StatusCode)
	}
	var listBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&listBody)
	data := listBody["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("expected 1 topic, got %d", len(data))
	}

	// Get by ID
	resp = performRequest(s, "GET", "/api/topics/1", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("get by id expected 200, got %d", resp.StatusCode)
	}

	// Close
	resp = performRequest(s, "POST", "/api/topics/1/close", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("close expected 200, got %d", resp.StatusCode)
	}

	// Open
	resp = performRequest(s, "POST", "/api/topics/1/open", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("open expected 200, got %d", resp.StatusCode)
	}

	// Delete
	resp = performRequest(s, "DELETE", "/api/topics/1", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("delete expected 200, got %d", resp.StatusCode)
	}

	// Not found
	resp = performRequest(s, "GET", "/api/topics/999", "", token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("not found expected 404, got %d", resp.StatusCode)
	}
}

func TestPermissions(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)

	// List
	resp := performRequest(s, "GET", "/api/permissions", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Update
	resp = performRequest(s, "PUT", "/api/permissions/forward", `{"allowed_role":"admin"}`, token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// Invalid role
	resp = performRequest(s, "PUT", "/api/permissions/forward", `{"allowed_role":"invalid"}`, token)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestStats(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)

	endpoints := []string{
		"/api/stats/tokens",
		"/api/stats/requests",
		"/api/stats/forwards",
		"/api/stats/messages",
		"/api/stats/providers",
	}

	for _, ep := range endpoints {
		resp := performRequest(s, "GET", ep, "", token)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s expected 200, got %d", ep, resp.StatusCode)
		}
	}
}

func TestScheduleEndpoints(t *testing.T) {
	s := setupTestServer(t)
	token := getAuthToken(s)

	resp := performRequest(s, "GET", "/api/schedule/snapshots", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("snapshots expected 200, got %d", resp.StatusCode)
	}

	resp = performRequest(s, "GET", "/api/schedule/anomalies", "", token)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("anomalies expected 200, got %d", resp.StatusCode)
	}
}
