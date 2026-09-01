package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// discoveryNotifee gets notified when we find a new peer via mDNS local discovery
type discoveryNotifee struct {
	h host.Host
}

// HandlePeerFound connects to peers discovered via mDNS
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	fmt.Printf("mDNS discovered new peer: %s\n", pi.ID.String())
	err := n.h.Connect(context.Background(), pi)
	if err != nil {
		fmt.Printf("Error connecting to peer %s: %s\n", pi.ID.String(), err)
	} else {
		fmt.Printf("Connected to peer %s\n", pi.ID.String())
	}
}

// SetupDiscovery sets up local mDNS discovery
func SetupDiscovery(h host.Host) error {
	s := mdns.NewMdnsService(h, "peerdrive-local", &discoveryNotifee{h: h})
	return s.Start()
}
