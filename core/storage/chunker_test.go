package storage

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestChunkFile(t *testing.T) {
	// Create a temporary file of 1MB + 10 bytes to test remainder
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "testfile.dat")
	
	fileSize := (1024 * 1024) + 10 // 1MB + 10 bytes
	data := make([]byte, fileSize)
	rand.Read(data)
	
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	meta, err := ChunkFile(filePath)
	if err != nil {
		t.Fatalf("ChunkFile failed: %v", err)
	}

	if meta.Size != int64(fileSize) {
		t.Errorf("Expected size %d, got %d", fileSize, meta.Size)
	}

	expectedChunks := (fileSize / DefaultChunkSize) + 1
	if len(meta.Chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(meta.Chunks))
	}

	// Verify the last chunk size is exactly 10 bytes
	lastChunk := meta.Chunks[len(meta.Chunks)-1]
	if lastChunk.Size != 10 {
		t.Errorf("Expected last chunk size 10, got %d", lastChunk.Size)
	}

	if meta.RootHash == "" {
		t.Error("RootHash is empty")
	}
}
