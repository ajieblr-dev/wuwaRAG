// Command api starts the Wuthering Waves Lore RAG REST API server.
package main

import (
	"log"

	"github.com/ajieblr-dev/wuwaRAG/internal/config"
	"github.com/ajieblr-dev/wuwaRAG/internal/db"
	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
	"github.com/ajieblr-dev/wuwaRAG/internal/server"
)

func main() {
	cfg := config.Load()

	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to initialize sqlite database at %s: %v", cfg.DBPath, err)
	}
	defer database.Close()
	log.Printf("SQLite database initialized at %s", cfg.DBPath)

	auditDB, err := db.NewAuditDB(cfg.DevAuditDBPath)
	if err != nil {
		log.Fatalf("failed to initialize dev audit database at %s: %v", cfg.DevAuditDBPath, err)
	}
	defer auditDB.Close()
	log.Printf("Development Audit database initialized at %s", cfg.DevAuditDBPath)

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
		rag.WithHybridSearch(cfg.HybridSearchEnabled),
		rag.WithQueryRewriter(rag.NewQueryRewriter(llmClient)),
		rag.WithQueryRewriteEnabled(cfg.QueryRewriteEnabled),
		rag.WithRRFConstant(cfg.RRFConstant),
		rag.WithCandidatePoolSize(cfg.CandidatePoolSize),
	)

	srv := server.NewServer(
		ragService,
		qdrantClient,
		embedder,
		fetchClient,
		cfg.Collection,
		cfg.DefaultWikiPage,
		database,
		cfg.JWTSecret,
		cfg.DefaultQuestionLimit,
		auditDB,
	)

	addr := ":" + cfg.Port
	log.Printf("WuwaLore RAG API listening on %s (collection=%s)", addr, cfg.Collection)
	if err := srv.Router().Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
