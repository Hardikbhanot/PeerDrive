package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/peerdrive/core/p2p"
	"github.com/peerdrive/core/storage"
)

type Server struct {
	db           *storage.DB
	shareManager *p2p.ShareManager
	node         *p2p.Node
	
	mu       sync.Mutex
	Progress map[string]int64 // shareID -> bytes downloaded

	PublicURL string
}

func NewServer(db *storage.DB, sm *p2p.ShareManager, node *p2p.Node) *Server {
	return &Server{
		db:           db,
		shareManager: sm,
		node:         node,
		Progress:     make(map[string]int64),
	}
}

func (s *Server) Start(port int) error {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/shares", s.handleListShares)
	mux.HandleFunc("POST /api/shares", s.handleCreateShare)
	mux.HandleFunc("POST /api/shares/{id}/authorize", s.handleAuthorize)
	mux.HandleFunc("POST /api/shares/{id}/revoke", s.handleRevoke)
	mux.HandleFunc("POST /api/download", s.handleDownload)
	mux.HandleFunc("POST /api/dialog/file", s.handlePickFile)
	mux.HandleFunc("GET /d/{token}", s.handleDirectDownload)
	
	// Serve static frontend files
	// For now, serve a simple string or a static dir if we create one later
	mux.Handle("/", http.FileServer(http.Dir("./ui")))

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Web UI listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"peer_id":    s.node.Host.ID().String(),
		"ip":         getLocalIP(),
		"port":       r.Host, // this gives us the host:port the client used
		"public_url": s.PublicURL,
		"status":     "running",
	}
	json.NewEncoder(w).Encode(status)
}

type ShareResponseUI struct {
	storage.Share
	FileSize        int64 `json:"file_size"`
	DownloadedBytes int64 `json:"downloaded_bytes"`
}

func (s *Server) handleListShares(w http.ResponseWriter, r *http.Request) {
	shares, err := s.db.ListShares()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var uiShares []ShareResponseUI
	for _, share := range shares {
		var size int64 = 0
		if stat, err := os.Stat(share.Path); err == nil {
			size = stat.Size()
		}
		uiShares = append(uiShares, ShareResponseUI{
			Share:           share,
			FileSize:        size,
			DownloadedBytes: s.Progress[share.ID],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if uiShares == nil {
		w.Write([]byte(`[]`))
		return
	}
	json.NewEncoder(w).Encode(uiShares)
}

type CreateShareRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var filePath string
	var err error

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read uploaded file: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileName := header.Filename
		if fileName == "" {
			fileName = "uploaded-file"
		}
		filePath = filepath.Join(os.TempDir(), fmt.Sprintf("peerdrive-%d-%s", time.Now().UnixNano(), filepath.Base(fileName)))

		tmpFile, err := os.Create(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create temp file: %v", err), http.StatusInternalServerError)
			return
		}
		defer tmpFile.Close()

		if _, err := io.Copy(tmpFile, file); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save uploaded file: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		var req CreateShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		filePath = req.Path
	}

	if filePath == "" {
		http.Error(w, "No file selected", http.StatusBadRequest)
		return
	}

	meta, err := storage.ChunkFile(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process file: %v", err), http.StatusInternalServerError)
		return
	}

	shareID := fmt.Sprintf("SHARE-%x", meta.RootHash[:8])
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate token: %v", err), http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	share := storage.Share{
		ID:        shareID,
		Path:      filePath,
		RootHash:  meta.RootHash,
		Token:     token,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if err := s.db.CreateShare(share); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save share: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(share)
}

type progressWriter struct {
	http.ResponseWriter
	shareID string
	server  *Server
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.ResponseWriter.Write(p)
	pw.server.mu.Lock()
	pw.server.Progress[pw.shareID] += int64(n)
	pw.server.mu.Unlock()
	return n, err
}

func (s *Server) handleDirectDownload(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	share, err := s.db.GetShareByToken(token)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if share == nil || !share.IsActive {
		http.Error(w, "Share not found or inactive", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(share.Path)))
	
	// Reset progress on new download
	s.mu.Lock()
	s.Progress[share.ID] = 0
	s.mu.Unlock()

	pw := &progressWriter{ResponseWriter: w, shareID: share.ID, server: s}
	http.ServeFile(pw, r, share.Path)
}

func (s *Server) handlePickFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	candidates := [][]string{
		{"osascript", "-e", `POSIX path of (choose file with prompt "Select a file to share")`},
		{"zenity", "--file-selection", "--title=Select a file to share"},
		{"kdialog", "--getopenfilename", "."},
	}

	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}

		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.Output()
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				if strings.Contains(string(exitError.Stderr), "-128") || strings.Contains(string(exitError.Stderr), "cancelled") {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			continue
		}

		path := strings.TrimSpace(string(out))
		if path == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"path": path})
		return
	}

	http.Error(w, "No supported file picker is available on this system. Use the browser file chooser in the dashboard.", http.StatusNotImplemented)
}

type AuthorizeRequest struct {
	PeerID string `json:"peer_id"`
}

func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("id")
	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.PeerID = strings.TrimSpace(req.PeerID)
	if err := s.db.AuthorizePeer(shareID, req.PeerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Printf("[Host] Authorized peer %s for share %s\n", req.PeerID, shareID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	shareID := r.PathValue("id")
	if err := s.db.RevokeShare(shareID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Printf("[Host] Revoked share %s\n", shareID)
	w.WriteHeader(http.StatusOK)
}

type DownloadRequest struct {
	ShareID    string `json:"share_id"`
	HostPeerID string `json:"host_peer_id"`
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.ShareID = strings.TrimSpace(req.ShareID)
	req.HostPeerID = strings.TrimSpace(req.HostPeerID)

	// Tell the share manager to request the share from the host
	err := s.shareManager.RequestShare(r.Context(), req.HostPeerID, req.ShareID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
