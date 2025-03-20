package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/isaacphi/mcp-filesystem/internal/server"
)

var (
	debug = os.Getenv("DEBUG") != ""
)

func main() {
	// Parse command line arguments
	workspaceDir := flag.String("workspace", "", "Path to workspace directory")
	debugFlag := flag.Bool("debug", debug, "Enable debug output")
	flag.Parse()

	// Set debug flag if specified on command line
	if *debugFlag {
		debug = true
	}

	// Validate workspace directory
	if *workspaceDir == "" {
		log.Fatal("workspace directory is required")
	}

	// Get absolute path to workspace directory
	absWorkspaceDir, err := filepath.Abs(*workspaceDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for workspace: %v", err)
	}

	// Check if workspace directory exists
	if _, err := os.Stat(absWorkspaceDir); os.IsNotExist(err) {
		log.Fatalf("Workspace directory does not exist: %s", absWorkspaceDir)
	}

	// Create done channel for shutdown signal
	done := make(chan struct{})

	// Set up signal handling for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create and start MCP server
	mcpServer, err := server.NewMCPServer(absWorkspaceDir, debug)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	if debug {
		log.Printf("Starting MCP server for workspace: %s", absWorkspaceDir)
	}

	if err := mcpServer.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}

	// Monitor parent process termination
	// Claude desktop does not properly kill child processes for MCP servers
	go monitorParentProcess(done)

	// Handle signals for clean shutdown
	go func() {
		select {
		case sig := <-sigChan:
			log.Printf("Received signal %v, shutting down...", sig)
			cleanup(mcpServer, done)
		case <-done:
			// Channel already closed
		}
	}()

	// Keep running until done
	<-done
	log.Printf("Server shutdown complete")
}

// monitorParentProcess watches for parent process death
func monitorParentProcess(done chan struct{}) {
	ppid := os.Getppid()

	if debug {
		log.Printf("Monitoring parent process: %d", ppid)
	}

	// Create a ticker to check parent process periodically
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentPpid := os.Getppid()
			if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
				log.Printf("Parent process %d terminated, initiating shutdown", ppid)
				close(done)
				return
			}
		case <-done:
			return
		}
	}
}

// cleanup performs cleanup before shutdown
func cleanup(s *server.MCPServer, done chan struct{}) {
	log.Printf("Cleanup initiated")

	// Stop MCP server
	s.Stop()

	// Close done channel if not already closed
	select {
	case <-done:
		// Already closed
	default:
		close(done)
	}
}
