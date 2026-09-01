package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

const DefaultChunkSize = 256 * 1024 // 256 KB

type ChunkInfo struct {
	Index  int
	Offset int64
	Size   int
	Hash   string
}

type FileMetadata struct {
	Path     string
	Size     int64
	RootHash string
	Chunks   []ChunkInfo
}

// ChunkFile reads a file and splits it into fixed-size chunks, hashing each one.
func ChunkFile(path string) (*FileMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	metadata := &FileMetadata{
		Path: path,
		Size: stat.Size(),
	}

	buf := make([]byte, DefaultChunkSize)
	hasher := sha256.New()
	rootHasher := sha256.New()
	
	var offset int64 = 0
	var index = 0

	for {
		bytesRead, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		if bytesRead == 0 {
			break
		}

		// Hash the chunk
		hasher.Reset()
		hasher.Write(buf[:bytesRead])
		chunkHash := hasher.Sum(nil)
		hashStr := hex.EncodeToString(chunkHash)

		// Add to root hasher
		rootHasher.Write(chunkHash)

		metadata.Chunks = append(metadata.Chunks, ChunkInfo{
			Index:  index,
			Offset: offset,
			Size:   bytesRead,
			Hash:   hashStr,
		})

		offset += int64(bytesRead)
		index++
		
		if err == io.EOF {
			break
		}
	}

	// Calculate Root Hash from all chunk hashes
	metadata.RootHash = hex.EncodeToString(rootHasher.Sum(nil))

	return metadata, nil
}
