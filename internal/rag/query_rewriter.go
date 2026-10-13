package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// QueryRewriter optimizes and expands user queries for hybrid search.
type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) (expandedQuery string, keywords []string, err error)
}

// RewrittenQuery holds the structured output of the rewriting step.
type RewrittenQuery struct {
	ExpandedQuery string   `json:"expanded_query"`
	Keywords      []string `json:"keywords"`
}

// LLMQueryRewriter uses the LLM with deterministic heuristic fallback.
type LLMQueryRewriter struct {
	llm *LLMClient
}

// NewQueryRewriter creates a new query rewriter.
func NewQueryRewriter(llm *LLMClient) *LLMQueryRewriter {
	return &LLMQueryRewriter{llm: llm}
}

var (
	reJSONBlock = regexp.MustCompile(`(?s)\{.*?\}`)
	reWords     = regexp.MustCompile(`[A-Za-z0-9'’\-]+`)
)

// Rewrite expands a user's question with canonical entity names and extracts key search terms.
func (r *LLMQueryRewriter) Rewrite(ctx context.Context, query string) (string, []string, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return "", nil, nil
	}

	// If no LLM available, use heuristic extraction immediately
	if r.llm == nil {
		return trimmed, ExtractHeuristicKeywords(trimmed), nil
	}

	systemPrompt := `You are an expert search query optimizer for a Wuthering Waves game lore retrieval engine.
Given a user question, expand it into a semantically dense query that resolves pronouns, clarifies character names, factions, Sentinels, and locations, and extract key search keywords.

Return ONLY a JSON object with this exact schema:
{
  "expanded_query": "concise query enriched with canonical names and context",
  "keywords": ["entity1", "entity2", "entity3"]
}

Examples:
- "who is the girl with the umbrella?" -> {"expanded_query": "Yangyang Midnight Rangers Jinzhou companion sword user", "keywords": ["Yangyang", "Midnight Rangers", "Jinzhou"]}
- "carlotta echoes bomb" -> {"expanded_query": "Carlotta Montelli Averardo Vault Echoes sabotage Carnevale", "keywords": ["Carlotta", "Averardo Vault", "Montelli", "Carnevale"]}
- "sentinel of mengzhou" -> {"expanded_query": "Sentinel Hsin the Moon Fox Mengzhou Xuanfang Hold Civilization Capsule", "keywords": ["Sentinel Hsin", "Moon Fox", "Mengzhou", "Civilization Capsule"]}
`

	userPrompt := fmt.Sprintf("User Question: %s\nJSON:", trimmed)

	// Short timeout for fast query pre-processing (max 4 seconds)
	rewriteCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	resp, err := r.llm.GenerateAnswer(rewriteCtx, systemPrompt, userPrompt)
	if err != nil {
		// Graceful fallback to heuristic extraction
		return trimmed, ExtractHeuristicKeywords(trimmed), nil
	}

	jsonMatch := reJSONBlock.FindString(resp.Answer)
	if jsonMatch == "" {
		return trimmed, ExtractHeuristicKeywords(trimmed), nil
	}

	var parsed RewrittenQuery
	if err := json.Unmarshal([]byte(jsonMatch), &parsed); err != nil {
		return trimmed, ExtractHeuristicKeywords(trimmed), nil
	}

	expanded := strings.TrimSpace(parsed.ExpandedQuery)
	if expanded == "" {
		expanded = trimmed
	}

	keywords := make([]string, 0, len(parsed.Keywords))
	seen := make(map[string]bool)
	for _, kw := range parsed.Keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" && len(kw) >= 2 && !seen[strings.ToLower(kw)] {
			seen[strings.ToLower(kw)] = true
			keywords = append(keywords, kw)
		}
	}

	if len(keywords) == 0 {
		keywords = ExtractHeuristicKeywords(trimmed)
	}

	return expanded, keywords, nil
}

// ExtractHeuristicKeywords extracts named entities, capitalized words, and significant content terms.
func ExtractHeuristicKeywords(text string) []string {
	words := reWords.FindAllString(text, -1)
	var keywords []string
	seen := make(map[string]bool)

	stop := map[string]bool{
		"what": true, "who": true, "where": true, "when": true, "why": true, "how": true,
		"is": true, "are": true, "was": true, "were": true, "the": true, "a": true, "an": true,
		"and": true, "or": true, "in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "about": true, "with": true, "by": true, "from": true, "tell": true,
		"me": true, "does": true, "did": true, "can": true, "could": true, "would": true,
		"happen": true, "happened": true, "happens": true, "play": true, "played": true,
		"role": true, "this": true, "that": true, "there": true, "their": true,
	}

	// 1. First pass: Collect multi-word capitalized phrases or single capitalized words
	var capBuffer []string
	for _, w := range words {
		if len(w) > 0 && w[0] >= 'A' && w[0] <= 'Z' && !stop[strings.ToLower(w)] {
			capBuffer = append(capBuffer, w)
		} else {
			if len(capBuffer) > 0 {
				phrase := strings.Join(capBuffer, " ")
				if !seen[strings.ToLower(phrase)] {
					seen[strings.ToLower(phrase)] = true
					keywords = append(keywords, phrase)
				}
				capBuffer = nil
			}
		}
	}
	if len(capBuffer) > 0 {
		phrase := strings.Join(capBuffer, " ")
		if !seen[strings.ToLower(phrase)] {
			seen[strings.ToLower(phrase)] = true
			keywords = append(keywords, phrase)
		}
	}

	// 2. Second pass: Collect remaining non-stop words if keywords are few
	for _, w := range words {
		low := strings.ToLower(w)
		if len(w) >= 3 && !stop[low] && !seen[low] {
			seen[low] = true
			keywords = append(keywords, w)
		}
	}

	return keywords
}
