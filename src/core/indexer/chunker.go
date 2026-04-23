package indexer

import (
	"strings"
)

const (
	MinChunkTokens = 400
	MaxChunkTokens = 800
)

// Split divides the content into indexable chunks.
// It recursively splits on \n\n, \n, and spaces.
func Split(content string, sourceType string) []string {
	return recursiveSplit(content, []string{"\n\n", "\n", " "})
}

func recursiveSplit(text string, delimiters []string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	tokens := estimateTokens(text)
	if tokens <= MaxChunkTokens || len(delimiters) == 0 {
		return []string{text}
	}

	delimiter := delimiters[0]
	parts := strings.Split(text, delimiter)
	
	var chunks []string
	var currentChunk []string
	currentTokens := 0

	for _, part := range parts {
		partTokens := estimateTokens(part)
		
		if currentTokens+partTokens > MaxChunkTokens && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, delimiter))
			currentChunk = nil
			currentTokens = 0
		}
		
		if partTokens > MaxChunkTokens {
			// Part itself is too large, split it further
			subChunks := recursiveSplit(part, delimiters[1:])
			chunks = append(chunks, subChunks...)
		} else {
			currentChunk = append(currentChunk, part)
			currentTokens += partTokens + 1 // +1 for the delimiter
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, delimiter))
	}

	return chunks
}

func estimateTokens(text string) int {
	// Simple word count as token estimate
	return len(strings.Fields(text))
}
