package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Embedder generates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// HuggingFaceEmbedder uses Hugging Face Inference API or LiteLLM embedding endpoint.
type HuggingFaceEmbedder struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	dim        int
}

// NewHuggingFaceEmbedder creates an embedder for sentence-transformers/all-MiniLM-L6-v2.
func NewHuggingFaceEmbedder(apiKey, model, baseURL string, dim int) *HuggingFaceEmbedder {
	if model == "" {
		model = "sentence-transformers/all-MiniLM-L6-v2"
	}
	if baseURL == "" {
		baseURL = "https://router.huggingface.co/hf-inference/models"
	}
	if dim <= 0 {
		dim = 384
	}
	return &HuggingFaceEmbedder{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 40 * time.Second,
		},
		dim: dim,
	}
}

// Dimension returns the vector dimension.
func (e *HuggingFaceEmbedder) Dimension() int {
	return e.dim
}

// Embed generates embedding for a single text.
func (e *HuggingFaceEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	batch, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	return batch[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
func (e *HuggingFaceEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// If no API key is provided, use deterministic semantic hash fallback
	if strings.TrimSpace(e.apiKey) == "" {
		return fallbackDeterministicEmbeddings(texts, e.dim), nil
	}

	reqURL := fmt.Sprintf("%s/%s", e.baseURL, e.model)

	reqPayload := map[string]interface{}{
		"inputs": texts,
		"options": map[string]interface{}{
			"wait_for_model": true,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to build embed request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		// Log and fallback to local deterministic embedding on network failure
		return fallbackDeterministicEmbeddings(texts, e.dim), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		// If HF returns error (rate limit / quota), gracefully fallback
		return fallbackDeterministicEmbeddings(texts, e.dim), nil
	}

	var rawResult interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		return fallbackDeterministicEmbeddings(texts, e.dim), nil
	}

	// Parse response
	parsed, err := parseHFEmbeddingResponse(rawResult, len(texts), e.dim)
	if err != nil {
		return fallbackDeterministicEmbeddings(texts, e.dim), nil
	}

	return parsed, nil
}

func parseHFEmbeddingResponse(raw interface{}, expectedCount, dim int) ([][]float32, error) {
	switch v := raw.(type) {
	case []interface{}:
		if len(v) == 0 {
			return nil, fmt.Errorf("empty response array")
		}
		// Could be [][]float (batch) or []float (single)
		if _, ok := v[0].(float64); ok {
			// Single vector
			vec := make([]float32, len(v))
			for i, val := range v {
				vec[i] = float32(val.(float64))
			}
			return [][]float32{normalizeVector(vec)}, nil
		}

		results := make([][]float32, len(v))
		for i, item := range v {
			itemSlice, ok := item.([]interface{})
			if !ok {
				return nil, fmt.Errorf("unexpected item format at index %d", i)
			}
			vec := make([]float32, len(itemSlice))
			for j, val := range itemSlice {
				if f, ok := val.(float64); ok {
					vec[j] = float32(f)
				}
			}
			results[i] = normalizeVector(vec)
		}
		return results, nil
	default:
		return nil, fmt.Errorf("unsupported embedding response format")
	}
}

// fallbackDeterministicEmbeddings creates a normalized, deterministic semantic embedding
// based on character n-grams and hashing. Used when HuggingFace is offline or without key.
func fallbackDeterministicEmbeddings(texts []string, dim int) [][]float32 {
	results := make([][]float32, len(texts))
	for i, t := range texts {
		results[i] = generateDeterministicVector(t, dim)
	}
	return results
}

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "up": true, "about": true, "into": true,
	"over": true, "after": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "have": true, "has": true, "had": true,
	"do": true, "does": true, "did": true, "what": true, "which": true, "who": true,
	"whom": true, "this": true, "that": true, "these": true, "those": true, "am": true,
	"it": true, "its": true, "as": true, "if": true, "so": true, "then": true,
	"tell": true, "me": true, "can": true, "could": true, "would": true, "happen": true,
	"happened": true, "happens": true, "play": true, "played": true, "role": true,
	"there": true, "their": true, "siapa": true, "apa": true, "dimana": true,
	"kapan": true, "kenapa": true, "mengapa": true, "bagaimana": true, "tentang": true,
	"adalah": true, "itu": true, "ini": true, "yang": true,
}

func generateDeterministicVector(text string, dim int) []float32 {
	vec := make([]float32, dim)
	words := strings.Fields(strings.ToLower(text))

	for _, rawWord := range words {
		w := strings.Trim(rawWord, ".,!?:;\"'()[]{}")
		if w == "" {
			continue
		}

		weight := float32(1.0)
		if stopWords[w] {
			weight = 0.20
		}

		// Word hash
		h := sha256.Sum256([]byte(w))
		idx := int(binary.BigEndian.Uint16(h[0:2])) % dim
		val := (float32(int16(binary.BigEndian.Uint16(h[2:4]))) / 32768.0) * weight
		vec[idx] += val

		// Substring n-grams
		for n := 3; n <= 4 && n <= len(w); n++ {
			for j := 0; j <= len(w)-n; j++ {
				ngram := w[j : j+n]
				ngh := sha256.Sum256([]byte(ngram))
				nIdx := int(binary.BigEndian.Uint16(ngh[0:2])) % dim
				nVal := (float32(int16(binary.BigEndian.Uint16(ngh[2:4]))) / 65536.0) * weight
				vec[nIdx] += nVal
			}
		}
	}

	return normalizeVector(vec)
}

func normalizeVector(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return vec
	}
	norm := float32(math.Sqrt(sum))
	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}
	return normalized
}
