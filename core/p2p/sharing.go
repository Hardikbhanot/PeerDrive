package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/peerdrive/core/storage"
)

const ShareProtocol = "/peerdrive/share/1.0.0"

type ShareManager struct {
	host host.Host
	db   *storage.DB
}

type ShareRequest struct {
	Action  string `json:"action"`   // "join", "get_chunk"
	ShareID string `json:"share_id"`
	Index   int    `json:"index,omitempty"` // for get_chunk
}

type ShareResponse struct {
	Status      string `json:"status"` // "ok", "error"
	Error       string `json:"error,omitempty"`
	RootHash    string `json:"root_hash,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	FileSize    int64  `json:"file_size,omitempty"`
	TotalChunks int    `json:"total_chunks,omitempty"`
	Data        []byte `json:"data,omitempty"` // for get_chunk
}

func NewShareManager(h host.Host, db *storage.DB) *ShareManager {
	sm := &ShareManager{
		host: h,
		db:   db,
	}
	h.SetStreamHandler(ShareProtocol, sm.handleStream)
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
			totalChunks := int(stat.Size() / storage.DefaultChunkSize)
			if stat.Size()%storage.DefaultChunkSize != 0 {
				totalChunks++
			}

			encoder.Encode(ShareResponse{
				Status:      "ok",
				RootHash:    share.RootHash,
				FileName:    filepath.Base(share.Path),
				FileSize:    stat.Size(),
				TotalChunks: totalChunks,
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

// RequestShare is called by the downloading peer
func (sm *ShareManager) RequestShare(ctx context.Context, peerIDString string, shareID string, onProgress func(int64)) error {
	peerID, err := peer.Decode(peerIDString)
	if err != nil {
		return fmt.Errorf("invalid peer id: %w", err)
	}

	s, err := sm.host.NewStream(ctx, peerID, ShareProtocol)
	if err != nil {
		return fmt.Errorf("failed to open stream to host: %w", err)
	}
	defer s.Close()

	encoder := json.NewEncoder(s)
	decoder := json.NewDecoder(s)

	req := ShareRequest{
		Action:  "join",
		ShareID: shareID,
	}
	if err := encoder.Encode(&req); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	var res ShareResponse
	if err := decoder.Decode(&res); err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if res.Status != "ok" {
		return fmt.Errorf("host rejected join: %s", res.Error)
	}

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

	for i := 0; i < res.TotalChunks; i++ {
		req := ShareRequest{
			Action:  "get_chunk",
			ShareID: shareID,
			Index:   i,
		}
		if err := encoder.Encode(&req); err != nil {
			return fmt.Errorf("failed to request chunk %d: %w", i, err)
		}

		var chunkRes ShareResponse
		if err := decoder.Decode(&chunkRes); err != nil {
			return fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		if chunkRes.Status != "ok" {
			return fmt.Errorf("host error on chunk %d: %s", i, chunkRes.Error)
		}

		n, err := out.Write(chunkRes.Data)
		if err != nil {
			return fmt.Errorf("failed to write chunk %d to disk: %w", i, err)
		}
		
		downloaded += int64(n)
		if onProgress != nil {
			onProgress(downloaded)
		}
	}

	seederShare := storage.Share{
		ID:        shareID,
		Path:      downloadPath,
		RootHash:  res.RootHash,
		Token:     "",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	sm.db.CreateShare(seederShare)

	return nil
}
