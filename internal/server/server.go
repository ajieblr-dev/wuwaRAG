package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ajieblr-dev/wuwaRAG/internal/auth"
	"github.com/ajieblr-dev/wuwaRAG/internal/db"
	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/parser"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
)

// CorpusPage represents per-page statistics.
type CorpusPage struct {
	Title  string `json:"title"`
	Chars  int    `json:"chars"`
	Chunks int    `json:"chunks"`
}

// SpeakerStat represents per-speaker frequency in dialogue.
type SpeakerStat struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CorpusData caches aggregated statistics from processed chunks.
type CorpusData struct {
	TotalPages    int                       `json:"total_pages"`
	TotalChunks   int                       `json:"total_chunks"`
	TotalSpeakers int                       `json:"total_speakers"`
	Pages         []CorpusPage              `json:"pages"`
	TopSpeakers   []SpeakerStat             `json:"top_speakers"`
	Chunks        []parser.Chunk            `json:"chunks,omitempty"`
	ChunksByPage  map[string][]parser.Chunk `json:"-"`
}

// Server holds the dependencies for the HTTP API.
type Server struct {
	router               *gin.Engine
	ragService           *rag.Service
	qdrant               *qdrant.Client
	embedder             rag.Embedder
	fetcher              *fetcher.Client
	collection           string
	defaultPage          string
	corpusMu             sync.RWMutex
	corpus               *CorpusData
	db                   *db.DB
	jwtSecret            string
	defaultQuestionLimit int
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
	database *db.DB,
	jwtSecret string,
	defaultQuestionLimit int,
) *Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(corsMiddleware())

	if jwtSecret == "" {
		jwtSecret = "wuwa-rag-local-dev-secret-change-in-prod"
	}
	if defaultQuestionLimit <= 0 {
		defaultQuestionLimit = 5
	}

	s := &Server{
		router:               router,
		ragService:           ragService,
		qdrant:               qdrantClient,
		embedder:             embedder,
		fetcher:              fetcherClient,
		collection:           collection,
		defaultPage:          defaultPage,
		db:                   database,
		jwtSecret:            jwtSecret,
		defaultQuestionLimit: defaultQuestionLimit,
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

func findWebRedesignDir() string {
	candidates := []string{
		"./web_redesign",
		"../web_redesign",
		"../../web_redesign",
		"/app/web_redesign",
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
		api.GET("/corpus", s.handleCorpus)
		api.GET("/pages", s.handlePages)
		api.GET("/speakers", s.handleSpeakers)

		// Public authentication
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", s.handleRegister)
			authGroup.POST("/login", s.handleLogin)
			authGroup.GET("/me", auth.AuthRequired(s.jwtSecret), s.handleAuthMe)
		}

		// Search routes (require authentication)
		api.POST("/search", auth.AuthRequired(s.jwtSecret), s.handleSearch)

		// Chat routes (require authentication)
		api.POST("/chat", auth.AuthRequired(s.jwtSecret), s.handleChat)
		api.POST("/query", auth.AuthRequired(s.jwtSecret), s.handleChat) // alias

		// Conversation threads (require authentication)
		threadsGroup := api.Group("/threads", auth.AuthRequired(s.jwtSecret))
		{
			threadsGroup.GET("", s.handleListThreads)
			threadsGroup.POST("", s.handleCreateThread)
			threadsGroup.GET("/:id", s.handleGetThread)
			threadsGroup.DELETE("/:id", s.handleDeleteThread)
		}

		// Protected Ingestion (admin only)
		api.POST("/ingest", auth.AdminRequired(s.jwtSecret), s.handleIngest)

		// Admin management
		adminGroup := api.Group("/admin", auth.AdminRequired(s.jwtSecret))
		{
			adminGroup.GET("/users", s.handleAdminListUsers)
			adminGroup.POST("/users/:id/reset", s.handleAdminResetQuota)
		}
	}

	// Discover directories
	redesignDir := findWebRedesignDir()
	webDir := findWebDir()

	// 1. Serve Admin Console and Classic Prototypes from webDir
	if webDir != "" {
		s.router.Static("/web", webDir)
		s.router.Static("/classic", webDir)
		s.router.Static("/fonts", filepath.Join(webDir, "fonts"))
		s.router.StaticFile("/admin.html", filepath.Join(webDir, "admin.html"))
		s.router.StaticFile("/admin-overview.html", filepath.Join(webDir, "admin-overview.html"))
		s.router.StaticFile("/admin-ingest.html", filepath.Join(webDir, "admin-ingest.html"))
		s.router.StaticFile("/admin-corpus.html", filepath.Join(webDir, "admin-corpus.html"))
		s.router.StaticFile("/admin-gateway.html", filepath.Join(webDir, "admin-gateway.html"))
		s.router.StaticFile("/admin-logs.html", filepath.Join(webDir, "admin-logs.html"))
		s.router.StaticFile("/admin-settings.html", filepath.Join(webDir, "admin-settings.html"))
		s.router.StaticFile("/design-system.html", filepath.Join(webDir, "design-system.html"))
		s.router.StaticFile("/console.html", filepath.Join(webDir, "admin-overview.html"))

		consoleHandler := func(c *gin.Context) {
			c.File(filepath.Join(webDir, "admin-overview.html"))
		}
		s.router.GET("/console", consoleHandler)
		s.router.GET("/admin", consoleHandler)
	}

	// 2. Serve Primary User Routes: prefer redesignDir if available, else webDir
	primaryDir := redesignDir
	if primaryDir == "" {
		primaryDir = webDir
	}

	if primaryDir != "" {
		s.router.StaticFile("/", filepath.Join(primaryDir, "index.html"))
		s.router.StaticFile("/index.html", filepath.Join(primaryDir, "index.html"))
		s.router.StaticFile("/ask.html", filepath.Join(primaryDir, "ask.html"))
		s.router.StaticFile("/search.html", filepath.Join(primaryDir, "search.html"))
		s.router.StaticFile("/about.html", filepath.Join(primaryDir, "about.html"))
		s.router.StaticFile("/styles.css", filepath.Join(primaryDir, "styles.css"))
		s.router.StaticFile("/app.js", filepath.Join(primaryDir, "app.js"))
	}

	// 3. Keep explicit /redesign routes
	if redesignDir != "" {
		s.router.Static("/redesign", redesignDir)
		s.router.Static("/web_redesign", redesignDir)
		s.router.GET("/redesign", func(c *gin.Context) {
			c.File(filepath.Join(redesignDir, "index.html"))
		})
	}

	// 4. Fallback route handler
	s.router.NoRoute(func(c *gin.Context) {
		cleanPath := strings.Trim(c.Request.URL.Path, "/")
		if (cleanPath == "console" || cleanPath == "admin" || cleanPath == "console.html") && webDir != "" {
			c.File(filepath.Join(webDir, "admin-overview.html"))
			return
		}

		reqPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if primaryDir != "" {
			pPath := filepath.Join(primaryDir, reqPath)
			if info, err := os.Stat(pPath); err == nil {
				if !info.IsDir() {
					c.File(pPath)
					return
				}
				idx := filepath.Join(pPath, "index.html")
				if _, err := os.Stat(idx); err == nil {
					c.File(idx)
					return
				}
			}
		}
		if webDir != "" && webDir != primaryDir {
			wPath := filepath.Join(webDir, reqPath)
			if info, err := os.Stat(wPath); err == nil {
				if !info.IsDir() {
					c.File(wPath)
					return
				}
				idx := filepath.Join(wPath, "index.html")
				if _, err := os.Stat(idx); err == nil {
					c.File(idx)
					return
				}
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})
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

	litellmOk := false
	if s.ragService != nil && s.ragService.PingLLM(ctx) == nil {
		litellmOk = true
	}

	info, _ := s.qdrant.GetCollectionInfo(ctx, s.collection)
	pointsCount := uint64(0)
	if info != nil {
		pointsCount = info.PointsCount
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "healthy",
		"service":           "Wuthering Waves Lore RAG API",
		"qdrant_connected":  qdrantOk,
		"litellm_connected": litellmOk,
		"collection":        s.collection,
		"indexed_chunks":    pointsCount,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
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

func findChunksFile() string {
	candidates := []string{
		"data/processed/chunks.json",
		"./data/processed/chunks.json",
		"../data/processed/chunks.json",
		"../../data/processed/chunks.json",
		"/app/data/processed/chunks.json",
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func (s *Server) getCorpusData() *CorpusData {
	s.corpusMu.RLock()
	if s.corpus != nil {
		defer s.corpusMu.RUnlock()
		return s.corpus
	}
	s.corpusMu.RUnlock()

	s.corpusMu.Lock()
	defer s.corpusMu.Unlock()
	if s.corpus != nil {
		return s.corpus
	}

	filePath := findChunksFile()
	if filePath == "" {
		s.corpus = &CorpusData{
			Pages:        []CorpusPage{},
			TopSpeakers:  []SpeakerStat{},
			Chunks:       []parser.Chunk{},
			ChunksByPage: make(map[string][]parser.Chunk),
		}
		return s.corpus
	}

	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		s.corpus = &CorpusData{
			Pages:        []CorpusPage{},
			TopSpeakers:  []SpeakerStat{},
			Chunks:       []parser.Chunk{},
			ChunksByPage: make(map[string][]parser.Chunk),
		}
		return s.corpus
	}

	var chunks []parser.Chunk
	if err := json.Unmarshal(rawBytes, &chunks); err != nil {
		s.corpus = &CorpusData{
			Pages:        []CorpusPage{},
			TopSpeakers:  []SpeakerStat{},
			Chunks:       []parser.Chunk{},
			ChunksByPage: make(map[string][]parser.Chunk),
		}
		return s.corpus
	}

	pageMap := make(map[string]*CorpusPage)
	speakerMap := make(map[string]int)
	chunksByPage := make(map[string][]parser.Chunk)

	for _, ch := range chunks {
		p, exists := pageMap[ch.Title]
		if !exists {
			p = &CorpusPage{Title: ch.Title}
			pageMap[ch.Title] = p
		}
		p.Chars += ch.CharCount
		p.Chunks++

		for _, sp := range ch.Speakers {
			spClean := strings.TrimSpace(sp)
			if spClean != "" {
				speakerMap[spClean]++
			}
		}

		chunksByPage[ch.Title] = append(chunksByPage[ch.Title], ch)
	}

	pages := make([]CorpusPage, 0, len(pageMap))
	for _, p := range pageMap {
		pages = append(pages, *p)
	}
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].Title < pages[j].Title
	})

	type spCount struct {
		name  string
		count int
	}
	allSpk := make([]spCount, 0, len(speakerMap))
	for name, count := range speakerMap {
		allSpk = append(allSpk, spCount{name: name, count: count})
	}
	sort.Slice(allSpk, func(i, j int) bool {
		if allSpk[i].count == allSpk[j].count {
			return allSpk[i].name < allSpk[j].name
		}
		return allSpk[i].count > allSpk[j].count
	})

	topLimit := 60
	if len(allSpk) < topLimit {
		topLimit = len(allSpk)
	}
	topSpeakers := make([]SpeakerStat, topLimit)
	for i := 0; i < topLimit; i++ {
		topSpeakers[i] = SpeakerStat{Name: allSpk[i].name, Count: allSpk[i].count}
	}

	s.corpus = &CorpusData{
		TotalPages:    len(pages),
		TotalChunks:   len(chunks),
		TotalSpeakers: len(speakerMap),
		Pages:         pages,
		TopSpeakers:   topSpeakers,
		Chunks:        chunks,
		ChunksByPage:  chunksByPage,
	}

	return s.corpus
}

func (s *Server) handleCorpus(c *gin.Context) {
	data := s.getCorpusData()

	pageQuery := strings.TrimSpace(c.Query("page"))
	speakerQuery := strings.TrimSpace(c.Query("speaker"))
	textQuery := strings.TrimSpace(c.Query("q"))
	if textQuery == "" {
		textQuery = strings.TrimSpace(c.Query("query"))
	}
	if textQuery == "" {
		textQuery = strings.TrimSpace(c.Query("search"))
	}

	offsetStr := c.DefaultQuery("offset", "0")
	limitStr := c.DefaultQuery("limit", "50")

	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var matched []parser.Chunk
	if pageQuery != "" || speakerQuery != "" || textQuery != "" {
		pLower := strings.ToLower(pageQuery)
		sLower := strings.ToLower(speakerQuery)
		qLower := strings.ToLower(textQuery)
		for _, ch := range data.Chunks {
			if pageQuery != "" && !strings.Contains(strings.ToLower(ch.Title), pLower) {
				continue
			}
			if speakerQuery != "" {
				speakerMatched := false
				for _, sp := range ch.Speakers {
					if strings.Contains(strings.ToLower(sp), sLower) {
						speakerMatched = true
						break
					}
				}
				if !speakerMatched {
					continue
				}
			}
			if textQuery != "" {
				textMatched := strings.Contains(strings.ToLower(ch.Content), qLower) ||
					strings.Contains(strings.ToLower(ch.Title), qLower) ||
					strings.Contains(strings.ToLower(ch.Section), qLower) ||
					strings.Contains(strings.ToLower(ch.Subsection), qLower)
				if !textMatched {
					for _, sp := range ch.Speakers {
						if strings.Contains(strings.ToLower(sp), qLower) {
							textMatched = true
							break
						}
					}
				}
				if !textMatched {
					continue
				}
			}
			matched = append(matched, ch)
		}
	} else {
		matched = data.Chunks
	}

	totalMatched := len(matched)
	var slice []parser.Chunk
	if offset < totalMatched {
		end := offset + limit
		if end > totalMatched {
			end = totalMatched
		}
		slice = matched[offset:end]
	} else {
		slice = []parser.Chunk{}
	}

	respPages := data.Pages
	if textQuery != "" {
		matchedPageMap := make(map[string]bool)
		for _, ch := range matched {
			matchedPageMap[ch.Title] = true
		}
		filtered := make([]CorpusPage, 0, len(matchedPageMap))
		for _, p := range data.Pages {
			if matchedPageMap[p.Title] {
				filtered = append(filtered, p)
			}
		}
		respPages = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"total_pages":    data.TotalPages,
		"total_chunks":   data.TotalChunks,
		"total_speakers": data.TotalSpeakers,
		"matched_chunks": totalMatched,
		"offset":         offset,
		"limit":          limit,
		"pages":          respPages,
		"top_speakers":   data.TopSpeakers,
		"chunks":         slice,
	})
}

func (s *Server) handlePages(c *gin.Context) {
	data := s.getCorpusData()
	c.JSON(http.StatusOK, gin.H{
		"total": len(data.Pages),
		"pages": data.Pages,
	})
}

func (s *Server) handleSpeakers(c *gin.Context) {
	data := s.getCorpusData()
	c.JSON(http.StatusOK, gin.H{
		"total":    data.TotalSpeakers,
		"speakers": data.TopSpeakers,
	})
}

type registerRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) handleRegister(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username, email, and password are required"})
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Username) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username must be at least 3 characters long"})
		return
	}
	if len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 6 characters long"})
		return
	}
	if !strings.Contains(req.Email, "@") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email address format"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	user, err := s.db.CreateUser(c.Request.Context(), req.Username, req.Email, hash, "user", s.defaultQuestionLimit)
	if err != nil {
		if errors.Is(err, db.ErrUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Username or email is already registered"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	token, err := auth.GenerateToken(s.jwtSecret, user.ID, user.Username, user.Role, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token": token,
		"user": gin.H{
			"id":                  user.ID,
			"username":            user.Username,
			"email":               user.Email,
			"role":                user.Role,
			"question_count":      user.QuestionCount,
			"question_limit":      user.QuestionLimit,
			"questions_remaining": user.QuestionLimit,
		},
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) handleLogin(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}

	user, err := s.db.GetUserByUsernameOrEmail(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials", "message": "Invalid username or password"})
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials", "message": "Invalid username or password"})
		return
	}

	token, err := auth.GenerateToken(s.jwtSecret, user.ID, user.Username, user.Role, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session token"})
		return
	}

	remaining := -1
	if strings.ToLower(user.Role) != "admin" {
		remaining = user.QuestionLimit - user.QuestionCount
		if remaining < 0 {
			remaining = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":                  user.ID,
			"username":            user.Username,
			"email":               user.Email,
			"role":                user.Role,
			"question_count":      user.QuestionCount,
			"question_limit":      user.QuestionLimit,
			"questions_remaining": remaining,
		},
	})
}

func (s *Server) handleAuthMe(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	userID := c.GetString(auth.ContextUserID)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := s.db.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	remaining := -1
	if strings.ToLower(user.Role) != "admin" {
		remaining = user.QuestionLimit - user.QuestionCount
		if remaining < 0 {
			remaining = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                  user.ID,
			"username":            user.Username,
			"email":               user.Email,
			"role":                user.Role,
			"question_count":      user.QuestionCount,
			"question_limit":      user.QuestionLimit,
			"questions_remaining": remaining,
		},
	})
}

type chatRequest struct {
	Question string            `json:"question" binding:"required"`
	History  []rag.ChatMessage `json:"history,omitempty"`
	ThreadID string            `json:"thread_id,omitempty"`
}

func (s *Server) handleChat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing or invalid 'question' field"})
		return
	}

	userID := c.GetString(auth.ContextUserID)
	var remaining, limit int

	// 1. Quota enforcement via database
	if s.db != nil && userID != "" {
		allowed, rem, lim, err := s.db.CheckAndIncrementQuestionCount(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify usage quota"})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "question_limit_reached",
				"message":             fmt.Sprintf("You have reached your limit of %d questions. Please contact an administrator for extended access.", lim),
				"questions_remaining": 0,
				"questions_limit":     lim,
			})
			return
		}
		remaining = rem
		limit = lim
	}

	// 2. Resolve or create thread
	threadID := req.ThreadID
	if s.db != nil && userID != "" {
		if threadID == "" {
			tTitle := strings.TrimSpace(req.Question)
			if len(tTitle) > 48 {
				tTitle = tTitle[:48] + "…"
			}
			newThread, err := s.db.CreateThread(c.Request.Context(), userID, tTitle)
			if err == nil && newThread != nil {
				threadID = newThread.ID
			}
		}

		if threadID != "" {
			_, _ = s.db.AddMessage(c.Request.Context(), threadID, userID, "user", req.Question, "", "")
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	res, err := s.ragService.AnswerConversation(ctx, req.Question, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to answer question: %v", err)})
		return
	}

	// 3. Persist assistant message
	if s.db != nil && userID != "" && threadID != "" {
		sourcesJSON, _ := json.Marshal(res.Sources)
		metaJSON, _ := json.Marshal(gin.H{
			"model":        res.Model,
			"latency_ms":   res.LatencyMs,
			"total_tokens": res.TotalTokens,
		})
		_, _ = s.db.AddMessage(c.Request.Context(), threadID, userID, "assistant", res.Answer, string(sourcesJSON), string(metaJSON))
	}

	c.JSON(http.StatusOK, gin.H{
		"question":            req.Question,
		"answer":              res.Answer,
		"sources":             res.Sources,
		"model":               res.Model,
		"latency_ms":          res.LatencyMs,
		"total_tokens":        res.TotalTokens,
		"thread_id":           threadID,
		"questions_remaining": remaining,
		"questions_limit":     limit,
	})
}

type createThreadRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleListThreads(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusOK, gin.H{"threads": []interface{}{}})
		return
	}
	userID := c.GetString(auth.ContextUserID)
	threads, err := s.db.ListThreads(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list threads"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"threads": threads})
}

func (s *Server) handleCreateThread(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	var req createThreadRequest
	_ = c.ShouldBindJSON(&req)

	userID := c.GetString(auth.ContextUserID)
	thread, err := s.db.CreateThread(c.Request.Context(), userID, req.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create thread"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"thread": thread})
}

func (s *Server) handleGetThread(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	userID := c.GetString(auth.ContextUserID)
	threadID := c.Param("id")

	thread, msgs, err := s.db.GetThreadWithMessages(c.Request.Context(), userID, threadID)
	if err != nil {
		if errors.Is(err, db.ErrThreadNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Thread not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get thread"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"thread": thread, "messages": msgs})
}

func (s *Server) handleDeleteThread(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	userID := c.GetString(auth.ContextUserID)
	threadID := c.Param("id")

	if err := s.db.DeleteThread(c.Request.Context(), userID, threadID); err != nil {
		if errors.Is(err, db.ErrThreadNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Thread not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete thread"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) handleAdminListUsers(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	users, err := s.db.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type resetQuotaRequest struct {
	Count int `json:"count"`
	Limit int `json:"limit"`
}

func (s *Server) handleAdminResetQuota(c *gin.Context) {
	if s.db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}
	targetUserID := c.Param("id")
	var req resetQuotaRequest
	_ = c.ShouldBindJSON(&req)

	if err := s.db.ResetUserQuestionCount(c.Request.Context(), targetUserID, req.Count, req.Limit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset quota"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "user_id": targetUserID})
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

	s.corpusMu.Lock()
	s.corpus = nil
	s.corpusMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"page":           page.Title,
		"chunks_indexed": len(chunks),
		"duration_ms":    duration,
		"collection":     s.collection,
	})
}
