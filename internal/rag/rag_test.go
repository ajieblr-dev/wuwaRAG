package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
)

func TestDeterministicEmbedder(t *testing.T) {
	embedder := NewHuggingFaceEmbedder("", "all-MiniLM-L6-v2", "", 384)
	vec, err := embedder.Embed(context.Background(), "Sentinel Hsin and the Waking Moon Festival")
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(vec) != 384 {
		t.Fatalf("Expected 384 dimensions, got %d", len(vec))
	}

	// Verify normalization
	var sumSq float64
	for _, val := range vec {
		sumSq += float64(val * val)
	}
	if sumSq < 0.99 || sumSq > 1.01 {
		t.Errorf("Expected normalized vector magnitude ~1.0, got %f", sumSq)
	}
}

func TestRAGService(t *testing.T) {
	// Mock Qdrant server
	qdrantMux := http.NewServeMux()
	qdrantMux.HandleFunc("/collections/wuwa_lore/points/search", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"id":      "chunk-1",
					"version": 0,
					"score":   0.88,
					"payload": map[string]interface{}{
						"title":      "Simulacrum of the Heart",
						"section":    "Dialogue",
						"subsection": "Talk to tea stall owner",
						"speakers":   []string{"Rover", "Tea Stall Owner"},
						"content":    "Tea Stall Owner: That'd be our new Moon Fox Rice Cakes, no question. They're Lady Hsin's favorite treat!",
					},
				},
			},
			"status": "ok",
		}
		json.NewEncoder(w).Encode(res)
	})
	qdrantServer := httptest.NewServer(qdrantMux)
	defer qdrantServer.Close()

	// Mock LiteLLM proxy server
	llmMux := http.NewServeMux()
	llmMux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		res := map[string]interface{}{
			"model": "wuwa-lore-model",
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Lady Hsin's favorite treat is Moon Fox Rice Cakes, as stated by the Tea Stall Owner in Xianling Village.",
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": 120,
				"total_tokens":  150,
			},
		}
		json.NewEncoder(w).Encode(res)
	})
	llmServer := httptest.NewServer(llmMux)
	defer llmServer.Close()

	qClient := qdrant.NewClient(qdrantServer.URL)
	embedder := NewHuggingFaceEmbedder("", "", "", 384)
	llmClient := NewLLMClient(llmServer.URL, "dummy-key", "wuwa-lore-model", "", "")

	svc := NewService(qClient, embedder, llmClient, "wuwa_lore", 3, 0.1)

	res, err := svc.AnswerQuestion(context.Background(), "What is Lady Hsin's favorite treat?")
	if err != nil {
		t.Fatalf("AnswerQuestion failed: %v", err)
	}

	if len(res.Sources) != 1 {
		t.Errorf("Expected 1 source snippet, got %d", len(res.Sources))
	}
	if res.Model != "wuwa-lore-model" {
		t.Errorf("Expected model 'wuwa-lore-model', got %s", res.Model)
	}
	if res.Answer == "" {
		t.Errorf("Expected non-empty answer")
	}
}

func TestExtractHeuristicKeywords(t *testing.T) {
	query := "What did Carlotta find in the Averardo Vault about the Carnevale?"
	kws := ExtractHeuristicKeywords(query)

	hasCarlotta := false
	hasVault := false
	for _, kw := range kws {
		if strings.Contains(strings.ToLower(kw), "carlotta") {
			hasCarlotta = true
		}
		if strings.Contains(strings.ToLower(kw), "vault") || strings.Contains(strings.ToLower(kw), "averardo") {
			hasVault = true
		}
	}

	if !hasCarlotta {
		t.Errorf("Expected keywords to extract 'Carlotta', got %v", kws)
	}
	if !hasVault {
		t.Errorf("Expected keywords to extract 'Averardo' or 'Vault', got %v", kws)
	}
}

func TestReciprocalRankFusion(t *testing.T) {
	vecPoints := []qdrant.ScoredPoint{
		{
			ID:    "doc-1",
			Score: 0.9,
			Payload: map[string]interface{}{
				"title":   "Doc One",
				"content": "General lore discussion",
			},
		},
		{
			ID:    "doc-2",
			Score: 0.7,
			Payload: map[string]interface{}{
				"title":   "Doc Two",
				"content": "Specific mention of Sentinel Jué",
			},
		},
	}

	lexPoints := []qdrant.ScoredPoint{
		{
			ID: "doc-2", // doc-2 matches exact keyword!
			Payload: map[string]interface{}{
				"title":    "Doc Two",
				"speakers": []interface{}{"Sentinel Jué"},
				"content":  "Specific mention of Sentinel Jué and Mianloong Chamber",
			},
		},
		{
			ID: "doc-3",
			Payload: map[string]interface{}{
				"title":   "Doc Three",
				"content": "Sentinel Jué resting place",
			},
		},
	}

	keywords := []string{"Sentinel Jué", "Mianloong Chamber"}
	fused := ReciprocalRankFusion(vecPoints, lexPoints, keywords, 60, 5)

	if len(fused) != 3 {
		t.Fatalf("Expected 3 fused documents, got %d", len(fused))
	}

	// doc-2 was in BOTH vector and keyword lists, so it must be ranked #1
	if fused[0].ID != "doc-2" {
		t.Errorf("Expected doc-2 to be ranked #1 by RRF, got %s", fused[0].ID)
	}
}

func TestLexicalKeywordDominance(t *testing.T) {
	// A generic document with high vector score but zero keyword match
	vecPoints := []qdrant.ScoredPoint{
		{
			ID:    "unrelated-generic-doc",
			Score: 0.85,
			Payload: map[string]interface{}{
				"title":   "General Academy Lore",
				"content": "This is a generic research institute in Huanglong with state-run branches.",
			},
		},
	}

	// An exact entity match document retrieved via keyword scroll
	lexPoints := []qdrant.ScoredPoint{
		{
			ID:    "abraxas-chunk",
			Score: 0.0,
			Payload: map[string]interface{}{
				"title":   "Dawn Breaks on Dark Tides",
				"content": "Levianthan: Abra... Abraxas! Why hast thou betrayed us?",
			},
		},
	}

	keywords := []string{"Abraxas"}
	fused := ReciprocalRankFusion(vecPoints, lexPoints, keywords, 60, 5)

	if len(fused) < 2 {
		t.Fatalf("Expected 2 documents, got %d", len(fused))
	}

	// The exact keyword match must be ranked #1 because of the lexical keyword boost
	if fused[0].ID != "abraxas-chunk" {
		t.Errorf("Expected abraxas-chunk to be ranked #1 due to exact keyword presence, got %s (score %f vs %f)",
			fused[0].ID, fused[0].Score, fused[1].Score)
	}
}

