package lsp

import (
	"sync"
)

// TextDocument represents a document open in the editor
type TextDocument struct {
	URI     string
	Text    string
	Version int
}

// DocumentManager manages text documents
type DocumentManager struct {
	documents map[string]*TextDocument
	mu        sync.RWMutex
}

// NewDocumentManager creates a new document manager
func NewDocumentManager() *DocumentManager {
	return &DocumentManager{
		documents: make(map[string]*TextDocument),
	}
}

// OpenDocument adds or updates a document
func (m *DocumentManager) OpenDocument(uri string, text string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.documents[uri] = &TextDocument{
		URI:     uri,
		Text:    text,
		Version: version,
	}
}

// UpdateDocument updates an existing document
func (m *DocumentManager) UpdateDocument(uri string, text string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc, ok := m.documents[uri]; ok {
		doc.Text = text
		doc.Version = version
	} else {
		// If the document doesn't exist, create it
		m.documents[uri] = &TextDocument{
			URI:     uri,
			Text:    text,
			Version: version,
		}
	}
}

// CloseDocument removes a document
func (m *DocumentManager) CloseDocument(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.documents, uri)
}

// GetDocument returns a document by URI
func (m *DocumentManager) GetDocument(uri string) (*TextDocument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.documents[uri]
	return doc, ok
}

// GetDocumentText returns the text of a document by URI
func (m *DocumentManager) GetDocumentText(uri string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if doc, ok := m.documents[uri]; ok {
		return doc.Text, true
	}
	return "", false
}

// GetLineAtPosition returns the line of text at the given position
func (m *DocumentManager) GetLineAtPosition(uri string, line int) (string, bool) {
	text, ok := m.GetDocumentText(uri)
	if !ok {
		return "", false
	}

	// Split the text into lines
	lines := splitLines(text)

	// Check if the line is valid
	if line < 0 || line >= len(lines) {
		return "", false
	}

	return lines[line], true
}

// splitLines splits text into lines
func splitLines(text string) []string {
	var lines []string
	var line string

	for _, r := range text {
		if r == '\n' {
			lines = append(lines, line)
			line = ""
		} else {
			line += string(r)
		}
	}

	// Add the last line if it doesn't end with a newline
	if line != "" {
		lines = append(lines, line)
	}

	return lines
}
