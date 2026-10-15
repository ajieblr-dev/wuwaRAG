package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatMessage represents a conversation turn.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents the response from an LLM.
type ChatResponse struct {
	Answer       string `json:"answer"`
	Model        string `json:"model"`
	TotalTokens  int    `json:"total_tokens"`
	PromptTokens int    `json:"prompt_tokens"`
}

// LLMClient generates responses using LiteLLM proxy or direct fallback.
type LLMClient struct {
	liteLLMURL   string
	liteLLMKey   string
	modelName    string
	geminiKey    string
	groqKey      string
	httpClient   *http.Client
}

// NewLLMClient creates an LLM client connecting to the LiteLLM proxy.
func NewLLMClient(liteLLMURL, liteLLMKey, modelName, geminiKey, groqKey string) *LLMClient {
	if liteLLMURL == "" {
		liteLLMURL = "http://localhost:4000"
	}
	if modelName == "" {
		modelName = "wuwa-lore-model"
	}
	return &LLMClient{
		liteLLMURL:   strings.TrimRight(liteLLMURL, "/"),
		liteLLMKey:   liteLLMKey,
		modelName:    modelName,
		geminiKey:    geminiKey,
		groqKey:      groqKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Ping checks if the LiteLLM proxy or gateway endpoint is reachable.
func (c *LLMClient) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// 1. Try unauthenticated /health/liveliness first
	req, err := http.NewRequestWithContext(pingCtx, http.MethodGet, c.liteLLMURL+"/health/liveliness", nil)
	if err == nil {
		if resp, err := c.httpClient.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
	}

	// 2. Fall back to /health with Bearer key if configured
	req2, err := http.NewRequestWithContext(pingCtx, http.MethodGet, c.liteLLMURL+"/health", nil)
	if err != nil {
		return err
	}
	if c.liteLLMKey != "" {
		req2.Header.Set("Authorization", "Bearer "+c.liteLLMKey)
	}
	resp2, err := c.httpClient.Do(req2)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode >= 200 && resp2.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("gateway returned status %d", resp2.StatusCode)
}

// GenerateAnswer calls LiteLLM proxy with fallback handling for a single turn.
func (c *LLMClient) GenerateAnswer(ctx context.Context, systemPrompt, userQuery string) (*ChatResponse, error) {
	return c.GenerateConversation(ctx, systemPrompt, nil, userQuery)
}

// GenerateConversation calls LiteLLM proxy with full multi-turn conversation history.
func (c *LLMClient) GenerateConversation(ctx context.Context, systemPrompt string, history []ChatMessage, userQuery string) (*ChatResponse, error) {
	messages := make([]ChatMessage, 0, len(history)+2)
	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	for _, h := range history {
		role := strings.ToLower(strings.TrimSpace(h.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(h.Content)
		if content == "" {
			continue
		}
		messages = append(messages, ChatMessage{Role: role, Content: content})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: userQuery})

	// 1. Try LiteLLM Proxy
	resp, err := c.callOpenAICompatible(ctx, c.liteLLMURL+"/v1/chat/completions", c.liteLLMKey, c.modelName, messages)
	if err == nil {
		return resp, nil
	}

	// 2. Direct Fallback to Groq if key is present
	if c.groqKey != "" {
		resp, errGroq := c.callOpenAICompatible(ctx, "https://api.groq.com/openai/v1/chat/completions", c.groqKey, "llama-3.3-70b-versatile", messages)
		if errGroq == nil {
			resp.Model = "groq/" + resp.Model + " (direct fallback)"
			return resp, nil
		}
	}

	// 3. Direct Fallback to Gemini if key is present
	if c.geminiKey != "" {
		resp, errGemini := c.callGeminiDirect(ctx, systemPrompt, history, userQuery)
		if errGemini == nil {
			return resp, nil
		}
	}

	// 4. Return LiteLLM error or informative error
	return nil, fmt.Errorf("all LLM providers failed: LiteLLM proxy error: %w", err)
}

func (c *LLMClient) callOpenAICompatible(ctx context.Context, endpoint, apiKey, model string, messages []ChatMessage) (*ChatResponse, error) {
	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"temperature": 0.2,
		"max_tokens":  4096,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("endpoint %s returned status %d: %s", endpoint, resp.StatusCode, string(respBytes))
	}

	var chatCompletion struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&chatCompletion); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatCompletion.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	return &ChatResponse{
		Answer:       strings.TrimSpace(chatCompletion.Choices[0].Message.Content),
		Model:        chatCompletion.Model,
		TotalTokens:  chatCompletion.Usage.TotalTokens,
		PromptTokens: chatCompletion.Usage.PromptTokens,
	}, nil
}

func (c *LLMClient) callGeminiDirect(ctx context.Context, systemPrompt string, history []ChatMessage, userQuery string) (*ChatResponse, error) {
	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent?key=%s", c.geminiKey)

	contents := make([]map[string]interface{}, 0, len(history)+1)
	for _, h := range history {
		role := "user"
		if strings.ToLower(strings.TrimSpace(h.Role)) == "assistant" {
			role = "model"
		}
		cText := strings.TrimSpace(h.Content)
		if cText != "" {
			contents = append(contents, map[string]interface{}{
				"role": role,
				"parts": []map[string]interface{}{
					{"text": cText},
				},
			})
		}
	}
	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]interface{}{
			{"text": userQuery},
		},
	})

	reqPayload := map[string]interface{}{
		"system_instruction": map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": systemPrompt},
			},
		},
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.2,
			"maxOutputTokens": 4096,
		},
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("direct gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("direct gemini returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount int `json:"promptTokenCount"`
			TotalTokenCount  int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned no candidates")
	}

	return &ChatResponse{
		Answer:       strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text),
		Model:        "gemini-1.5-flash (direct fallback)",
		TotalTokens:  geminiResp.UsageMetadata.TotalTokenCount,
		PromptTokens: geminiResp.UsageMetadata.PromptTokenCount,
	}, nil
}
