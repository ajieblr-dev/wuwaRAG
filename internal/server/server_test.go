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
