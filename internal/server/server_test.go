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

func TestChatWithNewOrNonExistentThreadIDCreatesThreadAndPersistsMessages(t *testing.T) {
	qdrantMux := http.NewServeMux()
	qdrantMux.HandleFunc("/collections/wuwa_lore/points/search", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"id":      "chunk-1",
					"score":   0.9,
					"payload": map[string]interface{}{"title": "Mount Firmament", "content": "Mount Firmament is situated..."},
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
				{"message": map[string]interface{}{"role": "assistant", "content": "Mount Firmament is in the southeastern waters."}},
			},
			"usage": map[string]interface{}{"total_tokens": 55},
		}
		_ = json.NewEncoder(w).Encode(res)
	})
	llmServer := httptest.NewServer(llmMux)
	defer llmServer.Close()

	tmpDir := t.TempDir()
	database, _ := db.New(filepath.Join(tmpDir, "chat_new_thread.db"))
	defer database.Close()

	jwtSecret := "test-secret"
	qClient := qdrant.NewClient(qdrantServer.URL)
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient(llmServer.URL, "dummy-key", "wuwa-lore-model", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.1)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", database, jwtSecret, 5)

	user, err := database.CreateUser(context.Background(), "session_user", "session@solaris.org", "pwd", "admin", -1)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	token, _ := auth.GenerateToken(jwtSecret, user.ID, user.Username, user.Role, 0)

	// User sends a question with a client-generated fake thread ID (e.g. from starting a new topic)
	clientFakeID := "th-client-temporary-session-123"
	chatPayload, _ := json.Marshal(map[string]interface{}{
		"question":  "Where is Mount Firmament?",
		"thread_id": clientFakeID,
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(chatPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	returnedThreadID, ok := resp["thread_id"].(string)
	if !ok || returnedThreadID == "" {
		t.Fatalf("Expected valid thread_id in chat response, got %v", resp["thread_id"])
	}

	// Verify that the thread actually exists in the database
	threads, err := database.ListThreads(context.Background(), user.ID)
	if err != nil || len(threads) == 0 {
		t.Fatalf("Expected thread to be persisted in database, got %v (len: %d)", err, len(threads))
	}
	if threads[0].ID != returnedThreadID {
		t.Errorf("Expected database thread ID %s, got %s", returnedThreadID, threads[0].ID)
	}

	// Verify that both user and assistant messages were saved in the thread
	_, msgs, err := database.GetThreadWithMessages(context.Background(), user.ID, returnedThreadID)
	if err != nil {
		t.Fatalf("Failed to get thread with messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages (user + assistant), got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Where is Mount Firmament?" {
		t.Errorf("Unexpected user message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content == "" {
		t.Errorf("Unexpected assistant message: %+v", msgs[1])
	}

	// Verify GET /api/v1/threads HTTP endpoint returns this thread
	reqList, _ := http.NewRequest(http.MethodGet, "/api/v1/threads", nil)
	reqList.Header.Set("Authorization", "Bearer "+token)
	wList := httptest.NewRecorder()
	srv.Router().ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("Expected GET /api/v1/threads 200, got %d", wList.Code)
	}
	var listResp map[string][]interface{}
	_ = json.Unmarshal(wList.Body.Bytes(), &listResp)
	if len(listResp["threads"]) != 1 {
		t.Errorf("Expected 1 thread in GET /api/v1/threads response, got %d", len(listResp["threads"]))
	}
}

func TestClearAllThreadsAndAuditIsolation(t *testing.T) {
	qdrantMux := http.NewServeMux()
	qdrantMux.HandleFunc("/collections/wuwa_lore/points/search", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"result": []map[string]interface{}{
				{"id": "chunk-1", "score": 0.9, "payload": map[string]interface{}{"title": "Jinzhou", "content": "Jinzhou city..."}},
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
				{"message": map[string]interface{}{"role": "assistant", "content": "Jinzhou is a city of Huanglong."}},
			},
			"usage": map[string]interface{}{"total_tokens": 60},
		}
		_ = json.NewEncoder(w).Encode(res)
	})
	llmServer := httptest.NewServer(llmMux)
	defer llmServer.Close()

	tmpDir := t.TempDir()
	database, _ := db.New(filepath.Join(tmpDir, "chat_user.db"))
	defer database.Close()

	auditDatabase, _ := db.NewAuditDB(filepath.Join(tmpDir, "chat_dev_audit.db"))
	defer auditDatabase.Close()

	jwtSecret := "test-secret"
	qClient := qdrant.NewClient(qdrantServer.URL)
	embedder := rag.NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := rag.NewLLMClient(llmServer.URL, "dummy-key", "wuwa-lore-model", "", "")
	ragService := rag.NewService(qClient, embedder, llmClient, "wuwa_lore", 5, 0.1)
	fetchClient := fetcher.NewClient("")

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart", database, jwtSecret, 5, auditDatabase)

	user, _ := database.CreateUser(context.Background(), "cleartester", "clear@solaris.org", "pwd", "user", 5)
	token, _ := auth.GenerateToken(jwtSecret, user.ID, user.Username, user.Role, 0)

	// 1. User asks question -> saved in database AND in audit database
	chatPayload, _ := json.Marshal(map[string]interface{}{
		"question": "Tell me about Jinzhou",
	})
	reqChat, _ := http.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(chatPayload))
	reqChat.Header.Set("Content-Type", "application/json")
	reqChat.Header.Set("Authorization", "Bearer "+token)
	wChat := httptest.NewRecorder()
	srv.Router().ServeHTTP(wChat, reqChat)
	if wChat.Code != http.StatusOK {
		t.Fatalf("Expected chat 200, got %d", wChat.Code)
	}

	// Verify thread exists in user database
	uThreads, _ := database.ListThreads(context.Background(), user.ID)
	if len(uThreads) != 1 {
		t.Fatalf("Expected 1 thread in user database, got %d", len(uThreads))
	}

	// Verify query exists in audit database
	auditLogs, totalAudit, _ := auditDatabase.ListQueryLogs(context.Background(), 10, 0, "", "")
	if totalAudit != 1 || len(auditLogs) != 1 {
		t.Fatalf("Expected 1 log in audit database, got total=%d len=%d", totalAudit, len(auditLogs))
	}

	// 2. User clears chat history via DELETE /api/v1/threads
	reqClear, _ := http.NewRequest(http.MethodDelete, "/api/v1/threads", nil)
	reqClear.Header.Set("Authorization", "Bearer "+token)
	wClear := httptest.NewRecorder()
	srv.Router().ServeHTTP(wClear, reqClear)
	if wClear.Code != http.StatusOK {
		t.Fatalf("Expected DELETE /api/v1/threads 200, got %d: %s", wClear.Code, wClear.Body.String())
	}

	// Verify user database threads are now EMPTY
	uThreadsAfter, _ := database.ListThreads(context.Background(), user.ID)
	if len(uThreadsAfter) != 0 {
		t.Errorf("Expected 0 threads in user database after clear, got %d", len(uThreadsAfter))
	}

	// CRITICAL REQUIREMENT 2 VERIFICATION:
	// Verify that the development audit database STILL RETAINS the question!
	auditLogsAfter, totalAuditAfter, _ := auditDatabase.ListQueryLogs(context.Background(), 10, 0, "", "")
	if totalAuditAfter != 1 || len(auditLogsAfter) != 1 {
		t.Errorf("Audit database was unexpectedly cleared! Expected 1 preserved query, got %d", totalAuditAfter)
	}
	if auditLogsAfter[0].Question != "Tell me about Jinzhou" {
		t.Errorf("Audit record content mismatch: %s", auditLogsAfter[0].Question)
	}
}

func TestAdminUserHistoryEndpoints(t *testing.T) {
	srv, database, jwtSecret := setupFullTestServer(t)

	tmpDir := t.TempDir()
	auditDatabase, _ := db.NewAuditDB(filepath.Join(tmpDir, "audit_admin_test.db"))
	defer auditDatabase.Close()

	// Seed audit database with a query log
	_, _ = auditDatabase.LogQuery(context.Background(), db.UserQueryLog{
		UserID:    "user-123",
		Username:  "rover",
		SessionID: "th-abc",
		Question:  "What is Tacet Discord?",
		Tokens:    150,
		LatencyMs: 950,
		Status:    "success",
	})

	srv.auditDB = auditDatabase

	user, _ := database.CreateUser(context.Background(), "reg_user", "reg@test.com", "pwd", "user", 5)
	userToken, _ := auth.GenerateToken(jwtSecret, user.ID, user.Username, user.Role, 0)

	admin, _ := database.CreateUser(context.Background(), "admin_user", "admin@test.com", "pwd", "admin", -1)
	adminToken, _ := auth.GenerateToken(jwtSecret, admin.ID, admin.Username, admin.Role, 0)

	// 1. Regular user accessing /api/v1/admin/history -> 403 Forbidden
	reqUserHist, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/history", nil)
	reqUserHist.Header.Set("Authorization", "Bearer "+userToken)
	wUserHist := httptest.NewRecorder()
	srv.Router().ServeHTTP(wUserHist, reqUserHist)
	if wUserHist.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for regular user, got %d", wUserHist.Code)
	}

	// 2. Admin user accessing /api/v1/admin/history -> 200 OK
	reqAdminHist, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/history?limit=10&status=success", nil)
	reqAdminHist.Header.Set("Authorization", "Bearer "+adminToken)
	wAdminHist := httptest.NewRecorder()
	srv.Router().ServeHTTP(wAdminHist, reqAdminHist)
	if wAdminHist.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for admin history, got %d: %s", wAdminHist.Code, wAdminHist.Body.String())
	}

	var histResp map[string]interface{}
	_ = json.Unmarshal(wAdminHist.Body.Bytes(), &histResp)
	total := int(histResp["total"].(float64))
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}

	// 3. Admin user accessing /api/v1/admin/history/stats -> 200 OK
	reqStats, _ := http.NewRequest(http.MethodGet, "/api/v1/admin/history/stats", nil)
	reqStats.Header.Set("Authorization", "Bearer "+adminToken)
	wStats := httptest.NewRecorder()
	srv.Router().ServeHTTP(wStats, reqStats)
	if wStats.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK for admin history stats, got %d", wStats.Code)
	}

	var statsResp map[string]interface{}
	_ = json.Unmarshal(wStats.Body.Bytes(), &statsResp)
	if int(statsResp["total_queries"].(float64)) != 1 {
		t.Errorf("Expected 1 query in stats, got %v", statsResp["total_queries"])
	}
}
