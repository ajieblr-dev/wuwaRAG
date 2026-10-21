package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ajieblr-dev/wuwaRAG/internal/auth"
	"github.com/ajieblr-dev/wuwaRAG/internal/db"
	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
)

func TestHealthEndpoint(t *testing.T) {
	// Mock Qdrant
	qdrantMux := http.NewServeMux()
	qdrantMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	qdrantMux.HandleFunc("/collections/wuwa_lore", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"result": map[string]interface{}{
				"status":        "green",
				"points_count":  100,
				"vectors_count": 100,
			},
		}
		json.NewEncoder(w).Encode(res)
	})
	qdrantServer := httptest.NewServer(qdrantMux)
	defer qdrantServer.Close()

	qClient := qdrant.NewClient(qdrantServer.URL)
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient("http://mock-litellm", "", "", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.2)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", nil, "test-secret", 5)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", resp["status"])
	}
	if resp["qdrant_connected"] != true {
		t.Errorf("Expected qdrant_connected to be true, got %v", resp["qdrant_connected"])
	}
	if _, ok := resp["litellm_connected"]; !ok {
		t.Errorf("Expected litellm_connected key in /health response")
	}
}

func TestCorpusEndpoints(t *testing.T) {
	qClient := qdrant.NewClient("http://mock-qdrant")
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient("http://mock-litellm", "", "", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.2)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", nil, "test-secret", 5)

	// Test GET /api/v1/corpus
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/corpus", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/corpus, got %d", w.Code)
	}

	var corpusResp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&corpusResp); err != nil {
		t.Fatalf("Failed to decode /api/v1/corpus response: %v", err)
	}
	if _, ok := corpusResp["total_pages"]; !ok {
		t.Errorf("Expected total_pages in corpus response")
	}
	if _, ok := corpusResp["total_chunks"]; !ok {
		t.Errorf("Expected total_chunks in corpus response")
	}

	// Test GET /api/v1/pages
	reqPages, _ := http.NewRequest(http.MethodGet, "/api/v1/pages", nil)
	wPages := httptest.NewRecorder()
	srv.Router().ServeHTTP(wPages, reqPages)
	if wPages.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/pages, got %d", wPages.Code)
	}

	// Test GET /api/v1/corpus?q=Abraxas
	reqQ, _ := http.NewRequest(http.MethodGet, "/api/v1/corpus?q=Abraxas", nil)
	wQ := httptest.NewRecorder()
	srv.Router().ServeHTTP(wQ, reqQ)
	if wQ.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/corpus?q=Abraxas, got %d", wQ.Code)
	}
	var qResp map[string]interface{}
	if err := json.NewDecoder(wQ.Body).Decode(&qResp); err != nil {
		t.Fatalf("Failed to decode /api/v1/corpus?q=Abraxas response: %v", err)
	}
	matchedCount, _ := qResp["matched_chunks"].(float64)
	if matchedCount < 2 {
		t.Errorf("Expected at least 2 matched chunks for 'Abraxas', got %v", matchedCount)
	}

	// Test GET /api/v1/speakers
	reqSpeakers, _ := http.NewRequest(http.MethodGet, "/api/v1/speakers", nil)
	wSpeakers := httptest.NewRecorder()
	srv.Router().ServeHTTP(wSpeakers, reqSpeakers)
	if wSpeakers.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for /api/v1/speakers, got %d", wSpeakers.Code)
	}
}

func TestStaticWebRoutes(t *testing.T) {
	qClient := qdrant.NewClient("http://mock-qdrant")
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient("http://mock-litellm", "", "", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.2)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", nil, "test-secret", 5)

	routes := []string{"/", "/index.html", "/ask.html", "/search.html", "/styles.css", "/app.js", "/console", "/admin", "/console.html", "/admin.html"}
	for _, route := range routes {
		req, _ := http.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently {
			t.Errorf("Expected route %q to return 200 or 301, got %d", route, w.Code)
		}
	}
}

func setupFullTestServer(t *testing.T) (*Server, *db.DB, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "server_full_test.db")
	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	jwtSecret := "integration-test-secret-key-1234"
	qClient := qdrant.NewClient("http://mock-qdrant")
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient("http://mock-litellm", "", "", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.2)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", database, jwtSecret, 5)
	return srv, database, jwtSecret
}

func TestAuthEndpoints(t *testing.T) {
	srv, _, _ := setupFullTestServer(t)

	// 1. Register valid user
	regPayload := map[string]string{
		"username": "rover_tester",
		"email":    "rover@tester.com",
		"password": "strongpassword123",
	}
	body, _ := json.Marshal(regPayload)
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Register expected 201 Created, got %d (%s)", w.Code, w.Body.String())
	}

	var regResp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &regResp)
	token, ok := regResp["token"].(string)
	if !ok || token == "" {
		t.Fatalf("Register did not return token: %v", regResp)
	}

	// 2. Duplicate registration -> 409 Conflict
	reqDup, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	srv.Router().ServeHTTP(wDup, reqDup)
	if wDup.Code != http.StatusConflict {
		t.Fatalf("Duplicate register expected 409 Conflict, got %d", wDup.Code)
	}

	// 3. Login success
	loginPayload := map[string]string{
		"username": "rover_tester",
		"password": "strongpassword123",
	}
	lBody, _ := json.Marshal(loginPayload)
	reqLogin, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(lBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()
	srv.Router().ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("Login expected 200 OK, got %d", wLogin.Code)
	}

	// 4. Login wrong password -> 401
	badLoginPayload := map[string]string{
		"username": "rover_tester",
		"password": "wrongpassword",
	}
	blBody, _ := json.Marshal(badLoginPayload)
	reqBadLogin, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(blBody))
	reqBadLogin.Header.Set("Content-Type", "application/json")
	wBadLogin := httptest.NewRecorder()
	srv.Router().ServeHTTP(wBadLogin, reqBadLogin)
	if wBadLogin.Code != http.StatusUnauthorized {
		t.Fatalf("Login with bad password expected 401, got %d", wBadLogin.Code)
	}

	// 5. Auth /me endpoint
	reqMe, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	wMe := httptest.NewRecorder()
	srv.Router().ServeHTTP(wMe, reqMe)
	if wMe.Code != http.StatusOK {
		t.Fatalf("Auth /me expected 200 OK, got %d", wMe.Code)
	}
	var meResp map[string]interface{}
	_ = json.Unmarshal(wMe.Body.Bytes(), &meResp)
	userObj := meResp["user"].(map[string]interface{})
	if userObj["username"] != "rover_tester" {
		t.Errorf("Expected username rover_tester, got %v", userObj["username"])
	}
	if userObj["questions_remaining"].(float64) != 5 {
		t.Errorf("Expected 5 questions remaining, got %v", userObj["questions_remaining"])
	}
}

func TestChatQuotaAndThreadPersistence(t *testing.T) {
	// Setup mock Qdrant and LiteLLM
	qdrantMux := http.NewServeMux()
	qdrantMux.HandleFunc("/collections/wuwa_lore/points/search", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"id":      "chunk-1",
					"score":   0.9,
					"payload": map[string]interface{}{"title": "Solaris Lore", "content": "The Lament fell..."},
				},
			},
			"status": "ok",
		}
		_ = json.NewEncoder(w).Encode(res)
	})
	qdrantServer := httptest.NewServer(qdrantMux)
	defer qdrantServer.Close()

	llmMux := http.NewServeMux()
	llmMux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"model": "wuwa-lore-model",
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"role": "assistant", "content": "The Lament occurred during..."}},
			},
			"usage": map[string]interface{}{"total_tokens": 80},
		}
		_ = json.NewEncoder(w).Encode(res)
	})
	llmServer := httptest.NewServer(llmMux)
	defer llmServer.Close()

	tmpDir := t.TempDir()
	database, _ := db.New(filepath.Join(tmpDir, "chat_quota.db"))
	defer database.Close()

	jwtSecret := "test-secret"
	qClient := qdrant.NewClient(qdrantServer.URL)
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient(llmServer.URL, "dummy-key", "wuwa-lore-model", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.1)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", database, jwtSecret, 5)

	// Create user with limit 5
	user, _ := database.CreateUser(context.Background(), "limit_user", "limit@solaris.org", "pwd", "user", 5)
	userToken, _ := auth.GenerateToken(jwtSecret, user.ID, user.Username, user.Role, 0)

	// 1. Unauthenticated /chat -> 401
	chatPayload := map[string]string{"question": "What is the Lament?"}
	cBody, _ := json.Marshal(chatPayload)
	reqNoAuth, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(cBody))
	reqNoAuth.Header.Set("Content-Type", "application/json")
	wNoAuth := httptest.NewRecorder()
	srv.Router().ServeHTTP(wNoAuth, reqNoAuth)
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("Chat without token expected 401, got %d", wNoAuth.Code)
	}

	// 2. Ask 5 questions with user token -> all succeed, quota decrements
	for i := 1; i <= 5; i++ {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(cBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+userToken)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Chat request %d failed: %d (%s)", i, w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		rem := int(resp["questions_remaining"].(float64))
		if rem != 5-i {
			t.Errorf("Iteration %d: expected remaining %d, got %d", i, 5-i, rem)
		}
	}

	// 3. 6th question -> blocked with 403 Forbidden
	req6, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(cBody))
	req6.Header.Set("Content-Type", "application/json")
	req6.Header.Set("Authorization", "Bearer "+userToken)
	w6 := httptest.NewRecorder()
	srv.Router().ServeHTTP(w6, req6)
	if w6.Code != http.StatusForbidden {
		t.Fatalf("6th chat request expected 403 Forbidden, got %d (%s)", w6.Code, w6.Body.String())
	}
	var errResp map[string]interface{}
	_ = json.Unmarshal(w6.Body.Bytes(), &errResp)
	if errResp["error"] != "question_limit_reached" {
		t.Errorf("Expected question_limit_reached, got %v", errResp["error"])
	}

	// 4. Verify thread and messages were saved in database
	threads, err := database.ListThreads(context.Background(), user.ID)
	if err != nil || len(threads) == 0 {
		t.Fatalf("Expected saved threads for user, got error: %v, count: %d", err, len(threads))
	}
	_, msgs, err := database.GetThreadWithMessages(context.Background(), user.ID, threads[0].ID)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("Expected saved messages in thread, got error: %v, count: %d", err, len(msgs))
	}
}

func TestAdminPrivilegesAndEndpointProtection(t *testing.T) {
	srv, database, jwtSecret := setupFullTestServer(t)

	// Create user and admin
	user, _ := database.CreateUser(context.Background(), "regular_user", "reg@test.com", "pwd", "user", 5)
	userToken, _ := auth.GenerateToken(jwtSecret, user.ID, user.Username, user.Role, 0)

	admin, _ := database.CreateUser(context.Background(), "sys_admin", "admin@test.com", "pwd", "admin", 5)
	adminToken, _ := auth.GenerateToken(jwtSecret, admin.ID, admin.Username, admin.Role, 0)

	// 1. Regular user accessing /api/v1/ingest -> 403 Forbidden
	reqIngestUser, _ := http.NewRequest(http.MethodPost, "/api/v1/ingest", bytes.NewReader([]byte("{}")))
	reqIngestUser.Header.Set("Content-Type", "application/json")
	reqIngestUser.Header.Set("Authorization", "Bearer "+userToken)
	wIngestUser := httptest.NewRecorder()
	srv.Router().ServeHTTP(wIngestUser, reqIngestUser)
	if wIngestUser.Code != http.StatusForbidden {
		t.Fatalf("Regular user on /api/v1/ingest expected 403, got %d", wIngestUser.Code)
	}

	// 2. Regular user accessing /api/v1/admin/users -> 403 Forbidden
	reqAdminUsers, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	reqAdminUsers.Header.Set("Authorization", "Bearer "+userToken)
	wAdminUsers := httptest.NewRecorder()
	srv.Router().ServeHTTP(wAdminUsers, reqAdminUsers)
	if wAdminUsers.Code != http.StatusForbidden {
		t.Fatalf("Regular user on /api/v1/admin/users expected 403, got %d", wAdminUsers.Code)
	}

	// 3. Admin accessing /api/v1/admin/users -> 200 OK
	reqAdminOk, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	reqAdminOk.Header.Set("Authorization", "Bearer "+adminToken)
	wAdminOk := httptest.NewRecorder()
	srv.Router().ServeHTTP(wAdminOk, reqAdminOk)
	if wAdminOk.Code != http.StatusOK {
		t.Fatalf("Admin on /api/v1/admin/users expected 200 OK, got %d", wAdminOk.Code)
	}
	var usersResp map[string]interface{}
	_ = json.Unmarshal(wAdminOk.Body.Bytes(), &usersResp)
	usersList := usersResp["users"].([]interface{})
	if len(usersList) != 2 {
		t.Errorf("Expected 2 users in admin list, got %d", len(usersList))
	}

	// 4. Admin resetting user quota -> 200 OK
	resetBody, _ := json.Marshal(map[string]int{"count": 0, "limit": 10})
	reqReset, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/admin/users/%s/reset", user.ID), bytes.NewReader(resetBody))
	reqReset.Header.Set("Content-Type", "application/json")
	reqReset.Header.Set("Authorization", "Bearer "+adminToken)
	wReset := httptest.NewRecorder()
	srv.Router().ServeHTTP(wReset, reqReset)
	if wReset.Code != http.StatusOK {
		t.Fatalf("Admin reset quota expected 200, got %d", wReset.Code)
	}

	// Verify user now has 10 question limit and 0 count
	updatedUser, _ := database.GetUserByID(context.Background(), user.ID)
	if updatedUser.QuestionLimit != 10 || updatedUser.QuestionCount != 0 {
		t.Errorf("User quota not reset: count=%d, limit=%d", updatedUser.QuestionCount, updatedUser.QuestionLimit)
	}
}
