package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/knowledge/internal/service"
)

func TestFixedChunkerUsesKnowledgeBaseStrategy(t *testing.T) {
	chunker := service.NewFixedChunker()
	content := strings.Repeat("a", 90)

	chunks, err := chunker.Chunk(context.Background(), service.ChunkInput{
		Content:  content,
		Strategy: json.RawMessage(`{"chunkSize":64,"overlap":16}`),
	})
	if err != nil {
		t.Fatalf("Chunk() error = %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	if chunks[0].Content != strings.Repeat("a", 64) {
		t.Fatalf("first chunk length = %d", len([]rune(chunks[0].Content)))
	}
	if chunks[1].Content != strings.Repeat("a", 42) {
		t.Fatalf("second chunk length = %d", len([]rune(chunks[1].Content)))
	}
	if chunks[0].ChunkType == nil || *chunks[0].ChunkType != "text" {
		t.Fatalf("chunk type = %v", chunks[0].ChunkType)
	}
}

func TestFixedChunkerRejectsEmptyContent(t *testing.T) {
	chunker := service.NewFixedChunker()

	_, err := chunker.Chunk(context.Background(), service.ChunkInput{
		Content: " \n\t ",
	})
	if err == nil {
		t.Fatal("Chunk() error = nil, want error")
	}
}
