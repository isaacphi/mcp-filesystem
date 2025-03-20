package gitignore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sabhiram/go-gitignore"
)

// Matcher provides functionality to check if files should be ignored
type Matcher struct {
	ignore         *ignore.GitIgnore
	workspacePath  string
	hasGitIgnore   bool
	defaultIgnores []string
}

// NewMatcher creates a new gitignore matcher for the given workspace
func NewMatcher(workspacePath string) (*Matcher, error) {
	// Default ignores - common patterns to ignore
	defaultIgnores := []string{
		".git/",
		".DS_Store",
		"node_modules/",
	}

	matcher := &Matcher{
		workspacePath:  workspacePath,
		defaultIgnores: defaultIgnores,
	}

	// Check if .gitignore exists
	gitignorePath := filepath.Join(workspacePath, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		// Read .gitignore file
		data, err := os.ReadFile(gitignorePath)
		if err != nil {
			return nil, err
		}

		// Parse .gitignore content
		ignoreObj := ignore.CompileIgnoreLines(strings.Split(string(data), "\n")...)

		matcher.ignore = ignoreObj
		matcher.hasGitIgnore = true
	}

	return matcher, nil
}

// ShouldIgnore checks if a file should be ignored based on .gitignore rules
func (m *Matcher) ShouldIgnore(path string) bool {
	// Skip dot files
	if filepath.Base(path)[0] == '.' {
		return true
	}

	// Check against default ignores
	relPath, err := filepath.Rel(m.workspacePath, path)
	if err != nil {
		// If we can't get relative path, don't ignore
		return false
	}

	// Convert to forward slashes for consistency (go-gitignore expects this)
	relPath = filepath.ToSlash(relPath)

	// Check default ignores first
	for _, pattern := range m.defaultIgnores {
		if strings.HasPrefix(relPath, pattern) || relPath == pattern {
			return true
		}
	}

	// Check .gitignore rules if available
	if m.hasGitIgnore {
		return m.ignore.MatchesPath(relPath)
	}

	return false
}

// ShouldIgnoreDir checks if a directory should be ignored
func (m *Matcher) ShouldIgnoreDir(path string) bool {
	// Always allow the workspace root
	if path == m.workspacePath {
		return false
	}
	
	return m.ShouldIgnore(path)
}
