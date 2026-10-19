// Command ingest fetches MediaWiki lore pages, parses them into chunks,
// generates embeddings, and upserts the vectors into Qdrant.
//
// Usage:
//
//	go run ./cmd/ingest --pages "Simulacrum_of_the_Heart,Hsin" [--gather-only] [--dry-run] [--from-cache]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ajieblr-dev/wuwaRAG/internal/config"
	"github.com/ajieblr-dev/wuwaRAG/internal/fetcher"
	"github.com/ajieblr-dev/wuwaRAG/internal/parser"
	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
	"github.com/ajieblr-dev/wuwaRAG/internal/rag"
)

func main() {
	pagesFlag := flag.String("pages", "", "Comma-separated MediaWiki page titles to ingest")
	pageFlag := flag.String("page", "", "Single MediaWiki page title (alternative to --pages)")
	pagesFile := flag.String("pages-file", "", "Path to file containing page titles (one per line)")
	allMainQuests := flag.Bool("all-main-quests", false, "Ingest all main quests from Prologue to Chapter IV (using data/main_quests.txt)")
	gatherOnly := flag.Bool("gather-only", false, "Only fetch raw pages to data/raw without embedding/indexing")
	dryRun := flag.Bool("dry-run", false, "Parse and preview chunks without writing to Qdrant")
	fromCache := flag.Bool("from-cache", false, "Load pages from data/raw cache instead of fetching live")
	chunkSize := flag.Int("chunk-size", 900, "Target chunk size in characters")
	overlap := flag.Int("overlap", 150, "Overlap in characters when splitting large scenes")
	rawDir := flag.String("raw-dir", "data/raw", "Directory for raw wiki JSON dumps")
	outFile := flag.String("out", "data/processed/chunks.json", "Destination for processed chunks JSON")
	datasetCode := flag.String("dataset-code", "lore-corpus-v1", "Code name identifier for this batch of data")
	concurrency := flag.Int("concurrency", 1, "Number of concurrent workers for live fetching/processing")
	timeout := flag.Duration("timeout", 45*time.Minute, "Overall context timeout for ingestion")
	flag.Parse()

	cfg := config.Load()

	// Resolve page list: explicit flags, file, or default.
	var pages []string
	if *allMainQuests {
		targetFile := *pagesFile
		if targetFile == "" {
			targetFile = "data/main_quests.txt"
		}
		loaded, err := loadPagesFromFile(targetFile)
		if err != nil {
			log.Fatalf("failed to load main quests from %s: %v", targetFile, err)
		}
		pages = loaded
	} else if *pagesFile != "" {
		loaded, err := loadPagesFromFile(*pagesFile)
		if err != nil {
			log.Fatalf("failed to load pages from %s: %v", *pagesFile, err)
		}
		pages = loaded
	} else {
		pagesRaw := firstNonEmpty(*pagesFlag, *pageFlag, flag.Arg(0), cfg.DefaultWikiPage)
		pages = splitPages(pagesRaw)
	}

	if len(pages) == 0 {
		pages = []string{"Simulacrum_of_the_Heart"}
	}

	log.Printf("resolved %d page(s) for processing", len(pages))

	fetchClient := fetcher.NewClient(cfg.WikiAPIURL)
	embedder := rag.NewHuggingFaceEmbedder(
		cfg.HuggingFaceKey,
		cfg.EmbeddingModel,
		cfg.EmbeddingBaseURL,
		cfg.VectorDim,
	)
	qdrantClient := qdrant.NewClient(cfg.QdrantURL)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var allChunks []parser.Chunk

	processPage := func(i int, title string) []parser.Chunk {
		var page *fetcher.WikiPage
		var err error

		if *fromCache {
			cachedPath := filepath.Join(*rawDir, sanitizeTitle(title)+".json")
			page, err = fetcher.LoadPageFromDisk(cachedPath)
			if err != nil {
				log.Printf("[%d/%d] cache miss for %q (%v), fetching live", i+1, len(pages), title, err)
			}
		}
		if page == nil {
			log.Printf("[%d/%d] fetching page %q...", i+1, len(pages), title)
			page, err = fetchClient.FetchPage(ctx, title)
			if err != nil {
				log.Printf("ERROR: [%d/%d] failed to fetch %q: %v", i+1, len(pages), title, err)
				return nil
			}
			if *datasetCode != "" {
				page.DatasetCode = *datasetCode
				page.GatheredAt = time.Now().UTC().Format(time.RFC3339)
			}
			// Polite delay between live API queries
			time.Sleep(100 * time.Millisecond)
		} else if *datasetCode != "" && page.DatasetCode == "" {
			page.DatasetCode = *datasetCode
		}

		savedPath, err := fetchClient.SavePageToDisk(page, *rawDir)
		if err != nil {
			log.Printf("WARNING: failed to save raw page %q: %v", title, err)
		} else {
			log.Printf("[%d/%d] saved raw page %q -> %s (%d chars wikitext)", i+1, len(pages), page.Title, savedPath, len(page.Wikitext))
		}

		if *gatherOnly {
			return nil
		}

		chunks := parser.ParseIntoChunks(page.Title, page.Wikitext, *chunkSize, *overlap)
		log.Printf("[%d/%d] page %q: parsed %d chunks", i+1, len(pages), page.Title, len(chunks))
		for ci := range chunks {
			code := page.DatasetCode
			if code == "" {
				code = *datasetCode
			}
			chunks[ci].DatasetCode = code
			if chunks[ci].Metadata == nil {
				chunks[ci].Metadata = make(map[string]interface{})
			}
			chunks[ci].Metadata["dataset_code"] = code
		}
		return chunks
	}

	if *concurrency > 1 {
		type job struct {
			idx   int
			title string
		}
		type res struct {
			idx    int
			chunks []parser.Chunk
		}
		jobs := make(chan job, len(pages))
		results := make(chan res, len(pages))

		for i, title := range pages {
			jobs <- job{idx: i, title: title}
		}
		close(jobs)

		var wg sync.WaitGroup
		for w := 0; w < *concurrency; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					chunks := processPage(j.idx, j.title)
					results <- res{idx: j.idx, chunks: chunks}
				}
			}()
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		chunkMap := make(map[int][]parser.Chunk)
		for r := range results {
			if len(r.chunks) > 0 {
				chunkMap[r.idx] = r.chunks
			}
		}

		if !*gatherOnly {
			for i := 0; i < len(pages); i++ {
				if chs, ok := chunkMap[i]; ok {
					allChunks = append(allChunks, chs...)
				}
			}
		}
	} else {
		for i, title := range pages {
			chs := processPage(i, title)
			if !*gatherOnly && len(chs) > 0 {
				allChunks = append(allChunks, chs...)
			}
		}
	}

	if *gatherOnly {
		log.Printf("gather-only complete: %d page(s) saved to %s", len(pages), *rawDir)
		return
	}

	if len(allChunks) == 0 {
		log.Fatal("no chunks produced; nothing to index")
	}

	if *dryRun {
		previewChunks(allChunks)
		log.Printf("dry-run complete: %d chunk(s), output and Qdrant untouched", len(allChunks))
		return
	}

	// Persist processed chunks for reproducible offline runs.
	if err := saveChunks(allChunks, *outFile); err != nil {
		log.Printf("WARNING: failed to write %s: %v", *outFile, err)
	} else {
		log.Printf("wrote %d chunks -> %s", len(allChunks), *outFile)
	}

	// Full ingestion: embed and upsert.
	if err := qdrantClient.EnsureCollection(ctx, cfg.Collection, embedder.Dimension()); err != nil {
		log.Fatalf("failed to ensure collection %q: %v", cfg.Collection, err)
	}

	texts := make([]string, len(allChunks))
	for i, ch := range allChunks {
		texts[i] = ch.Content
	}

	log.Printf("generating embeddings for %d chunks...", len(texts))
	embeddings, err := embedder.EmbedBatch(ctx, texts)
	if err != nil {
		log.Fatalf("failed to generate embeddings: %v", err)
	}
	if len(embeddings) != len(allChunks) {
		log.Fatalf("embedding count mismatch: got %d, want %d", len(embeddings), len(allChunks))
	}

	points := make([]qdrant.Point, len(allChunks))
	for i, ch := range allChunks {
		payload := map[string]interface{}{
			"title":       ch.Title,
			"section":     ch.Section,
			"subsection":  ch.Subsection,
			"content":     ch.Content,
			"speakers":    ch.Speakers,
			"chunk_index": ch.Index,
			"char_count":  ch.CharCount,
		}
		if ch.DatasetCode != "" {
			payload["dataset_code"] = ch.DatasetCode
		}
		points[i] = qdrant.Point{
			ID:      ch.ID,
			Vector:  embeddings[i],
			Payload: payload,
		}
	}

	log.Printf("upserting %d points to collection %q...", len(points), cfg.Collection)
	if err := qdrantClient.UpsertPoints(ctx, cfg.Collection, points, 50); err != nil {
		log.Fatalf("failed to upsert points: %v", err)
	}
	log.Printf("ingestion complete: %d chunks indexed into %q", len(points), cfg.Collection)
}

func splitPages(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func sanitizeTitle(title string) string {
	s := strings.ReplaceAll(title, " ", "_")
	return strings.ReplaceAll(s, "/", "_")
}

func saveChunks(chunks []parser.Chunk, dest string) error {
	if dir := filepath.Dir(dest); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(chunks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

func previewChunks(chunks []parser.Chunk) {
	const maxPreview = 3
	n := min(len(chunks), maxPreview)
	speakers := make(map[string]struct{})
	for _, ch := range chunks {
		for _, sp := range ch.Speakers {
			speakers[sp] = struct{}{}
		}
	}
	fmt.Printf("---- dry-run preview: %d chunks, %d distinct speakers ----\n", len(chunks), len(speakers))
	for i := 0; i < n; i++ {
		ch := chunks[i]
		body := ch.Content
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		fmt.Printf("[%d] %s | %s | speakers=%v\n%s\n\n", ch.Index, ch.Title, ch.Section, ch.Speakers, body)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func loadPagesFromFile(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	var pages []string
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seen[line] {
			seen[line] = true
			pages = append(pages, line)
		}
	}
	return pages, nil
}

