package rag

import (
	"math"
	"sort"
	"strings"

	"github.com/ajieblr-dev/wuwaRAG/internal/qdrant"
)

// CandidateDoc represents a candidate document during hybrid search and reranking.
type CandidateDoc struct {
	Snippet      SourceSnippet
	VectorScore  float64
	VectorRank   int
	LexicalScore float64
	LexicalRank  int
	RRFScore     float64
	FinalScore   float64
}

// ScorePointLexical calculates a lexical/keyword score for a point based on query terms.
func ScorePointLexical(p qdrant.ScoredPoint, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0.0
	}

	payload := p.Payload
	title, _ := payload["title"].(string)
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
	speakersText := strings.Join(speakers, " ")

	titleLower := strings.ToLower(title)
	speakersLower := strings.ToLower(speakersText)
	subsectionLower := strings.ToLower(subsection)
	contentLower := strings.ToLower(content)

	var score float64
	matchedKeywords := 0

	for _, kw := range keywords {
		kwLower := strings.ToLower(strings.TrimSpace(kw))
		if kwLower == "" {
			continue
		}

		matched := false
		if strings.Contains(titleLower, kwLower) {
			score += 2.0
			matched = true
		}
		if strings.Contains(speakersLower, kwLower) {
			score += 3.0
			matched = true
		}
		if strings.Contains(subsectionLower, kwLower) {
			score += 2.0
			matched = true
		}
		if count := strings.Count(contentLower, kwLower); count > 0 {
			// Sub-linear term frequency saturation with strong base for content matches
			score += 3.0 + 2.0*math.Log1p(float64(count))
			matched = true
		}

		if matched {
			matchedKeywords++
		}
	}

	// Reward documents that match multiple distinct keywords
	if matchedKeywords > 1 {
		score *= (1.0 + 0.50*float64(matchedKeywords))
	}

	return score
}

// ConvertPointToSnippet converts a Qdrant point to a SourceSnippet.
func ConvertPointToSnippet(p qdrant.ScoredPoint) SourceSnippet {
	payload := p.Payload
	title, _ := payload["title"].(string)
	section, _ := payload["section"].(string)
	subsection, _ := payload["subsection"].(string)
	content, _ := payload["content"].(string)
	datasetCode, _ := payload["dataset_code"].(string)
	if datasetCode == "" {
		datasetCode, _ = payload["datasetCode"].(string)
	}

	var speakers []string
	if rawSpeakers, ok := payload["speakers"].([]interface{}); ok {
		for _, sp := range rawSpeakers {
			if spStr, ok := sp.(string); ok {
				speakers = append(speakers, spStr)
			}
		}
	}

	return SourceSnippet{
		ID:           qdrantPointIDToString(p.ID),
		DatasetCode:  datasetCode,
		Title:        title,
		PageTitle:    title,
		Section:      section,
		SectionTitle: section,
		Subsection:   subsection,
		Speakers:     speakers,
		Content:      content,
		ChunkText:    content,
		Score:        p.Score,
	}
}

func qdrantPointIDToString(id interface{}) string {
	if s, ok := id.(string); ok {
		return s
	}
	return ""
}

// ReciprocalRankFusion performs RRF between vector candidates and keyword candidates.
func ReciprocalRankFusion(
	vectorPoints []qdrant.ScoredPoint,
	lexicalPoints []qdrant.ScoredPoint,
	keywords []string,
	k int,
	topK int,
) []SourceSnippet {
	if k <= 0 {
		k = 60
	}
	if topK <= 0 {
		topK = 8
	}

	candidates := make(map[string]*CandidateDoc)

	// 1. Process Vector Search results (assign ranks 1..N)
	for rank, vp := range vectorPoints {
		id := qdrantPointIDToString(vp.ID)
		if id == "" {
			continue
		}
		candidates[id] = &CandidateDoc{
			Snippet:     ConvertPointToSnippet(vp),
			VectorScore: vp.Score,
			VectorRank:  rank + 1,
		}
	}

	// 2. Score and sort Lexical Search results
	type scoredLex struct {
		point qdrant.ScoredPoint
		score float64
	}
	var lexList []scoredLex
	for _, lp := range lexicalPoints {
		score := ScorePointLexical(lp, keywords)
		lexList = append(lexList, scoredLex{point: lp, score: score})
	}
	sort.Slice(lexList, func(i, j int) bool {
		return lexList[i].score > lexList[j].score
	})

	// 3. Process Lexical results (assign ranks 1..M)
	for rank, item := range lexList {
		id := qdrantPointIDToString(item.point.ID)
		if id == "" {
			continue
		}
		if cand, exists := candidates[id]; exists {
			cand.LexicalScore = item.score
			cand.LexicalRank = rank + 1
		} else {
			snip := ConvertPointToSnippet(item.point)
			candidates[id] = &CandidateDoc{
				Snippet:      snip,
				VectorScore:  0,
				VectorRank:   0, // not in vector top results
				LexicalScore: item.score,
				LexicalRank:  rank + 1,
			}
		}
	}

	// 4. Calculate RRF scores and composite final ranking
	var docList []*CandidateDoc
	for _, doc := range candidates {
		rrf := 0.0
		if doc.VectorRank > 0 {
			rrf += 1.0 / float64(k+doc.VectorRank)
		}
		if doc.LexicalRank > 0 {
			rrf += 1.0 / float64(k+doc.LexicalRank)
		}
		doc.RRFScore = rrf

		// Composite ranking: RRF (primary) + Vector cosine weight + Lexical keyword bonus
		doc.FinalScore = rrf*10.0 + (doc.VectorScore * 0.25) + math.Min(doc.LexicalScore*0.08, 0.40)
		docList = append(docList, doc)
	}

	// Sort descending by FinalScore
	sort.Slice(docList, func(i, j int) bool {
		return docList[i].FinalScore > docList[j].FinalScore
	})

	// 5. Select topK results with content deduplication
	seenContent := make(map[string]bool)
	var results []SourceSnippet

	for _, doc := range docList {
		if len(results) >= topK {
			break
		}

		contentKey := strings.TrimSpace(doc.Snippet.Content)
		if len(contentKey) > 160 {
			contentKey = contentKey[:160]
		}
		fingerprint := doc.Snippet.Title + "::" + contentKey
		if seenContent[fingerprint] {
			continue // skip duplicate chunk
		}
		seenContent[fingerprint] = true

		snip := doc.Snippet
		// Format score between 0.0 and 1.0 for UI display
		snip.Score = math.Min(1.0, math.Round(doc.FinalScore*100)/100)
		results = append(results, snip)
	}

	return results
}
