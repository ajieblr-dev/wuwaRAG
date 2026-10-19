package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Point represents a vector data point with an ID, vector embedding, and JSON payload.
type Point struct {
	ID      string                 `json:"id"`
	Vector  []float32              `json:"vector"`
	Payload map[string]interface{} `json:"payload"`
}

// ScoredPoint represents a search result from Qdrant.
type ScoredPoint struct {
	ID      interface{}            `json:"id"`
	Version int                    `json:"version"`
	Score   float64                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// CollectionInfo contains metadata about a Qdrant collection.
type CollectionInfo struct {
	Status      string `json:"status"`
	PointsCount uint64 `json:"points_count"`
	VectorsCount uint64 `json:"vectors_count"`
}

// Client is a REST client for Qdrant vector database.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Qdrant client.
func NewClient(baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:6333"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Ping checks if Qdrant service is responding.
func (c *Client) Ping(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/readyz", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Try healthz if readyz fails
		reqURL = fmt.Sprintf("%s/healthz", c.baseURL)
		req2, err2 := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err2 != nil {
			return err
		}
		resp2, err2 := c.httpClient.Do(req2)
		if err2 != nil {
			return fmt.Errorf("qdrant connection failed: %w", err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode == http.StatusOK {
			return nil
		}
		return fmt.Errorf("qdrant ping status code: %d", resp2.StatusCode)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qdrant ping status code: %d", resp.StatusCode)
	}
	return nil
}

// EnsureCollection verifies if the collection exists, creating it if necessary.
func (c *Client) EnsureCollection(ctx context.Context, collectionName string, vectorDim int) error {
	checkURL := fmt.Sprintf("%s/collections/%s", c.baseURL, collectionName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return nil // Collection already exists
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// Create collection with Cosine distance
	createPayload := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     vectorDim,
			"distance": "Cosine",
		},
	}
	bodyBytes, err := json.Marshal(createPayload)
	if err != nil {
		return err
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, checkURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("failed to create collection %q: %w", collectionName, err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(putResp.Body)
		return fmt.Errorf("failed to create collection %q (status %d): %s", collectionName, putResp.StatusCode, string(respBody))
	}

	return nil
}

// UpsertPoints inserts or updates vector points in Qdrant in batches.
func (c *Client) UpsertPoints(ctx context.Context, collectionName string, points []Point, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100
	}

	upsertURL := fmt.Sprintf("%s/collections/%s/points?wait=true", c.baseURL, collectionName)

	for i := 0; i < len(points); i += batchSize {
		end := i + batchSize
		if end > len(points) {
			end = len(points)
		}
		batch := points[i:end]

		reqBody := map[string]interface{}{
			"points": batch,
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal points batch: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, upsertURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("failed to build upsert request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upsert points batch %d-%d: %w", i, end, err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("upsert error for batch %d-%d (status %d): %s", i, end, resp.StatusCode, string(respBody))
		}
		resp.Body.Close()
	}

	return nil
}

// Search searches for the nearest vectors in the collection.
func (c *Client) Search(ctx context.Context, collectionName string, vector []float32, limit int, scoreThreshold float64) ([]ScoredPoint, error) {
	if limit <= 0 {
		limit = 5
	}

	searchURL := fmt.Sprintf("%s/collections/%s/points/search", c.baseURL, collectionName)

	searchPayload := map[string]interface{}{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
	}
	if scoreThreshold > 0 {
		searchPayload["score_threshold"] = scoreThreshold
	}

	bodyBytes, err := json.Marshal(searchPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search query: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to build search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var searchResp struct {
		Result []ScoredPoint `json:"result"`
		Status string        `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return searchResp.Result, nil
}

// GetCollectionInfo returns stats for a collection.
func (c *Client) GetCollectionInfo(ctx context.Context, collectionName string) (*CollectionInfo, error) {
	infoURL := fmt.Sprintf("%s/collections/%s", c.baseURL, collectionName)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, infoURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get collection info (status %d): %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		Result struct {
			Status       string `json:"status"`
			PointsCount  uint64 `json:"points_count"`
			VectorsCount uint64 `json:"vectors_count"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode collection info: %w", err)
	}

	return &CollectionInfo{
		Status:       res.Result.Status,
		PointsCount:  res.Result.PointsCount,
		VectorsCount: res.Result.VectorsCount,
	}, nil
}

func pointIDToString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ScrollByText searches points matching supplied keywords in payload fields across distinct terms.
func (c *Client) ScrollByText(ctx context.Context, collectionName string, keywords []string, limit int) ([]ScoredPoint, error) {
	if limit <= 0 {
		limit = 25
	}
	if len(keywords) == 0 {
		return nil, nil
	}

	var validKeywords []string
	seenKw := make(map[string]bool)
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		lower := strings.ToLower(kw)
		if len(kw) >= 2 && !seenKw[lower] {
			seenKw[lower] = true
			validKeywords = append(validKeywords, kw)
		}
	}
	if len(validKeywords) == 0 {
		return nil, nil
	}

	// Sort keywords by length descending to prioritize specific entities/phrases over generic short words
	sort.Slice(validKeywords, func(i, j int) bool {
		return len(validKeywords[i]) > len(validKeywords[j])
	})

	// Cap at top 12 distinct keywords to ensure specific single entities aren't truncated by phrases
	if len(validKeywords) > 12 {
		validKeywords = validKeywords[:12]
	}

	scrollURL := fmt.Sprintf("%s/collections/%s/points/scroll", c.baseURL, collectionName)
	seenPoints := make(map[string]bool)
	var mergedPoints []ScoredPoint

	for _, kw := range validKeywords {
		shouldConditions := []map[string]interface{}{
			{"key": "title", "match": map[string]interface{}{"text": kw}},
			{"key": "speakers", "match": map[string]interface{}{"text": kw}},
			{"key": "subsection", "match": map[string]interface{}{"text": kw}},
			{"key": "content", "match": map[string]interface{}{"text": kw}},
		}

		scrollPayload := map[string]interface{}{
			"limit":        limit,
			"with_payload": true,
			"filter": map[string]interface{}{
				"should": shouldConditions,
			},
		}

		bodyBytes, err := json.Marshal(scrollPayload)
		if err != nil {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, scrollURL, bytes.NewReader(bodyBytes))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		var scrollResp struct {
			Result struct {
				Points []ScoredPoint `json:"points"`
			} `json:"result"`
			Status string `json:"status"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&scrollResp); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		for _, pt := range scrollResp.Result.Points {
			idStr := pointIDToString(pt.ID)
			if !seenPoints[idStr] {
				seenPoints[idStr] = true
				mergedPoints = append(mergedPoints, pt)
			}
		}
	}

	return mergedPoints, nil
}

