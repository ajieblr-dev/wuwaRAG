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

// Service coordinates retrieval from Qdrant and answer generation from LiteLLM.
type Service struct {
	qdrant         *qdrant.Client
	embedder       Embedder
	llm            *LLMClient
	collection     string
	topK           int
	scoreThreshold float64
}

// NewService creates a new RAG orchestration service.
func NewService(qdrantClient *qdrant.Client, embedder Embedder, llm *LLMClient, collection string, topK int, scoreThreshold float64) *Service {
	if collection == "" {
		collection = "wuwa_lore"
	}
	if topK <= 0 {
		topK = 5
	}
	return &Service{
		qdrant:         qdrantClient,
		embedder:       embedder,
		llm:            llm,
		collection:     collection,
		topK:           topK,
		scoreThreshold: scoreThreshold,
	}
}

// SearchOnly retrieves relevant context snippets without calling the LLM.
func (s *Service) SearchOnly(ctx context.Context, query string, limit int) ([]SourceSnippet, error) {
	if limit <= 0 {
		limit = s.topK
	}

	queryVec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	scoredPoints, err := s.qdrant.Search(ctx, s.collection, queryVec, limit, s.scoreThreshold)
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}

	snippets := make([]SourceSnippet, 0, len(scoredPoints))
	for _, p := range scoredPoints {
		idStr := fmt.Sprintf("%v", p.ID)
		payload := p.Payload

		title, _ := payload["title"].(string)
		section, _ := payload["section"].(string)
		subsection, _ := payload["subsection"].(string)
		content, _ := payload["content"].(string)

		var speakers []string
		if rawSpeakers, ok := payload["speakers"].([]interface{}); ok {
			for _, sp := range rawSpeakers {
				if spStr, ok := sp.(string); ok {
					speakers = append(speakers, spStr)
				}
			}
		}

		snippets = append(snippets, SourceSnippet{
			ID:           idStr,
			Title:        title,
			PageTitle:    title,
			Section:      section,
			SectionTitle: section,
			Subsection:   subsection,
			Speakers:     speakers,
			Content:      content,
			ChunkText:    content,
			Score:        p.Score,
		})
	}

	return snippets, nil
}

// AnswerQuestion executes the full RAG pipeline: retrieval, context injection, and LLM inference.
func (s *Service) AnswerQuestion(ctx context.Context, question string) (*QueryResult, error) {
	start := time.Now()

	// 1. Retrieve relevant lore snippets
	snippets, err := s.SearchOnly(ctx, question, s.topK)
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

	systemPrompt := `You are an expert Wuthering Waves lore archivist and assistant specializing in the "Simulacrum of the Heart" quest line and Solaris-3 world lore.
Your mission is to provide accurate, engaging, and faithful answers based on the provided lore documents.
Guidelines:
1. Answer the question using ONLY the provided lore documents.
2. If the context does not contain the answer, explicitly state that the detail is not found in the quest records rather than hallucinating.
3. Mention relevant characters, speakers, and quest scenes whenever applicable to provide rich context.
4. Keep the tone knowledgeable and lore-accurate.`

	userPrompt := fmt.Sprintf("Retrieved Lore Records:\n%s\nQuestion: %s\n\nPlease answer the question based on the lore above.", contextBuilder.String(), question)

	// 3. Invoke LLM Gateway
	var answer string
	var modelUsed string
	var totalTokens int

	llmResp, err := s.llm.GenerateAnswer(ctx, systemPrompt, userPrompt)
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
