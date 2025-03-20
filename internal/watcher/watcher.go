package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/isaacphi/mcp-filesystem/internal/gitignore"
)

// Event types for file system changes
const (
	EventCreate int = iota
	EventModify
	EventDelete
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	EventType int
}

// FileWatcher watches a workspace for file changes
type FileWatcher struct {
	workspacePath string
	matcher       *gitignore.Matcher
	watcher       *fsnotify.Watcher
	events        chan FileEvent
	done          chan struct{}
	watchedDirs   map[string]bool
	mu            sync.RWMutex
	debug         bool
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(workspacePath string, debug bool) (*FileWatcher, error) {
	matcher, err := gitignore.NewMatcher(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create gitignore matcher: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %v", err)
	}

	return &FileWatcher{
		workspacePath: workspacePath,
		matcher:       matcher,
		watcher:       watcher,
		events:        make(chan FileEvent),
		done:          make(chan struct{}),
		watchedDirs:   make(map[string]bool),
		debug:         debug,
	}, nil
}

// startWatching adds a directory to the watcher
func (fw *FileWatcher) startWatching(path string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// Skip if already watched
	if fw.watchedDirs[path] {
		return nil
	}

	// Add to watcher
	if err := fw.watcher.Add(path); err != nil {
		return err
	}

	fw.watchedDirs[path] = true
	if fw.debug {
		log.Printf("Started watching: %s", path)
	}

	return nil
}

// stopWatching removes a directory from the watcher
func (fw *FileWatcher) stopWatching(path string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.watchedDirs[path] {
		_ = fw.watcher.Remove(path)
		delete(fw.watchedDirs, path)
		if fw.debug {
			log.Printf("Stopped watching: %s", path)
		}
	}
}

// Start begins watching the workspace for changes
func (fw *FileWatcher) Start(ctx context.Context) (<-chan FileEvent, error) {
	// Perform an initial scan of the workspace
	if err := fw.scanWorkspace(); err != nil {
		return nil, err
	}

	// Start the event loop
	go fw.eventLoop(ctx)

	return fw.events, nil
}

// Stop stops watching for changes
func (fw *FileWatcher) Stop() {
	close(fw.done)
	if err := fw.watcher.Close(); err != nil {
		log.Printf("Error closing watcher: %v", err)
	}
}

// scanWorkspace recursively adds all directories in the workspace to the watcher
func (fw *FileWatcher) scanWorkspace() error {
	return filepath.Walk(fw.workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			if fw.matcher.ShouldIgnoreDir(path) {
				if fw.debug {
					log.Printf("Skipping ignored directory: %s", path)
				}
				return filepath.SkipDir
			}

			// Add directory to watcher
			if err := fw.startWatching(path); err != nil {
				return err
			}
		}

		return nil
	})
}

// eventLoop processes fsnotify events
func (fw *FileWatcher) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.done:
			return
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleFsEvent(event)
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Error: %v", err)
		}
	}
}

// handleFsEvent processes a single fsnotify event
func (fw *FileWatcher) handleFsEvent(event fsnotify.Event) {
	// Check if this path should be ignored
	if fw.matcher.ShouldIgnore(event.Name) {
		return
	}

	if fw.debug {
		log.Printf("Event: %s %s", event.Name, event.Op.String())
	}

	// Get file info
	fileInfo, err := os.Stat(event.Name)
	isDir := err == nil && fileInfo.IsDir()

	// Handle directory events
	if isDir {
		if event.Op&fsnotify.Create != 0 {
			// New directory - add to watcher
			if err := fw.startWatching(event.Name); err != nil {
				log.Printf("Error watching new directory: %v", err)
				return
			}

			// Scan the new directory for sub-directories
			_ = filepath.Walk(event.Name, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() && path != event.Name {
					if fw.matcher.ShouldIgnoreDir(path) {
						return filepath.SkipDir
					}
					_ = fw.startWatching(path)
				}
				return nil
			})
		} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			// Directory removed - remove from watcher
			fw.stopWatching(event.Name)
		}
		return
	}

	// Handle file events
	var eventType int
	if event.Op&fsnotify.Create != 0 {
		eventType = EventCreate
	} else if event.Op&fsnotify.Write != 0 {
		eventType = EventModify
	} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		eventType = EventDelete
	} else {
		// Ignore other event types
		return
	}

	// Send event to channel
	select {
	case fw.events <- FileEvent{Path: event.Name, EventType: eventType}:
	case <-fw.done:
		return
	}
}

// GetInitialFiles returns a list of all existing files in the workspace
func (fw *FileWatcher) GetInitialFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(fw.workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories and ignored files
		if info.IsDir() {
			if fw.matcher.ShouldIgnoreDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		if !fw.matcher.ShouldIgnore(path) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
