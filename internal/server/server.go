package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/parser"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
)

// Server holds the dependencies for the HTTP API.
type Server struct {
	router      *gin.Engine
	ragService  *rag.Service
	qdrant      *qdrant.Client
	embedder    rag.Embedder
	fetcher     *fetcher.Client
	collection  string
	defaultPage string
}

// IngestionService encapsulates the ingestion workflow.
type IngestionService struct {
	fetcher    *fetcher.Client
	embedder   rag.Embedder
	qdrant     *qdrant.Client
	collection string
}

// NewServer initializes and configures the Gin web server.
func NewServer(
	ragService *rag.Service,
	qdrantClient *qdrant.Client,
	embedder rag.Embedder,
	fetcherClient *fetcher.Client,
	collection string,
	defaultPage string,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(corsMiddleware())

	s := &Server{
		router:      router,
		ragService:  ragService,
		qdrant:      qdrantClient,
		embedder:    embedder,
		fetcher:     fetcherClient,
		collection:  collection,
		defaultPage: defaultPage,
	}

	s.registerRoutes()
	return s
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func findWebDir() string {
	candidates := []string{
		"./web",
		"../web",
		"../../web",
		"/app/web",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
				return dir
			}
		}
	}
	return ""
}

func (s *Server) registerRoutes() {
	s.router.GET("/health", s.handleHealth)

	api := s.router.Group("/api/v1")
	{
		api.GET("/status", s.handleStatus)
		api.POST("/chat", s.handleChat)
		api.POST("/query", s.handleChat) // alias
		api.POST("/search", s.handleSearch)
		api.POST("/ingest", s.handleIngest)
	}

	// Serve Frontend if web directory is available
	webDir := findWebDir()
	if webDir != "" {
		s.router.Static("/web", webDir)
		s.router.Static("/fonts", filepath.Join(webDir, "fonts"))
		s.router.StaticFile("/", filepath.Join(webDir, "index.html"))
		s.router.StaticFile("/index.html", filepath.Join(webDir, "index.html"))
		s.router.StaticFile("/ask.html", filepath.Join(webDir, "ask.html"))
		s.router.StaticFile("/search.html", filepath.Join(webDir, "search.html"))
		s.router.StaticFile("/about.html", filepath.Join(webDir, "about.html"))
		s.router.StaticFile("/admin.html", filepath.Join(webDir, "admin.html"))
		s.router.StaticFile("/admin-overview.html", filepath.Join(webDir, "admin-overview.html"))
		s.router.StaticFile("/admin-ingest.html", filepath.Join(webDir, "admin-ingest.html"))
		s.router.StaticFile("/admin-corpus.html", filepath.Join(webDir, "admin-corpus.html"))
		s.router.StaticFile("/admin-gateway.html", filepath.Join(webDir, "admin-gateway.html"))
		s.router.StaticFile("/admin-logs.html", filepath.Join(webDir, "admin-logs.html"))
		s.router.StaticFile("/admin-settings.html", filepath.Join(webDir, "admin-settings.html"))
		s.router.StaticFile("/design-system.html", filepath.Join(webDir, "design-system.html"))
		s.router.StaticFile("/styles.css", filepath.Join(webDir, "styles.css"))
		s.router.StaticFile("/app.js", filepath.Join(webDir, "app.js"))

		s.router.NoRoute(func(c *gin.Context) {
			reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
			filePath := filepath.Join(webDir, reqPath)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
		})
	}
}

// Router returns the configured Gin engine.
func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	qdrantOk := false
	if err := s.qdrant.Ping(ctx); err == nil {
		qdrantOk = true
	}

	info, _ := s.qdrant.GetCollectionInfo(ctx, s.collection)
	pointsCount := uint64(0)
	if info != nil {
		pointsCount = info.PointsCount
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "healthy",
		"service":          "Wuthering Waves Lore RAG API",
		"qdrant_connected": qdrantOk,
		"collection":       s.collection,
		"indexed_chunks":   pointsCount,
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	info, err := s.qdrant.GetCollectionInfo(ctx, s.collection)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"collection":   s.collection,
			"status":       "not_ready_or_unreachable",
			"error":        err.Error(),
			"points_count": 0,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"collection":    s.collection,
		"status":        info.Status,
		"points_count":  info.PointsCount,
		"vectors_count": info.VectorsCount,
	})
}

type chatRequest struct {
	Question string `json:"question" binding:"required"`
}

func (s *Server) handleChat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid 'question' field"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	res, err := s.ragService.AnswerQuestion(ctx, req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to answer question: %v", err)})
		return
	}

	c.JSON(http.StatusOK, res)
}

type searchRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}

func (s *Server) handleSearch(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid 'query' field"})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	snippets, err := s.ragService.SearchOnly(ctx, req.Query, req.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Search failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"count":   len(snippets),
		"limit":   req.Limit,
		"results": snippets,
	})
}

type ingestRequest struct {
	Page      string `json:"page"`
	ChunkSize int    `json:"chunk_size"`
	Overlap   int    `json:"overlap"`
}

func (s *Server) handleIngest(c *gin.Context) {
	var req ingestRequest
	_ = c.ShouldBindJSON(&req)

	pageTitle := req.Page
	if strings.TrimSpace(pageTitle) == "" {
		pageTitle = s.defaultPage
	}
	if strings.TrimSpace(pageTitle) == "" {
		pageTitle = "Simulacrum_of_the_Heart"
	}

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 900
	}

	overlap := req.Overlap
	if overlap <= 0 {
		overlap = 150
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	start := time.Now()

	// 1. Fetch wiki page
	page, err := s.fetcher.FetchPage(ctx, pageTitle)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to fetch wiki page %q: %v", pageTitle, err)})
		return
	}

	// 2. Parse into contextual chunks
	chunks := parser.ParseIntoChunks(page.Title, page.Wikitext, chunkSize, overlap)
	if len(chunks) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No valid lore chunks could be parsed from page"})
		return
	}

	// 3. Ensure Qdrant collection
	if err := s.qdrant.EnsureCollection(ctx, s.collection, s.embedder.Dimension()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to prepare Qdrant collection: %v", err)})
		return
	}

	// 4. Generate embeddings in batches and build Qdrant points
	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Content
	}

	embeddings, err := s.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate embeddings: %v", err)})
		return
	}

	points := make([]qdrant.Point, len(chunks))
	for i, ch := range chunks {
		points[i] = qdrant.Point{
			ID:     ch.ID,
			Vector: embeddings[i],
			Payload: map[string]interface{}{
				"title":       ch.Title,
				"section":     ch.Section,
				"subsection":  ch.Subsection,
				"content":     ch.Content,
				"speakers":    ch.Speakers,
				"chunk_index": ch.Index,
				"char_count":  ch.CharCount,
			},
		}
	}

	// 5. Upsert to Qdrant
	if err := s.qdrant.UpsertPoints(ctx, s.collection, points, 50); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Failed to upsert points to Qdrant: %v", err)})
		return
	}

	duration := time.Since(start).Milliseconds()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"page":           page.Title,
		"chunks_indexed": len(chunks),
		"duration_ms":    duration,
		"collection":     s.collection,
	})
}
