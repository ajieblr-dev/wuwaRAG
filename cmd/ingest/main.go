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
	gatherOnly := flag.Bool("gather-only", false, "Only fetch raw pages to data/raw without embedding/indexing")
	dryRun := flag.Bool("dry-run", false, "Parse and preview chunks without writing to Qdrant")
	fromCache := flag.Bool("from-cache", false, "Load pages from data/raw cache instead of fetching live")
	chunkSize := flag.Int("chunk-size", 900, "Target chunk size in characters")
	overlap := flag.Int("overlap", 150, "Overlap in characters when splitting large scenes")
	rawDir := flag.String("raw-dir", "data/raw", "Directory for raw wiki JSON dumps")
	outFile := flag.String("out", "data/processed/chunks.json", "Destination for processed chunks JSON")
	flag.Parse()

	cfg := config.Load()

	// Resolve page list: explicit flags win, otherwise config default.
	pagesRaw := firstNonEmpty(*pagesFlag, *pageFlag, flag.Arg(0), cfg.DefaultWikiPage)
	pages := splitPages(pagesRaw)
	if len(pages) == 0 {
		pages = []string{"Simulacrum_of_the_Heart"}
	}

	fetchClient := fetcher.NewClient(cfg.WikiAPIURL)
	embedder := rag.NewHuggingFaceEmbedder(
		cfg.HuggingFaceKey,
		cfg.EmbeddingModel,
		cfg.EmbeddingBaseURL,
		cfg.VectorDim,
	)
	qdrantClient := qdrant.NewClient(cfg.QdrantURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var allChunks []parser.Chunk

	for _, title := range pages {
		var page *fetcher.WikiPage
		var err error

		if *fromCache {
			cachedPath := filepath.Join(*rawDir, sanitizeTitle(title)+".json")
			page, err = fetcher.LoadPageFromDisk(cachedPath)
			if err != nil {
				log.Printf("cache miss for %q (%v), fetching live", title, err)
			}
		}
		if page == nil {
			log.Printf("fetching page %q...", title)
			page, err = fetchClient.FetchPage(ctx, title)
			if err != nil {
				log.Printf("ERROR: failed to fetch %q: %v", title, err)
				continue
			}
		}

		savedPath, err := fetchClient.SavePageToDisk(page, *rawDir)
		if err != nil {
			log.Printf("WARNING: failed to save raw page %q: %v", title, err)
		} else {
			log.Printf("saved raw page %q -> %s (%d chars wikitext)", page.Title, savedPath, len(page.Wikitext))
		}

		if *gatherOnly {
			continue
		}

		chunks := parser.ParseIntoChunks(page.Title, page.Wikitext, *chunkSize, *overlap)
		log.Printf("page %q: parsed %d chunks", page.Title, len(chunks))
		allChunks = append(allChunks, chunks...)
	}

	if *gatherOnly {
		log.Printf("gather-only complete: %d page(s) saved to %s", len(pages), *rawDir)
		return
	}

	if len(allChunks) == 0 {
		log.Fatal("no chunks produced; nothing to index")
	}

	// Persist processed chunks for reproducible offline runs.
	if err := saveChunks(allChunks, *outFile); err != nil {
		log.Printf("WARNING: failed to write %s: %v", *outFile, err)
	} else {
		log.Printf("wrote %d chunks -> %s", len(allChunks), *outFile)
	}

	if *dryRun {
		previewChunks(allChunks)
		log.Printf("dry-run complete: %d chunk(s), Qdrant untouched", len(allChunks))
		return
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
