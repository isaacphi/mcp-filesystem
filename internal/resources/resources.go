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
)

// URI prefix for file resources
const FileURIPrefix = "file://"

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
		return FileURIPrefix + path
	}
	return FileURIPrefix + absPath
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

// GetFileMIMEType returns the MIME type for a file
func GetFileMIMEType(path string) string {
	// Get MIME type from file extension
	ext := filepath.Ext(path)
	mimeType := mime.TypeByExtension(ext)

	// If MIME type is not found, use a default
	if mimeType == "" {
		// Try to determine if it's a text file
		if IsLikelyTextFile(path) {
			return "text/plain"
		}
		return "application/octet-stream"
	}

	return mimeType
}

// IsLikelyTextFile checks if a file is likely to be a text file
func IsLikelyTextFile(path string) bool {
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

// EnsureUTF8 converts text to UTF-8 encoding
func EnsureUTF8(data []byte) ([]byte, error) {
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

// FileExists checks if a file exists and is accessible
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ReadFileContents reads and returns the contents of a file
// handling encoding conversion to UTF-8
func ReadFileContents(path string) ([]byte, error) {
	// Read file content
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	// Convert to UTF-8 if necessary
	return EnsureUTF8(data)
}

// LogDebug logs a debug message if debug is enabled
func LogDebug(debug bool, format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}
