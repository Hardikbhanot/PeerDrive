package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
)

type Node struct {
	Host host.Host
	DHT  *dht.IpfsDHT
}

func NewNode(ctx context.Context, ident *Identity, listenAddrs []string) (*Node, error) {
	// If no listen addresses provided, use default
	if len(listenAddrs) == 0 {
		listenAddrs = []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		}
	}

	opts := []libp2p.Option{
		libp2p.Identity(ident.PrivKey),
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.DefaultTransports,
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
		// NAT Traversal
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize DHT (Dual DHT for both LAN and WAN)
	kaddht, err := dht.New(h, dht.Mode(dht.ModeServer))
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	if err = kaddht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	return &Node{
		Host: h,
		DHT:  kaddht,
	}, nil
}

func (n *Node) PrintListenAddresses() {
	fmt.Println("Peer ID:", n.Host.ID().String())
	fmt.Println("Listening on:")
	for _, addr := range n.Host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, n.Host.ID().String())
	}
}

func (n *Node) Close() error {
	if n.DHT != nil {
		n.DHT.Close()
	}
	return n.Host.Close()
}
