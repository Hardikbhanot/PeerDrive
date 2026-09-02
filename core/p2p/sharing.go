package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/peerdrive/core/storage"
)

const ShareProtocol = "/peerdrive/share/1.0.0"

type ShareManager struct {
	node *Node
	db   *storage.DB
}

type ShareRequest struct {
	Action  string `json:"action"`   // "join", "get_chunk"
	ShareID string `json:"share_id"`
	Index   int    `json:"index,omitempty"` // for get_chunk
}

type ShareResponse struct {
	Status      string   `json:"status"` // "ok", "error"
	Error       string   `json:"error,omitempty"`
	RootHash    string   `json:"root_hash,omitempty"`
	FileName    string   `json:"file_name,omitempty"`
	FileSize    int64    `json:"file_size,omitempty"`
	TotalChunks int      `json:"total_chunks,omitempty"`
	ChunkHashes []string `json:"chunk_hashes,omitempty"` // for per-chunk verification
	Data        []byte   `json:"data,omitempty"`         // for get_chunk
}

func NewShareManager(n *Node, db *storage.DB) *ShareManager {
	sm := &ShareManager{
		node: n,
		db:   db,
	}
	n.Host.SetStreamHandler(ShareProtocol, sm.handleStream)
	return sm
}

func (sm *ShareManager) handleStream(s network.Stream) {
	defer s.Close()
	
	peerID := s.Conn().RemotePeer().String()
	
	decoder := json.NewDecoder(s)
	encoder := json.NewEncoder(s)
	
	for {
		var req ShareRequest
		if err := decoder.Decode(&req); err != nil {
			fmt.Printf("[Host] Stream closed or error decoding: %v\n", err)
			return // Connection closed or error
		}
		
		fmt.Printf("[Host] Received request from peer %s for share %s\n", peerID, req.ShareID)

		// 1. Verify Authorization
		authorized, err := sm.db.IsPeerAuthorized(req.ShareID, peerID)
		if err != nil {
			fmt.Printf("[Host] DB Error checking auth: %v\n", err)
			encoder.Encode(ShareResponse{Status: "error", Error: "internal error"})
			return
		}
		
		if !authorized {
			fmt.Printf("[Host] Rejected: peer %s is not authorized for share %s\n", peerID, req.ShareID)
			encoder.Encode(ShareResponse{Status: "error", Error: "unauthorized or expired share"})
			return
		}
		
		// 2. Handle Request
		share, err := sm.db.GetShare(req.ShareID)
		if err != nil || share == nil {
			encoder.Encode(ShareResponse{Status: "error", Error: "share not found"})
			return
		}

		switch req.Action {
		case "join":
			stat, err := os.Stat(share.Path)
			if err != nil {
				encoder.Encode(ShareResponse{Status: "error", Error: "file missing"})
				return
			}
			meta, err := storage.ChunkFile(share.Path)
			if err != nil {
				encoder.Encode(ShareResponse{Status: "error", Error: "file hashing failed"})
				return
			}

			hashes := make([]string, len(meta.Chunks))
			for i, c := range meta.Chunks {
				hashes[i] = c.Hash
			}

			encoder.Encode(ShareResponse{
				Status:      "ok",
				RootHash:    share.RootHash,
				FileName:    filepath.Base(share.Path),
				FileSize:    stat.Size(),
				TotalChunks: len(meta.Chunks),
				ChunkHashes: hashes,
			})
			
		case "get_chunk":
			file, err := os.Open(share.Path)
			if err != nil {
				encoder.Encode(ShareResponse{Status: "error", Error: "failed to open file"})
				return
			}
			defer file.Close()

			offset := int64(req.Index) * storage.DefaultChunkSize
			_, err = file.Seek(offset, 0)
			if err != nil {
				encoder.Encode(ShareResponse{Status: "error", Error: "failed to seek"})
				return
			}

			buf := make([]byte, storage.DefaultChunkSize)
			n, err := file.Read(buf)
			if err != nil && err != io.EOF {
				encoder.Encode(ShareResponse{Status: "error", Error: "failed to read"})
				return
			}

			encoder.Encode(ShareResponse{
				Status: "ok",
				Data:   buf[:n],
			})
			
		default:
			encoder.Encode(ShareResponse{Status: "error", Error: "unknown action"})
		}
	}
}

func hashToCID(hashHex string) (cid.Cid, error) {
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return cid.Undef, err
	}
	mh, err := multihash.Encode(hashBytes, multihash.SHA2_256)
	if err != nil {
		return cid.Undef, err
	}
	return cid.NewCidV1(cid.Raw, mh), nil
}

// RequestShare is called by the downloading peer
func (sm *ShareManager) RequestShare(ctx context.Context, peerIDString string, shareID string, onProgress func(int64)) error {
	hostPeerID, err := peer.Decode(peerIDString)
	if err != nil {
		return fmt.Errorf("invalid host peer id: %w", err)
	}

	// 1. Contact host to get metadata (TotalChunks and ChunkHashes)
	s, err := sm.node.Host.NewStream(ctx, hostPeerID, ShareProtocol)
	if err != nil {
		return fmt.Errorf("failed to open stream to host: %w", err)
	}

	encoder := json.NewEncoder(s)
	decoder := json.NewDecoder(s)

	req := ShareRequest{Action: "join", ShareID: shareID}
	if err := encoder.Encode(&req); err != nil {
		s.Close()
		return fmt.Errorf("failed to send join request: %w", err)
	}

	var res ShareResponse
	if err := decoder.Decode(&res); err != nil {
		s.Close()
		return fmt.Errorf("failed to read join response: %w", err)
	}
	s.Close()

	if res.Status != "ok" {
		return fmt.Errorf("host rejected join: %s", res.Error)
	}

	if len(res.ChunkHashes) != res.TotalChunks {
		return fmt.Errorf("host did not provide all chunk hashes (got %d, expected %d)", len(res.ChunkHashes), res.TotalChunks)
	}

	// 2. Discover other peers (seeders) via DHT
	fmt.Printf("Discovering peers for %s...\n", shareID)
	c, err := hashToCID(res.RootHash)
	if err != nil {
		return fmt.Errorf("invalid root hash: %w", err)
	}
	
	// We always have the host. Let's find more.
	providers := sm.node.DHT.FindProvidersAsync(ctx, c, 10)
	var peers []peer.ID
	peers = append(peers, hostPeerID) // Always include the host
	
	timeout := time.After(3 * time.Second)
	collecting := true
	for collecting {
		select {
		case p, ok := <-providers:
			if !ok {
				collecting = false
			} else if p.ID != sm.node.Host.ID() && p.ID != hostPeerID {
				peers = append(peers, p.ID)
			}
		case <-timeout:
			collecting = false
		}
	}
	fmt.Printf("Found %d seeders for swarm!\n", len(peers))

	// 3. Setup Multithreaded Download
	downloadPath := filepath.Join(os.TempDir(), "peerdrive_"+res.FileName)
	out, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer out.Close()

	var downloaded int64 = 0
	if onProgress != nil {
		onProgress(downloaded)
	}
	
	var fileMu sync.Mutex
	chunkQueue := make(chan int, res.TotalChunks)
	for i := 0; i < res.TotalChunks; i++ {
		chunkQueue <- i
	}
	close(chunkQueue)

	var wg sync.WaitGroup
	errCh := make(chan error, res.TotalChunks)

	// Spawn a worker for each peer
	for _, pID := range peers {
		wg.Add(1)
		go func(workerPeer peer.ID) {
			defer wg.Done()
			
			// Open a single stream to this peer to reuse for multiple chunks
			stream, err := sm.node.Host.NewStream(ctx, workerPeer, ShareProtocol)
			if err != nil {
				fmt.Printf("Worker failed to connect to peer %s: %v\n", workerPeer, err)
				return // Peer offline or unreachable
			}
			defer stream.Close()
			
			wEncoder := json.NewEncoder(stream)
			wDecoder := json.NewDecoder(stream)
			
			for chunkIdx := range chunkQueue {
				// Request the chunk
				chunkReq := ShareRequest{Action: "get_chunk", ShareID: shareID, Index: chunkIdx}
				if err := wEncoder.Encode(&chunkReq); err != nil {
					errCh <- fmt.Errorf("worker %s failed to send chunk req: %w", workerPeer, err)
					return
				}
				
				var chunkRes ShareResponse
				if err := wDecoder.Decode(&chunkRes); err != nil {
					errCh <- fmt.Errorf("worker %s failed to read chunk: %w", workerPeer, err)
					return
				}
				if chunkRes.Status != "ok" {
					errCh <- fmt.Errorf("worker %s host error: %s", workerPeer, chunkRes.Error)
					return
				}
				
				// 4. Verify Per-Chunk Hash
				h := sha256.New()
				h.Write(chunkRes.Data)
				calculatedHash := hex.EncodeToString(h.Sum(nil))
				
				if calculatedHash != res.ChunkHashes[chunkIdx] {
					errCh <- fmt.Errorf("chunk %d corrupted from peer %s", chunkIdx, workerPeer)
					return
				}

				// Write to file concurrently
				fileMu.Lock()
				offset := int64(chunkIdx) * storage.DefaultChunkSize
				out.Seek(offset, io.SeekStart)
				n, wErr := out.Write(chunkRes.Data)
				downloaded += int64(n)
				if onProgress != nil {
					onProgress(downloaded)
				}
				fileMu.Unlock()
				
				if wErr != nil {
					errCh <- fmt.Errorf("failed to write chunk %d: %w", chunkIdx, wErr)
					return
				}
			}
		}(pID)
	}

	wg.Wait()
	close(errCh)
	
	if len(errCh) > 0 {
		var firstErr error
		for err := range errCh {
			firstErr = err
			break
		}
		os.Remove(downloadPath)
		return fmt.Errorf("download failed: %v", firstErr)
	}

	// 5. Final Verification (Root Hash) as double check
	fmt.Printf("Verifying final file integrity...\n")
	meta, err := storage.ChunkFile(downloadPath)
	if err != nil {
		os.Remove(downloadPath)
		return fmt.Errorf("failed to hash downloaded file: %w", err)
	}
	if meta.RootHash != res.RootHash {
		os.Remove(downloadPath)
		return fmt.Errorf("file integrity verification failed! Expected %s but got %s", res.RootHash, meta.RootHash)
	}

	// 6. Become a Seeder!
	seederShare := storage.Share{
		ID:        shareID,
		Path:      downloadPath,
		RootHash:  res.RootHash,
		Token:     "",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	sm.db.CreateShare(seederShare)
	
	// Announce to DHT
	if sm.node.DHT != nil {
		sm.node.DHT.Provide(context.Background(), c, true)
		fmt.Printf("Announced to DHT as seeder for %s!\n", shareID)
	}

	return nil
}
