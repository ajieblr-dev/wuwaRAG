package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
)

// SourceSnippet represents a retrieved piece of lore used as evidence.
type SourceSnippet struct {
	ID           string   `json:"id"`
	DatasetCode  string   `json:"dataset_code,omitempty"`
	Title        string   `json:"title"`
	PageTitle    string   `json:"page_title,omitempty"`
	Section      string   `json:"section"`
	SectionTitle string   `json:"section_title,omitempty"`
	Subsection   string   `json:"subsection,omitempty"`
	Speakers     []string `json:"speakers,omitempty"`
	Content      string   `json:"content"`
	ChunkText    string   `json:"chunk_text,omitempty"`
	Score        float64  `json:"score"`
}

// QueryResult represents the complete RAG response.
type QueryResult struct {
	Question    string          `json:"question"`
	Answer      string          `json:"answer"`
	Sources     []SourceSnippet `json:"sources"`
	Model       string          `json:"model"`
	LatencyMs   int64           `json:"latency_ms"`
	TotalTokens int             `json:"total_tokens,omitempty"`
}

// Option configures Service settings.
type Option func(*Service)

// WithHybridSearch enables or disables vector + keyword hybrid search.
func WithHybridSearch(enabled bool) Option {
	return func(s *Service) {
		s.hybridEnabled = enabled
	}
}

// WithQueryRewriter configures the query rewriter component.
func WithQueryRewriter(rewriter QueryRewriter) Option {
	return func(s *Service) {
		s.queryRewriter = rewriter
	}
}

// WithQueryRewriteEnabled enables or disables LLM query rewriting.
func WithQueryRewriteEnabled(enabled bool) Option {
	return func(s *Service) {
		s.queryRewriteEnabled = enabled
	}
}

// WithRRFConstant configures the k constant in 1/(k + rank).
func WithRRFConstant(k int) Option {
	return func(s *Service) {
		if k > 0 {
			s.rrfK = k
		}
	}
}

// WithCandidatePoolSize configures how many vector and keyword candidates are retrieved before fusion.
func WithCandidatePoolSize(size int) Option {
	return func(s *Service) {
		if size > 0 {
			s.candidatePoolSize = size
		}
	}
}

// Service coordinates retrieval from Qdrant and answer generation from LiteLLM.
type Service struct {
	qdrant              *qdrant.Client
	embedder            Embedder
	llm                 *LLMClient
	collection          string
	topK                int
	scoreThreshold      float64
	queryRewriter       QueryRewriter
	hybridEnabled       bool
	queryRewriteEnabled bool
	rrfK                int
	candidatePoolSize   int
}

// NewService creates a new RAG orchestration service.
func NewService(qdrantClient *qdrant.Client, embedder Embedder, llm *LLMClient, collection string, topK int, scoreThreshold float64, opts ...Option) *Service {
	if collection == "" {
		collection = "wuwa_lore"
	}
	if topK <= 0 {
		topK = 5
	}
	s := &Service{
		qdrant:              qdrantClient,
		embedder:            embedder,
		llm:                 llm,
		collection:          collection,
		topK:                topK,
		scoreThreshold:      scoreThreshold,
		hybridEnabled:       false,
		queryRewriteEnabled: false,
		rrfK:                60,
		candidatePoolSize:   25,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// PingLLM checks if the LLM gateway is reachable.
func (s *Service) PingLLM(ctx context.Context) error {
	if s.llm == nil {
		return fmt.Errorf("no LLM client configured")
	}
	return s.llm.Ping(ctx)
}

// SearchOnly retrieves relevant context snippets without calling the LLM.
func (s *Service) SearchOnly(ctx context.Context, query string, limit int) ([]SourceSnippet, error) {
	return s.SearchOnlyWithHistory(ctx, query, nil, limit)
}

// SearchOnlyWithHistory retrieves relevant context snippets, taking conversation history into account during query rewriting.
func (s *Service) SearchOnlyWithHistory(ctx context.Context, query string, history []ChatMessage, limit int) ([]SourceSnippet, error) {
	if limit <= 0 {
		limit = s.topK
	}

	searchQuery := query
	var keywords []string

	// 1. Pre-Retrieval: Query Rewriting / Expansion with history
	if s.queryRewriteEnabled && s.queryRewriter != nil {
		expanded, kw, err := s.queryRewriter.RewriteWithHistory(ctx, query, history)
		if err == nil && expanded != "" {
			if !strings.Contains(strings.ToLower(expanded), strings.ToLower(query)) {
				searchQuery = strings.TrimSpace(query + " " + expanded)
			} else {
				searchQuery = expanded
			}
			keywords = kw
		}
	}

	rawKeywords := ExtractHeuristicKeywords(query)
	for _, rk := range rawKeywords {
		found := false
		for _, kw := range keywords {
			if strings.EqualFold(kw, rk) {
				found = true
				break
			}
		}
		if !found {
			keywords = append(keywords, rk)
		}
	}

	// 2. Retrieval: Hybrid Search or Pure Vector
	if s.hybridEnabled {
		return s.hybridSearch(ctx, searchQuery, keywords, limit)
	}

	return s.vectorOnlySearch(ctx, searchQuery, limit)
}

func (s *Service) hybridSearch(ctx context.Context, searchQuery string, keywords []string, limit int) ([]SourceSnippet, error) {
	poolSize := s.candidatePoolSize
	if poolSize < limit*3 {
		poolSize = limit * 3
	}

	// A. Dense Vector candidate retrieval
	queryVec, err := s.embedder.Embed(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	vectorPoints, err := s.qdrant.Search(ctx, s.collection, queryVec, poolSize, s.scoreThreshold)
	if err != nil {
		return nil, fmt.Errorf("qdrant vector search failed: %w", err)
	}

	// B. Lexical/Keyword candidate retrieval
	lexicalPoints, err := s.qdrant.ScrollByText(ctx, s.collection, keywords, poolSize)
	if err != nil {
		// Fallback to vector results if scroll isn't supported (e.g. mock test server)
		lexicalPoints = nil
	}

	// C. Reciprocal Rank Fusion & Reranking (deduplicates identical contents)
	snippets := ReciprocalRankFusion(vectorPoints, lexicalPoints, keywords, s.rrfK, limit)
	return snippets, nil
}

func (s *Service) vectorOnlySearch(ctx context.Context, searchQuery string, limit int) ([]SourceSnippet, error) {
	queryVec, err := s.embedder.Embed(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	fetchLimit := limit * 2
	scoredPoints, err := s.qdrant.Search(ctx, s.collection, queryVec, fetchLimit, s.scoreThreshold)
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	seenContent := make(map[string]bool)
	var snippets []SourceSnippet
	for _, p := range scoredPoints {
		if len(snippets) >= limit {
			break
		}
		snip := ConvertPointToSnippet(p)
		contentKey := strings.TrimSpace(snip.Content)
		if len(contentKey) > 160 {
			contentKey = contentKey[:160]
		}
		fingerprint := snip.Title + "::" + contentKey
		if seenContent[fingerprint] {
			continue
		}
		seenContent[fingerprint] = true
		snippets = append(snippets, snip)
	}
	return snippets, nil
}

// AnswerQuestion executes the full RAG pipeline for a single question.
func (s *Service) AnswerQuestion(ctx context.Context, question string) (*QueryResult, error) {
	return s.AnswerConversation(ctx, question, nil)
}

// AnswerConversation executes the full RAG pipeline with multi-turn conversation history.
func (s *Service) AnswerConversation(ctx context.Context, question string, history []ChatMessage) (*QueryResult, error) {
	start := time.Now()

	// 1. Retrieve relevant lore snippets with contextual rewrite if history is present
	snippets, err := s.SearchOnlyWithHistory(ctx, question, history, s.topK)
	if err != nil {
		return nil, err
	}

	// 2. Build context block
	var contextBuilder strings.Builder
	if len(snippets) == 0 {
		contextBuilder.WriteString("No specific lore records found in the database for this query.\n")
	} else {
		for i, snip := range snippets {
			contextBuilder.WriteString(fmt.Sprintf("--- [DOCUMENT %d: %s | %s] (Relevance: %.2f) ---\n", i+1, snip.Title, snip.Section, snip.Score))
			if snip.Subsection != "" {
				contextBuilder.WriteString(fmt.Sprintf("Objective / Scene: %s\n", snip.Subsection))
			}
			if len(snip.Speakers) > 0 {
				contextBuilder.WriteString(fmt.Sprintf("Speakers: %s\n", strings.Join(snip.Speakers, ", ")))
			}
			contextBuilder.WriteString(fmt.Sprintf("%s\n\n", snip.Content))
		}
	}

	systemPrompt := `You are an expert Wuthering Waves lore archivist and assistant specializing in Solaris-3 world lore and all Main Quest storylines from Prologue through Chapter IV (including Huanglong, Mt. Firmament, Rinascita, Septimont, Lahai-Roi, and Mengzhou).
Your mission is to provide accurate, engaging, deeply grounded, and faithful answers based on the provided lore documents.

Guidelines:
1. Grounding & Citations:
   - Answer the question using ONLY the provided lore documents.
   - Every factual claim, event, or character detail MUST carry bracketed citation numbers corresponding to the source documents, e.g. [1], [2], or [1][3].
   - If a specific detail is not found in the records, explicitly state that it is unrecorded or not found in current quest files rather than speculating.

2. Structure & Tone:
   - Begin directly with an executive summary line starting with: "In summary: <one to two concise sentences directly answering the question> [X]".
   - Never use filler preambles or meta-commentary like "Based on the provided lore records...", "According to the documents...", or "Here is the information". Speak with the voice of an authoritative Solaris-3 lore archivist.
   - Organize the remainder of your answer with clear markdown headings (### Topic), bold entity names (**Rover**, **Jiyan**, **Ovathrax**), and structured bullet points (* Detail [X]).
   - Cite relevant speakers, quest chapters, and scene locations to give rich contextual depth.
   - For multi-turn conversations, maintain continuity and reference earlier context naturally.`

	userPrompt := fmt.Sprintf("Retrieved Lore Records:\n%s\nCurrent Question: %s\n\nPlease answer the question based on the lore above, citing document numbers with [1], [2], etc., and starting with 'In summary: ...'.", contextBuilder.String(), question)

	// 3. Invoke LLM Gateway with history
	var answer string
	var modelUsed string
	var totalTokens int

	llmResp, err := s.llm.GenerateConversation(ctx, systemPrompt, history, userPrompt)
	if err != nil {
		// If LLM gateway fails, fall back to providing the retrieved context directly
		if len(snippets) > 0 {
			var fallbackAnswer strings.Builder
			fallbackAnswer.WriteString("(Notice: LLM Gateway unavailable; displaying direct retrieved lore records):\n\n")
			for _, snip := range snippets {
				fallbackAnswer.WriteString(fmt.Sprintf("• [%s - %s]:\n%s\n\n", snip.Section, snip.Subsection, snip.Content))
			}
			answer = fallbackAnswer.String()
			modelUsed = "retrieval-fallback"
		} else {
			return nil, fmt.Errorf("failed to generate response: %w", err)
		}
	} else {
		answer = llmResp.Answer
		modelUsed = llmResp.Model
		totalTokens = llmResp.TotalTokens
	}

	latency := time.Since(start).Milliseconds()

	return &QueryResult{
		Question:    question,
		Answer:      answer,
		Sources:     snippets,
		Model:       modelUsed,
		LatencyMs:   latency,
		TotalTokens: totalTokens,
	}, nil
}
