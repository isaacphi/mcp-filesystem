package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/isaacphi/mcp-filesystem/internal/resources"
	"github.com/isaacphi/mcp-filesystem/internal/watcher"
)

// MCPServer represents the MCP server for the filesystem
type MCPServer struct {
	workspacePath   string
	mcpServer       *server.MCPServer
	watcher         *watcher.FileWatcher
	resourceManager *resources.ResourceManager
	debug           bool
	ctx             context.Context
	cancelFunc      context.CancelFunc
	registeredFiles map[string]bool
	subscribedFiles map[string]bool // Track which files are subscribed to
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

	// Create the MCP server with mark3labs/mcp-go
	mcpServer := server.NewMCPServer(
		"MCP Filesystem Server",
		"1.0.0",
		server.WithResourceCapabilities(true, true), // Enable subscription and list_changed
		server.WithLogging(),                        // Enable logging
	)

	if debug {
		log.Printf("Created MCP server for workspace: %s", workspacePath)
	}

	return &MCPServer{
		workspacePath:   workspacePath,
		resourceManager: resourceManager,
		mcpServer:       mcpServer,
		watcher:         fileWatcher,
		debug:           debug,
		ctx:             ctx,
		cancelFunc:      cancel,
		registeredFiles: make(map[string]bool),
		subscribedFiles: make(map[string]bool),
	}, nil
}

// Start starts the MCP server
func (s *MCPServer) Start() error {
	// Register all existing files
	if err := s.registerExistingFiles(); err != nil {
		return fmt.Errorf("failed to register existing files: %v", err)
	}

	// Register a handler for resource subscribe requests
	s.mcpServer.AddNotificationHandler("resources/subscribe", s.handleSubscribe)

	// Start file watcher
	fileEvents, err := s.watcher.Start(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to start file watcher: %v", err)
	}

	// Process file events in a separate goroutine
	go s.processFileEvents(fileEvents)

	// Start the MCP server using stdio transport
	if err := server.ServeStdio(s.mcpServer); err != nil {
		return fmt.Errorf("failed to start MCP server: %v", err)
	}

	return nil
}

// Stop stops the MCP server
func (s *MCPServer) Stop() {
	if s.debug {
		log.Printf("Stopping MCP server")
	}
	s.cancelFunc() // Cancel the context
	s.watcher.Stop()
}

// handleSubscribe handles resource subscription requests from clients
func (s *MCPServer) handleSubscribe(ctx context.Context, notification mcp.JSONRPCNotification) {
	// Get the URI from the AdditionalFields
	uri, ok := notification.Params.AdditionalFields["uri"]
	if !ok {
		if s.debug {
			log.Printf("Warning: Received subscribe request without URI")
		}
		return
	}

	// Convert to string
	uriString, ok := uri.(string)
	if !ok {
		if s.debug {
			log.Printf("Warning: URI is not a string: %v", uri)
		}
		return
	}

	if uriString == "" {
		if s.debug {
			log.Printf("Warning: Received subscribe request with empty URI")
		}
		return
	}

	// Get the path from the URI
	path := s.getPathFromURI(uriString)
	if path == "" {
		if s.debug {
			log.Printf("Warning: Failed to get path from URI: %s", uriString)
		}
		return
	}

	// Register the subscription
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subscribedFiles[path] = true

	if s.debug {
		log.Printf("Subscribed to resource: %s", path)
	}
}

// getPathFromURI extracts the file path from a URI
func (s *MCPServer) getPathFromURI(uri string) string {
	// Handle file:// URIs
	if strings.HasPrefix(uri, resources.FileURIPrefix) {
		return strings.TrimPrefix(uri, resources.FileURIPrefix)
	}

	return ""
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

	// Create a resource for the file
	uri := s.resourceManager.GetFileURI(path)
	resourceID := s.resourceManager.GetResourceIDFromPath(path)
	description := fmt.Sprintf("File: %s", resourceID)
	mimeType := resources.GetFileMIMEType(path)

	if s.debug {
		log.Printf("Registering resource: %s (URI: %s, MIME: %s)", resourceID, uri, mimeType)
	}

	// Create resource with the mark3labs/mcp-go API
	resource := mcp.NewResource(
		uri,
		resourceID,
		mcp.WithResourceDescription(description),
		mcp.WithMIMEType(mimeType),
	)

	// Add the resource handler
	s.mcpServer.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		// Check if file still exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", path)
		}

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %v", err)
		}

		// Detect and handle text encodings (UTF-8, UTF-16, etc.)
		data, err = resources.EnsureUTF8(data)
		if err != nil {
			return nil, fmt.Errorf("encoding error: %v", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Text:     string(data),
			},
		}, nil
	})

	s.registeredFiles[path] = true

	if s.debug {
		log.Printf("Registered file: %s", path)
	}

	// Notify clients that the resource list has changed
	// This uses context.Background() because we don't have a client ctx here
	err := s.mcpServer.SendNotificationToClient(
		context.Background(),
		"resources/list_changed",
		map[string]interface{}{},
	)
	if err != nil && s.debug {
		log.Printf("Warning: Failed to send list_changed notification: %v", err)
	}

	return nil
}

// updateFile handles file modifications
func (s *MCPServer) updateFile(path string) error {
	s.mu.RLock()
	isRegistered := s.registeredFiles[path]
	isSubscribed := s.subscribedFiles[path]
	s.mu.RUnlock()

	if !isRegistered {
		// If not registered, register it
		return s.registerFile(path)
	}

	// File is registered, now check if it's subscribed
	if isSubscribed {
		if s.debug {
			log.Printf("File modified and subscribed: %s", path)
		}

		// Send a notification that the resource has been updated
		uri := s.resourceManager.GetFileURI(path)
		
		// Create the resource updated notification
		err := s.mcpServer.SendNotificationToClient(
			context.Background(),
			"resources/updated",
			map[string]interface{}{
				"uri": uri,
			},
		)
		
		if err != nil && s.debug {
			log.Printf("Warning: Failed to send resource updated notification: %v", err)
		}
	} else if s.debug {
		log.Printf("File modified but not subscribed: %s", path)
	}

	return nil
}

// unregisterFile handles file deletion
// Instead of deregistering, we keep track of it but leave the resource registered
// The resource handler will return an error if the file is requested
func (s *MCPServer) unregisterFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if not registered
	if !s.registeredFiles[path] {
		return nil
	}

	// We don't actually deregister the resource from the mcpServer
	// Instead, we just mark it as not registered in our map
	// The resource handler will return an error if the file is requested
	delete(s.registeredFiles, path)
	
	// Also remove any subscriptions
	delete(s.subscribedFiles, path)

	if s.debug {
		log.Printf("Unregistered file: %s", path)
	}

	// Notify clients that the resource list has changed
	// This uses context.Background() because we don't have a client ctx here
	err := s.mcpServer.SendNotificationToClient(
		context.Background(),
		"resources/list_changed",
		map[string]interface{}{},
	)
	if err != nil && s.debug {
		log.Printf("Warning: Failed to send list_changed notification: %v", err)
	}

	return nil
}
