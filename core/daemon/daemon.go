package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/peerdrive/core/api"
	"github.com/peerdrive/core/p2p"
	"github.com/peerdrive/core/storage"
)

type Daemon struct {
	configDir string
	node      *p2p.Node
	db        *storage.DB
	sm        *p2p.ShareManager
	apiServer *api.Server
	cancel    context.CancelFunc
}

func NewDaemon(configDir string, apiPort int, publicURL string) (*Daemon, error) {
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		configDir = filepath.Join(homeDir, ".peerdrive")
	}

	// Load or generate identity
	ident, err := p2p.LoadOrGenerateIdentity(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	// Setup Local DB
	dbPath := filepath.Join(configDir, "peerdrive.db")
	db, err := storage.InitDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Setup Libp2p Node
	ctx, cancel := context.WithCancel(context.Background())
	node, err := p2p.NewNode(ctx, ident, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize libp2p node: %w", err)
	}

	// Setup Share Manager
	sm := p2p.NewShareManager(node, db)

	// Start mDNS Discovery
	if err := p2p.SetupDiscovery(node.Host); err != nil {
		fmt.Printf("Warning: failed to setup mDNS: %v\n", err)
	}

	// Start Web API Server
	apiServer := api.NewServer(db, sm, node)
	apiServer.PublicURL = publicURL
	go func() {
		if err := apiServer.Start(apiPort); err != nil {
			fmt.Printf("API Server failed: %v\n", err)
		}
	}()

	return &Daemon{
		configDir: configDir,
		node:      node,
		db:        db,
		sm:        sm,
		apiServer: apiServer,
		cancel:    cancel,
	}, nil
}

func (d *Daemon) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.node != nil {
		d.node.Host.Close()
	}
	// SQLite closes automatically on process exit, or we can leave it
}
