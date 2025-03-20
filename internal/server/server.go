package server

import (
	"context"
	"fmt"
	"log"
	"sync"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"

	"github.com/isaacphi/mcp-filesystem/internal/resources"
	"github.com/isaacphi/mcp-filesystem/internal/watcher"
)

// MCPServer represents the MCP server for the filesystem
type MCPServer struct {
	workspacePath   string
	mcpServer       *mcp_golang.Server
	watcher         *watcher.FileWatcher
	resourceManager *resources.ResourceManager
	debug           bool
	ctx             context.Context
	cancelFunc      context.CancelFunc
	registeredFiles map[string]bool
	mu              sync.RWMutex
}

// NewMCPServer creates a new MCP server
func NewMCPServer(workspacePath string, debug bool) (*MCPServer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	fileWatcher, err := watcher.NewFileWatcher(workspacePath, debug)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create file watcher: %v", err)
	}

	resourceManager := resources.NewResourceManager(workspacePath, debug)

	return &MCPServer{
		workspacePath:   workspacePath,
		resourceManager: resourceManager,
		watcher:         fileWatcher,
		debug:           debug,
		ctx:             ctx,
		cancelFunc:      cancel,
		registeredFiles: make(map[string]bool),
	}, nil
}

// Start starts the MCP server
func (s *MCPServer) Start() error {
	// Create and initialize MCP server
	s.mcpServer = mcp_golang.NewServer(
		stdio.NewStdioServerTransport(),
		mcp_golang.WithName("MCP Filesystem Server"),
		mcp_golang.WithVersion("1.0.0"),
	)

	// Start serving MCP requests
	if err := s.mcpServer.Serve(); err != nil {
		return fmt.Errorf("failed to start MCP server: %v", err)
	}

	// Register all existing files
	if err := s.registerExistingFiles(); err != nil {
		return fmt.Errorf("failed to register existing files: %v", err)
	}

	// Start file watcher
	fileEvents, err := s.watcher.Start(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %v", err)
	}

	// Process file events
	go s.processFileEvents(fileEvents)

	return nil
}

// Stop stops the MCP server
func (s *MCPServer) Stop() {
	s.cancelFunc()
	s.watcher.Stop()
}

// registerExistingFiles registers all existing files in the workspace
func (s *MCPServer) registerExistingFiles() error {
	files, err := s.watcher.GetInitialFiles()
	if err != nil {
		return fmt.Errorf("failed to get initial files: %v", err)
	}

	if s.debug {
		log.Printf("Found %d files to register", len(files))
	}

	// Register each file
	for _, file := range files {
		if err := s.registerFile(file); err != nil {
			log.Printf("Warning: failed to register file %s: %v", file, err)
		}
	}

	return nil
}

// processFileEvents processes file events from the watcher
func (s *MCPServer) processFileEvents(events <-chan watcher.FileEvent) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			s.handleFileEvent(event)
		}
	}
}

// handleFileEvent handles a file event
func (s *MCPServer) handleFileEvent(event watcher.FileEvent) {
	var err error

	switch event.EventType {
	case watcher.EventCreate:
		err = s.registerFile(event.Path)
	case watcher.EventModify:
		err = s.updateFile(event.Path)
	case watcher.EventDelete:
		err = s.unregisterFile(event.Path)
	}

	if err != nil {
		log.Printf("Error handling event for %s: %v", event.Path, err)
	}
}

// registerFile registers a file as a resource
func (s *MCPServer) registerFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if already registered
	if s.registeredFiles[path] {
		return nil
	}

	// Register file resource
	if err := s.resourceManager.RegisterFileResource(s.mcpServer, path); err != nil {
		return err
	}

	s.registeredFiles[path] = true

	if s.debug {
		log.Printf("Registered file: %s", path)
	}

	return nil
}

// updateFile handles file modifications
func (s *MCPServer) updateFile(path string) error {
	s.mu.RLock()
	isRegistered := s.registeredFiles[path]
	s.mu.RUnlock()

	if isRegistered {
		// For MCP resources, we don't need to update them explicitly
		// The resource handler will read the latest content when requested
		if s.debug {
			log.Printf("File modified: %s", path)
		}

		// No need to send notifications - the content will be read on demand
		return nil
	}

	// If not registered, register it
	return s.registerFile(path)
}

// unregisterFile removes a file resource
func (s *MCPServer) unregisterFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if not registered
	if !s.registeredFiles[path] {
		return nil
	}

	// Deregister file resource
	if err := s.resourceManager.DeregisterFileResource(s.mcpServer, path); err != nil {
		return err
	}

	delete(s.registeredFiles, path)

	if s.debug {
		log.Printf("Unregistered file: %s", path)
	}

	return nil
}
