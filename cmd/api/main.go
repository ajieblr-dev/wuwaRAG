// Command api starts the Wuthering Waves Lore RAG REST API server.
package main

import (
	"log"

	"github.com/ajieblr-dev/wuwaRAG/internal/config"
	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
	"github.com/ajieblr-dev/wuwaRAG/internal/server"
)

func main() {
	cfg := config.Load()

	fetchClient := fetcher.NewClient(cfg.WikiAPIURL)
	qdrantClient := qdrant.NewClient(cfg.QdrantURL)
	embedder := rag.NewHuggingFaceEmbedder(
		cfg.HuggingFaceKey,
		cfg.EmbeddingModel,
		cfg.EmbeddingBaseURL,
		cfg.VectorDim,
	)
	llmClient := rag.NewLLMClient(
		cfg.LiteLLMURL,
		cfg.LiteLLMKey,
		cfg.LiteLLMModel,
		cfg.GeminiAPIKey,
		cfg.GroqAPIKey,
	)
	ragService := rag.NewService(
		qdrantClient,
		embedder,
		llmClient,
		cfg.Collection,
		cfg.TopK,
		cfg.ScoreThreshold,
	)

	srv := server.NewServer(
		ragService,
		qdrantClient,
		embedder,
		fetchClient,
		cfg.Collection,
		cfg.DefaultWikiPage,
	)

	addr := ":" + cfg.Port
	log.Printf("WuwaLore RAG API listening on %s (collection=%s)", addr, cfg.Collection)
	if err := srv.Router().Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
