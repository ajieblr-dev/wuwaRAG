package parser

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// Chunk represents a clean text segment ready for vector embedding and retrieval.
type Chunk struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Section    string                 `json:"section"`
	Subsection string                 `json:"subsection,omitempty"`
	Content    string                 `json:"content"`
	Speakers   []string               `json:"speakers,omitempty"`
	Index      int                    `json:"index"`
	CharCount  int                    `json:"char_count"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// Regex patterns for wikitext cleaning.
var (
	reComments        = regexp.MustCompile(`(?s)<!--.*?-->`)
	reRefTags         = regexp.MustCompile(`(?s)<ref[^>]*>.*?</ref>`)
	reHTMLTags        = regexp.MustCompile(`<[^>]+>`)
	reWikiFiles       = regexp.MustCompile(`(?i)\[\[(File|Image|Category):[^\]]+\]\]`)
	rePipedLinks      = regexp.MustCompile(`\[\[(?:[^|\]]+\|)?([^\]]+)\]\]`)
	reExternalLinks   = regexp.MustCompile(`\[https?://[^\s\]]+(?:\s+([^\]]+))?\]`)
	reAudioTemplates  = regexp.MustCompile(`\{\{A\|[^\}]*\}\}`)
	reIconTemplates   = regexp.MustCompile(`\{\{Icon\|([^\}]+)\}\}`)
	reRoverTemplate   = regexp.MustCompile(`\{\{Rover\}\}`)
	reDIconTemplate   = regexp.MustCompile(`\{\{DIcon(?:\|[^\}]*)?\}\}`)
	reChoiceTemplate  = regexp.MustCompile(`\{Choice\}`)
	reBlackScreen     = regexp.MustCompile(`(?i)\{\{Black Screen\|([^\}]+)\}\}`)
	reQuestDesc       = regexp.MustCompile(`(?i)\{\{Quest Description\|([^\}]+)\}\}`)
	reWavesLineStart  = regexp.MustCompile(`(?i)\{\{WavesLine\s*\|\s*text\s*=\s*`)
	reGenericTemplate = regexp.MustCompile(`(?s)\{\{[^\}]+\}\}`)
	reBoldItalic      = regexp.MustCompile(`'{4,5}`)
	reBold            = regexp.MustCompile(`'''`)
	reItalic          = regexp.MustCompile(`''`)
	reSpeakerLine     = regexp.MustCompile(`(?m)^[:*#\s]*\*{0,2}(?:\(([^)]+)\)|([A-Za-z0-9\s'’\-]+))\*{0,2}\s*:\s*(.*)$`)
	reMultipleNewlines= regexp.MustCompile(`\n{3,}`)
)

// CleanWikitext cleans raw MediaWiki markup into readable natural text.
func CleanWikitext(text string) string {
	// Remove HTML comments and reference tags
	text = reComments.ReplaceAllString(text, "")
	text = reRefTags.ReplaceAllString(text, "")

	// Handle specific game wiki templates before generic template removal
	text = reRoverTemplate.ReplaceAllString(text, "Rover")
	text = reDIconTemplate.ReplaceAllString(text, "[Dialogue Option] ")
	text = reChoiceTemplate.ReplaceAllString(text, "[Choice] ")
	text = reAudioTemplates.ReplaceAllString(text, "")
	text = reIconTemplates.ReplaceAllString(text, "$1")
	text = reBlackScreen.ReplaceAllString(text, "[Scene: $1]")
	text = reQuestDesc.ReplaceAllString(text, "Quest Objective: $1\n")

	// Unwrap WavesLine templates
	text = reWavesLineStart.ReplaceAllString(text, "")

	// Remove media attachments
	text = reWikiFiles.ReplaceAllString(text, "")

	// Unwrap piped wiki links: [[Target|Anchor]] -> Anchor, [[Target]] -> Target
	text = rePipedLinks.ReplaceAllString(text, "$1")

	// Unwrap external links
	text = reExternalLinks.ReplaceAllString(text, "$1")

	// Remove remaining unmatched templates (e.g. {{Stub|...}}, {{Quest Infobox|...}})
	// We run this iteratively to handle nested templates if any
	for i := 0; i < 3; i++ {
		prev := text
		text = reGenericTemplate.ReplaceAllString(text, "")
		if text == prev {
			break
		}
	}

	// Remove remaining raw HTML tags (e.g. <div>, <br>)
	text = reHTMLTags.ReplaceAllString(text, " ")

	// Normalize wiki bold/italic formatting to readable text
	text = reBoldItalic.ReplaceAllString(text, "")
	text = reBold.ReplaceAllString(text, "")
	text = reItalic.ReplaceAllString(text, "")

	// Clean wiki list / dialogue markers at beginning of lines
	lines := strings.Split(text, "\n")
	cleanedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "----" {
			if len(cleanedLines) > 0 && cleanedLines[len(cleanedLines)-1] != "" {
				cleanedLines = append(cleanedLines, "")
			}
			continue
		}

		// MediaWiki objective / section marker: ;Text -> Objective: Text
		if strings.HasPrefix(trimmed, ";") {
			objText := strings.TrimSpace(strings.TrimPrefix(trimmed, ";"))
			cleanedLines = append(cleanedLines, fmt.Sprintf("\n[Objective: %s]", objText))
			continue
		}

		// Clean dialogue leading colons (e.g. ::'''Speaker:''' -> Speaker: ...)
		trimmed = strings.TrimLeft(trimmed, ":*# ")
		cleanedLines = append(cleanedLines, trimmed)
	}

	result := strings.Join(cleanedLines, "\n")
	result = reMultipleNewlines.ReplaceAllString(result, "\n\n")
	return strings.TrimSpace(result)
}

// ExtractSpeakers parses speaker names from cleaned or semi-cleaned dialogue lines.
func ExtractSpeakers(text string) []string {
	seen := make(map[string]bool)
	var speakers []string

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		matches := reSpeakerLine.FindStringSubmatch(line)
		if len(matches) > 0 {
			name := strings.TrimSpace(matches[1])
			if name == "" {
				name = strings.TrimSpace(matches[2])
			}
			name = strings.Trim(name, "*:() ")
			// Filter out common false positives
			if name != "" && len(name) < 40 && !strings.HasPrefix(name, "http") && !strings.EqualFold(name, "scene") && !strings.EqualFold(name, "objective") {
				if !seen[name] {
					seen[name] = true
					speakers = append(speakers, name)
				}
			}
		}
	}

	return speakers
}

// SectionHeader represents a parsed section header in wikitext.
type SectionHeader struct {
	Level int
	Title string
	Pos   int
}

// Heading regexp for == Title ==
var reHeading = regexp.MustCompile(`(?m)^(={2,5})([^=\n]+)(={2,5})$`)

// ParseIntoChunks splits full wikitext of a page into structured, context-rich chunks.
func ParseIntoChunks(pageTitle, rawWikitext string, targetChunkSize, chunkOverlap int) []Chunk {
	if targetChunkSize <= 0 {
		targetChunkSize = 900
	}
	if chunkOverlap <= 0 {
		chunkOverlap = 150
	}

	// 1. Split wikitext into primary sections by == Heading ==
	matches := reHeading.FindAllStringSubmatchIndex(rawWikitext, -1)
	type sectionBlock struct {
		title string
		level int
		body  string
	}

	var sections []sectionBlock
	lastIndex := 0
	currentTitle := "Introduction"
	currentLevel := 1

	for _, m := range matches {
		openLen := m[3] - m[2]
		closeLen := m[7] - m[6]
		if openLen != closeLen {
			continue
		}
		headerText := strings.TrimSpace(rawWikitext[m[4]:m[5]])
		level := openLen
		body := rawWikitext[lastIndex:m[0]]

		if strings.TrimSpace(body) != "" {
			sections = append(sections, sectionBlock{
				title: currentTitle,
				level: currentLevel,
				body:  body,
			})
		}

		currentTitle = headerText
		currentLevel = level
		lastIndex = m[1]
	}

	if lastIndex < len(rawWikitext) {
		body := rawWikitext[lastIndex:]
		if strings.TrimSpace(body) != "" {
			sections = append(sections, sectionBlock{
				title: currentTitle,
				level: currentLevel,
				body:  body,
			})
		}
	}

	var allChunks []Chunk
	chunkIndex := 0

	for _, sec := range sections {
		// Skip trivial / non-lore sections like Gallery, Navigation, Other Languages
		titleLower := strings.ToLower(sec.title)
		if strings.Contains(titleLower, "gallery") ||
			strings.Contains(titleLower, "navigation") ||
			strings.Contains(titleLower, "other languages") ||
			strings.Contains(titleLower, "change history") {
			continue
		}

		cleanedBody := CleanWikitext(sec.body)
		if len(strings.TrimSpace(cleanedBody)) == 0 {
			continue
		}

		// Split by scene/objective markers if present
		sceneBlocks := splitByObjectives(cleanedBody)

		for _, scene := range sceneBlocks {
			sceneText := strings.TrimSpace(scene.text)
			if len(sceneText) == 0 {
				continue
			}

			// If sceneText is larger than targetChunkSize, chunk with sliding window
			textChunks := chunkText(sceneText, targetChunkSize, chunkOverlap)
			for _, chunkStr := range textChunks {
				chunkStr = strings.TrimSpace(chunkStr)
				// Skip very short fragments unless they contain dialogue
				if len(chunkStr) < 50 && !strings.Contains(chunkStr, ":") {
					continue
				}
				// Skip cutscene boilerplate lines without dialogue
				if strings.HasPrefix(chunkStr, "[Objective: (Cutscene") {
					continue
				}

				speakers := ExtractSpeakers(chunkStr)
				deterministicID := generateDeterministicUUID(fmt.Sprintf("%s:%s:%d:%s", pageTitle, sec.title, chunkIndex, chunkStr[:min(len(chunkStr), 50)]))

				allChunks = append(allChunks, Chunk{
					ID:         deterministicID,
					Title:      pageTitle,
					Section:    sec.title,
					Subsection: scene.objective,
					Content:    chunkStr,
					Speakers:   speakers,
					Index:      chunkIndex,
					CharCount:  len(chunkStr),
					Metadata: map[string]interface{}{
						"page_title":  pageTitle,
						"section":     sec.title,
						"subsection":  scene.objective,
						"speakers":    speakers,
						"chunk_index": chunkIndex,
						"char_count":  len(chunkStr),
					},
				})
				chunkIndex++
			}
		}
	}

	return allChunks
}

type sceneBlock struct {
	objective string
	text      string
}

func splitByObjectives(text string) []sceneBlock {
	lines := strings.Split(text, "\n")
	var blocks []sceneBlock
	currentObj := ""
	var currentLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[Objective:") && strings.HasSuffix(trimmed, "]") {
			if len(currentLines) > 0 {
				blocks = append(blocks, sceneBlock{
					objective: currentObj,
					text:      strings.Join(currentLines, "\n"),
				})
				currentLines = nil
			}
			currentObj = strings.TrimSuffix(strings.TrimPrefix(trimmed, "[Objective:"), "]")
			currentObj = strings.TrimSpace(currentObj)
			currentLines = append(currentLines, line)
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if len(currentLines) > 0 {
		blocks = append(blocks, sceneBlock{
			objective: currentObj,
			text:      strings.Join(currentLines, "\n"),
		})
	}

	return blocks
}

func chunkText(text string, maxChars, overlap int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if current.Len()+len(p)+2 > maxChars && current.Len() > 0 {
			chunks = append(chunks, current.String())
			// Carry over overlap if possible
			currStr := current.String()
			current.Reset()
			if overlap > 0 && len(currStr) > overlap {
				tail := currStr[len(currStr)-overlap:]
				// find word boundary
				if idx := strings.Index(tail, "\n"); idx != -1 {
					current.WriteString(strings.TrimSpace(tail[idx:]))
					current.WriteString("\n\n")
				}
			}
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}

	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}

	return chunks
}

func generateDeterministicUUID(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	u, err := uuid.FromBytes(hash[:16])
	if err != nil {
		return uuid.New().String()
	}
	return u.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
