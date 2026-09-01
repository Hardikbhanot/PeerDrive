package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/peerdrive/core/daemon"
)

func main() {
	var configDir string
	var apiPort int
	var publicURL string
	
	flag.StringVar(&configDir, "config", "", "Path to the configuration and database directory")
	flag.IntVar(&apiPort, "port", 8080, "Port for the Web UI API")
	flag.StringVar(&publicURL, "public-url", "", "Public URL for QR codes (e.g. https://xyz.lhr.life)")
	flag.Parse()

	fmt.Println("Starting PeerDrive Core...")

	d, err := daemon.NewDaemon(configDir, apiPort, publicURL)
	if err != nil {
		fmt.Printf("Failed to start daemon: %v\n", err)
		os.Exit(1)
	}
	defer d.Stop()

	// Setup signal handling for clean shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Node is running! Press Ctrl+C to stop.")
	<-sigCh
	fmt.Println("Shutting down...")
}
