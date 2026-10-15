package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Section represents a parsed section from the MediaWiki API.
type Section struct {
	TocLevel int    `json:"toclevel"`
	Level    string `json:"level"`
	Line     string `json:"line"`
	Number   string `json:"number"`
	Index    string `json:"index"`
}

// WikiPage represents the fetched MediaWiki page.
type WikiPage struct {
	DatasetCode string    `json:"dataset_code,omitempty"`
	GatheredAt  string    `json:"gathered_at,omitempty"`
	Title       string    `json:"title"`
	PageID      int       `json:"pageid"`
	Wikitext    string    `json:"wikitext"`
	Sections    []Section `json:"sections"`
}

type parseResponse struct {
	Parse struct {
		Title    string `json:"title"`
		PageID   int    `json:"pageid"`
		Wikitext struct {
			Content string `json:"*"`
		} `json:"wikitext"`
		Sections []Section `json:"sections"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

type prefixSearchResponse struct {
	Query struct {
		PrefixSearch []struct {
			Title  string `json:"title"`
			PageID int    `json:"pageid"`
		} `json:"prefixsearch"`
	} `json:"query"`
}

// Client interacts with the Fandom MediaWiki API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new MediaWiki API fetcher client.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://wutheringwaves.fandom.com/api.php"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
		userAgent: "WuwaLoreRAG/1.0 (https://github.com/ajieblr-dev/wuwaRAG; lore-assistant@local)",
	}
}

// FetchPage fetches full wikitext and section metadata for a given page title.
func (c *Client) FetchPage(ctx context.Context, title string) (*WikiPage, error) {
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", title)
	params.Set("prop", "wikitext|sections")
	params.Set("format", "json")
	params.Set("redirects", "1")

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page %q: %w", title, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var parsed parseResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if parsed.Error != nil {
		return nil, fmt.Errorf("mediawiki error [%s]: %s", parsed.Error.Code, parsed.Error.Info)
	}

	return &WikiPage{
		Title:    parsed.Parse.Title,
		PageID:   parsed.Parse.PageID,
		Wikitext: parsed.Parse.Wikitext.Content,
		Sections: parsed.Parse.Sections,
	}, nil
}

// FetchSection fetches the wikitext of a specific section of a page.
func (c *Client) FetchSection(ctx context.Context, title string, sectionIndex int) (string, error) {
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", title)
	params.Set("section", fmt.Sprintf("%d", sectionIndex))
	params.Set("prop", "wikitext")
	params.Set("format", "json")

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch section %d of %q: %w", sectionIndex, title, err)
	}
	defer resp.Body.Close()

	var parsed parseResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if parsed.Error != nil {
		return "", fmt.Errorf("mediawiki error [%s]: %s", parsed.Error.Code, parsed.Error.Info)
	}

	return parsed.Parse.Wikitext.Content, nil
}

// SearchPages searches for pages matching a prefix or keyword.
func (c *Client) SearchPages(ctx context.Context, query string) ([]string, error) {
	params := url.Values{}
	params.Set("action", "query")
	params.Set("list", "prefixsearch")
	params.Set("pssearch", query)
	params.Set("pslimit", "10")
	params.Set("format", "json")

	reqURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search pages: %w", err)
	}
	defer resp.Body.Close()

	var searchResp prefixSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	titles := make([]string, 0, len(searchResp.Query.PrefixSearch))
	for _, item := range searchResp.Query.PrefixSearch {
		if strings.TrimSpace(item.Title) != "" {
			titles = append(titles, item.Title)
		}
	}

	return titles, nil
}

// SavePageToDisk saves a fetched WikiPage to disk as pretty JSON.
func (c *Client) SavePageToDisk(page *WikiPage, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %q: %w", destDir, err)
	}

	sanitized := strings.ReplaceAll(page.Title, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "/", "_")
	filePath := filepath.Join(destDir, sanitized+".json")

	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal page: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write page to %q: %w", filePath, err)
	}

	return filePath, nil
}

// LoadPageFromDisk loads a WikiPage from a local JSON file.
func LoadPageFromDisk(filePath string) (*WikiPage, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", filePath, err)
	}

	var page WikiPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wiki page: %w", err)
	}

	return &page, nil
}

