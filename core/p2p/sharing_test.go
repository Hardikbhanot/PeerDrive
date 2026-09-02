package p2p

import (
	"context"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peerdrive/core/storage"
)

func TestMultiPeerSwarm(t *testing.T) {
	testDir := t.TempDir()
	filePath := filepath.Join(testDir, "test_file.bin")
	fileSize := 1024 * 1024 * 2
	
	f, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, fileSize)
	rand.Read(buf)
	f.Write(buf)
	f.Close()

	ident1, _ := LoadOrGenerateIdentity(filepath.Join(testDir, "id1"))
	node1, _ := NewNode(context.Background(), ident1, []string{"/ip4/127.0.0.1/tcp/0"})
	defer node1.Close()

	ident2, _ := LoadOrGenerateIdentity(filepath.Join(testDir, "id2"))
	node2, _ := NewNode(context.Background(), ident2, []string{"/ip4/127.0.0.1/tcp/0"})
	defer node2.Close()

	db1, _ := storage.InitDB(filepath.Join(testDir, "db1.sqlite"))
	_ = NewShareManager(node1, db1)

	db2, _ := storage.InitDB(filepath.Join(testDir, "db2.sqlite"))
	sm2 := NewShareManager(node2, db2)

	node2.Host.Connect(context.Background(), node1.Host.Peerstore().PeerInfo(node1.Host.ID()))

	meta, _ := storage.ChunkFile(filePath)
	shareID := "SHARE-TEST"
	share := storage.Share{
		ID:        shareID,
		Path:      filePath,
		RootHash:  meta.RootHash,
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	db1.CreateShare(share)
	db1.AuthorizePeer(shareID, node2.Host.ID().String())

	c, _ := hashToCID(meta.RootHash)
	node1.DHT.Provide(context.Background(), c, true)
	time.Sleep(1 * time.Second)

	err = sm2.RequestShare(context.Background(), node1.Host.ID().String(), shareID, nil)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	downloadedShare, err := db2.GetShare(shareID)
	if err != nil || downloadedShare == nil {
		t.Fatalf("Node 2 did not save share to DB")
	}

	downloadedMeta, _ := storage.ChunkFile(downloadedShare.Path)
	if downloadedMeta.RootHash != meta.RootHash {
		t.Fatalf("Root hashes do not match! Expected %s got %s", meta.RootHash, downloadedMeta.RootHash)
	}
}
