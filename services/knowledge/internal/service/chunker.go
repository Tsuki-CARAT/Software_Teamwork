package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	defaultChunkSize    = 1600
	defaultChunkOverlap = 200
	minChunkSize        = 64
	maxChunkSize        = 8000
	maxChunkOverlap     = 2000
)

type FixedChunker struct{}

func NewFixedChunker() *FixedChunker {
	return &FixedChunker{}
}

func (c *FixedChunker) Chunk(ctx context.Context, input ChunkInput) ([]ChunkSpec, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, fmt.Errorf("document content is empty")
	}
	size, overlap := chunkSettings(input.Strategy)
	runes := []rune(content)
	if len(runes) <= size {
		kind := "text"
		return []ChunkSpec{{
			Content:    content,
			TokenCount: estimateTokenCount(content),
			ChunkType:  &kind,
			Metadata:   map[string]any{},
		}}, nil
	}
	step := size - overlap
	if step <= 0 {
		step = size
	}
	chunks := []ChunkSpec{}
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		text := strings.TrimSpace(string(runes[start:end]))
		if text != "" {
			kind := "text"
			chunks = append(chunks, ChunkSpec{
				Content:    text,
				TokenCount: estimateTokenCount(text),
				ChunkType:  &kind,
				Metadata:   map[string]any{},
			})
		}
		if end == len(runes) {
			break
		}
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("document content is empty")
	}
	return chunks, nil
}

func chunkSettings(raw json.RawMessage) (int, int) {
	settings := struct {
		Size      int `json:"size"`
		ChunkSize int `json:"chunkSize"`
		Overlap   int `json:"overlap"`
	}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &settings)
	}
	size := settings.Size
	if size == 0 {
		size = settings.ChunkSize
	}
	if size == 0 {
		size = defaultChunkSize
	}
	if size < minChunkSize {
		size = minChunkSize
	}
	if size > maxChunkSize {
		size = maxChunkSize
	}
	overlap := settings.Overlap
	if overlap < 0 {
		overlap = 0
	}
	if overlap > maxChunkOverlap {
		overlap = maxChunkOverlap
	}
	if overlap >= size {
		overlap = size / 4
	}
	return size, overlap
}

func estimateTokenCount(content string) int {
	runeCount := utf8.RuneCountInString(content)
	if runeCount == 0 {
		return 0
	}
	estimate := runeCount / 4
	if estimate == 0 {
		return 1
	}
	return estimate
}
