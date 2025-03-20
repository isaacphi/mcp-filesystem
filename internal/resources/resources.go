package resources

import (
	"fmt"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	mcp_golang "github.com/metoro-io/mcp-golang"
)

// URI prefix for file resources
const fileURIPrefix = "file://"

// ResourceManager manages file resources for the MCP server
type ResourceManager struct {
	workspacePath string
	debug         bool
}

// NewResourceManager creates a new resource manager
func NewResourceManager(workspacePath string, debug bool) *ResourceManager {
	return &ResourceManager{
		workspacePath: workspacePath,
		debug:         debug,
	}
}

// GetFileURI returns the URI for a file path
func (rm *ResourceManager) GetFileURI(path string) string {
	// Use absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fileURIPrefix + path
	}
	return fileURIPrefix + absPath
}

// GetResourceIDFromPath returns a resource ID from a file path
func (rm *ResourceManager) GetResourceIDFromPath(path string) string {
	// Use relative path from workspace as ID for better readability
	relPath, err := filepath.Rel(rm.workspacePath, path)
	if err != nil {
		// If relative path fails, use the basename
		return filepath.Base(path)
	}
	return relPath
}

// GetFileResourceHandler returns a resource handler function for a file
func (rm *ResourceManager) GetFileResourceHandler(path string) func() (*mcp_golang.ResourceResponse, error) {
	return func() (*mcp_golang.ResourceResponse, error) {
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
		data, err = ensureUTF8(data)
		if err != nil {
			return nil, fmt.Errorf("encoding error: %v", err)
		}

		// Get MIME type for the file
		mimeType := getFileMIMEType(path)
		uri := rm.GetFileURI(path)

		return mcp_golang.NewResourceResponse(
			mcp_golang.NewTextEmbeddedResource(uri, string(data), mimeType),
		), nil
	}
}

// RegisterFileResource registers a file as a resource with the MCP server
func (rm *ResourceManager) RegisterFileResource(server *mcp_golang.Server, path string) error {
	resourceID := rm.GetResourceIDFromPath(path)
	description := fmt.Sprintf("File: %s", resourceID)
	mimeType := getFileMIMEType(path)
	uri := rm.GetFileURI(path)

	if rm.debug {
		log.Printf("Registering resource: %s (URI: %s, MIME: %s)\n", resourceID, uri, mimeType)
	}

	return server.RegisterResource(
		uri,
		resourceID,
		description,
		mimeType,
		rm.GetFileResourceHandler(path),
	)
}

// DeregisterFileResource removes a file resource from the MCP server
func (rm *ResourceManager) DeregisterFileResource(server *mcp_golang.Server, path string) error {
	uri := rm.GetFileURI(path)

	if rm.debug {
		log.Printf("Deregistering resource: %s\n", uri)
	}

	return server.DeregisterResource(uri)
}

// getFileMIMEType returns the MIME type for a file
func getFileMIMEType(path string) string {
	// Get MIME type from file extension
	ext := filepath.Ext(path)
	mimeType := mime.TypeByExtension(ext)

	// If MIME type is not found, use a default
	if mimeType == "" {
		// Try to determine if it's a text file
		if isLikelyTextFile(path) {
			return "text/plain"
		}
		return "application/octet-stream"
	}

	return mimeType
}

// isLikelyTextFile checks if a file is likely to be a text file
func isLikelyTextFile(path string) bool {
	// Common text file extensions
	textExts := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".xml": true,
		".html": true, ".css": true, ".js": true, ".ts": true,
		".go": true, ".py": true, ".java": true, ".c": true,
		".cpp": true, ".h": true, ".sh": true, ".yaml": true,
		".yml": true, ".toml": true, ".cfg": true, ".conf": true,
		".ini": true, ".sql": true, ".log": true, ".csv": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	return textExts[ext]
}

// ensureUTF8 converts text to UTF-8 encoding
func ensureUTF8(data []byte) ([]byte, error) {
	// Check for UTF-16 BOM
	if len(data) >= 2 {
		if data[0] == 0xFE && data[1] == 0xFF { // UTF-16BE BOM
			decoder := unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder()
			result, _, err := transform.Bytes(decoder, data)
			return result, err
		} else if data[0] == 0xFF && data[1] == 0xFE { // UTF-16LE BOM
			decoder := unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()
			result, _, err := transform.Bytes(decoder, data)
			return result, err
		}
	}

	// Already UTF-8 or other encoding
	return data, nil
}
