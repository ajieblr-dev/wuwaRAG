package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart")

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

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart")

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

	srv := NewServer(ragService, qClient, embedder, fetchClient, "wuwa_lore", "Simulacrum_of_the_Heart")

	routes := []string{"/", "/index.html", "/ask.html", "/search.html", "/styles.css", "/app.js"}
	for _, route := range routes {
		req, _ := http.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently {
			t.Errorf("Expected route %q to return 200 or 301, got %d", route, w.Code)
		}
	}
}
