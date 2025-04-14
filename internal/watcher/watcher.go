package watcher

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sabhiram/go-gitignore"
)

// EventType represents the type of file event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
)

// FileEvent represents a file event from the watcher
type FileEvent struct {
	Path      string
	EventType EventType
}

// FileWatcher watches for file changes in a workspace
type FileWatcher struct {
	workspacePath string
	fsWatcher     *fsnotify.Watcher
	ignoreList    *ignore.GitIgnore
	watchDirs     map[string]bool
	debug         bool
	stopped       bool
	mu            sync.RWMutex
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(workspacePath string, debug bool) (*FileWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %v", err)
	}

	// Load gitignore if it exists
	var ignoreList *ignore.GitIgnore
	gitignorePath := filepath.Join(workspacePath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		if debug {
			log.Printf("Loading gitignore file: %s", gitignorePath)
		}
		ignoreList, err = ignore.CompileIgnoreFile(gitignorePath)
		if err != nil {
			log.Printf("Warning: Failed to compile gitignore file: %v", err)
			// Continue without gitignore
			ignoreList = ignore.CompileIgnoreLines()
		}
	} else {
		// No gitignore file, create an empty one
		ignoreList = ignore.CompileIgnoreLines()
	}

	return &FileWatcher{
		workspacePath: workspacePath,
		fsWatcher:     fsWatcher,
		ignoreList:    ignoreList,
		watchDirs:     make(map[string]bool),
		debug:         debug,
	}, nil
}

// Start starts the file watcher
func (w *FileWatcher) Start(ctx context.Context) (<-chan FileEvent, error) {
	if err := w.addWorkspaceToWatcher(); err != nil {
		return nil, fmt.Errorf("failed to add workspace to watcher: %v", err)
	}

	eventChan := make(chan FileEvent)

	go func() {
		defer close(eventChan)
		defer w.fsWatcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}
				w.handleFsEvent(event, eventChan)
			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	return eventChan, nil
}

// Stop stops the file watcher
func (w *FileWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.stopped {
		w.stopped = true
		w.fsWatcher.Close()
	}
}

// handleFsEvent processes a fsnotify event
func (w *FileWatcher) handleFsEvent(event fsnotify.Event, eventChan chan<- FileEvent) {
	// Skip temporary files and ignored files
	if w.shouldIgnoreFile(event.Name) {
		if w.debug {
			log.Printf("Ignoring event for file: %s", event.Name)
		}
		return
	}

	// Handle directory creation
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if w.debug {
				log.Printf("Directory created: %s", event.Name)
			}
			// Add the new directory to the watcher
			if err := w.addDirectoryToWatcher(event.Name); err != nil {
				log.Printf("Failed to add directory to watcher: %v", err)
			}
			return
		}
	}

	// Handle file events
	var fileEventType EventType
	switch {
	case event.Has(fsnotify.Create):
		fileEventType = EventCreate
	case event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod):
		fileEventType = EventModify
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		fileEventType = EventDelete
	default:
		// Unknown event type, ignore
		return
	}

	// Send event to the channel
	fileEvent := FileEvent{
		Path:      event.Name,
		EventType: fileEventType,
	}

	if w.debug {
		log.Printf("File event: %v %s", fileEventType, event.Name)
	}

	eventChan <- fileEvent
}

// GetInitialFiles returns a list of all files in the workspace
func (w *FileWatcher) GetInitialFiles() ([]string, error) {
	var files []string

	err := filepath.Walk(w.workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored files and directories
		if w.shouldIgnoreFile(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories but add them to the watcher
		if info.IsDir() {
			if err := w.addDirectoryToWatcher(path); err != nil {
				log.Printf("Failed to add directory to watcher: %v", err)
			}
			return nil
		}

		// Add file to the list
		files = append(files, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk workspace: %v", err)
	}

	return files, nil
}

// addWorkspaceToWatcher adds the workspace directory to the watcher
func (w *FileWatcher) addWorkspaceToWatcher() error {
	return w.addDirectoryToWatcher(w.workspacePath)
}

// addDirectoryToWatcher adds a directory to the watcher
func (w *FileWatcher) addDirectoryToWatcher(dirPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Skip if already watching or if the directory is ignored
	if w.watchDirs[dirPath] || w.shouldIgnoreFile(dirPath) {
		return nil
	}

	// Add to the watcher
	if err := w.fsWatcher.Add(dirPath); err != nil {
		return fmt.Errorf("failed to add directory to watcher: %v", err)
	}

	w.watchDirs[dirPath] = true

	if w.debug {
		log.Printf("Added directory to watcher: %s", dirPath)
	}

	return nil
}

// shouldIgnoreFile checks if a file should be ignored
func (w *FileWatcher) shouldIgnoreFile(path string) bool {
	// Extract the relative path from the workspace
	relPath, err := filepath.Rel(w.workspacePath, path)
	if err != nil {
		// If we can't get the relative path, ignore it
		return true
	}

	// Convert Windows path separators to Unix for gitignore compatibility
	relPath = strings.Replace(relPath, "\\", "/", -1)

	// Check against gitignore
	if w.ignoreList.MatchesPath(relPath) {
		return true
	}

	// Check for hidden files (starting with .)
	baseName := filepath.Base(path)
	if strings.HasPrefix(baseName, ".") {
		// But allow .gitignore itself
		return baseName != ".gitignore"
	}

	return false
}
