package lsp

import (
	"strings"
	"sync"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// TextDocument represents a document open in the editor
type TextDocument struct {
	URI     string
	Text    string
	Version int
	tree    *tree_sitter.Tree
}

// DocumentManager manages text documents
type DocumentManager struct {
	documents map[string]*TextDocument
	mu        sync.RWMutex
	xmlParser *tree_sitter.Parser
	phpParser *tree_sitter.Parser
}

// NewDocumentManager creates a new document manager
func NewDocumentManager() *DocumentManager {
	xmlParser := tree_sitter.NewParser()
	xmlParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML()))

	phpParser := tree_sitter.NewParser()
	phpParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))

	return &DocumentManager{
		documents: make(map[string]*TextDocument),
		xmlParser: xmlParser,
		phpParser: phpParser,
	}
}

// OpenDocument adds or updates a document
func (m *DocumentManager) OpenDocument(uri string, text string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc := &TextDocument{
		URI:     uri,
		Text:    text,
		Version: version,
	}

	// Parse XML document if it's an XML file
	if isXMLFile(uri) {
		doc.tree = m.xmlParser.Parse([]byte(text), nil)
	}

	m.documents[uri] = doc
}

// UpdateDocument updates an existing document
func (m *DocumentManager) UpdateDocument(uri string, text string, version int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc, ok := m.documents[uri]; ok {
		doc.Text = text
		doc.Version = version

		// Update the tree if it's an XML file
		if isXMLFile(uri) {
			// Close the old tree if it exists
			if doc.tree != nil {
				doc.tree.Close()
			}
			doc.tree = m.xmlParser.Parse([]byte(text), nil)
		}
	} else {
		// If the document doesn't exist, create it
		doc := &TextDocument{
			URI:     uri,
			Text:    text,
			Version: version,
		}

		// Parse XML document if it's an XML file
		if isXMLFile(uri) {
			doc.tree = m.xmlParser.Parse([]byte(text), nil)
		}

		m.documents[uri] = doc
	}
}

// CloseDocument removes a document
func (m *DocumentManager) CloseDocument(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close the tree if it exists
	if doc, ok := m.documents[uri]; ok && doc.tree != nil {
		doc.tree.Close()
	}

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

func (m *DocumentManager) GetNodeAtPosition(uri string, line int, character int) (*tree_sitter.Node, *TextDocument, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if the document exists
	doc, ok := m.documents[uri]
	if !ok || doc.tree == nil {
		return nil, nil, false
	}

	// Find the closest element to our cursor position
	treeSitterPos := tree_sitter.Point{
		Row:    uint(line),
		Column: uint(character),
	}

	// Find the node at the cursor position
	node := doc.tree.RootNode().NamedDescendantForPointRange(treeSitterPos, treeSitterPos)

	return node, doc, true
}

// isXMLFile checks if a URI points to an XML file
func isXMLFile(uri string) bool {
	return strings.HasSuffix(strings.ToLower(uri), ".xml")
}

// IsServiceIDContext checks if the position is in a service ID attribute context
func (m *DocumentManager) IsServiceIDContext(uri string, line int, character int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if the document exists
	doc, ok := m.documents[uri]
	if !ok || doc.tree == nil {
		return false
	}

	// Find the closest element to our cursor position
	treeSitterPos := tree_sitter.Point{
		Row:    uint(line),
		Column: uint(character),
	}

	// Find the node at the cursor position
	node := doc.tree.RootNode().NamedDescendantForPointRange(treeSitterPos, treeSitterPos)

	if node == nil {
		return false
	}

	// Check if we're in an attribute value
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(doc.Text))
		if attrName != "id" {
			return false
		}

		// Get the parent element
		parentElement := attrNode.Parent()
		if parentElement == nil {
			return false
		}

		// Check if the parent element has a type="service" attribute
		attrValues := treesitterhelper.GetXmlAttributeValues(parentElement, doc.Text)
		if attrValues == nil || attrValues["type"] != "service" {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := treesitterhelper.GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text([]byte(doc.Text))
		return elementName == "argument"
	}

	return false
}

// Close closes the document manager and frees resources
func (m *DocumentManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close all trees
	for _, doc := range m.documents {
		if doc.tree != nil {
			doc.tree.Close()
			doc.tree = nil
		}
	}

	// Close the parser
	if m.xmlParser != nil {
		m.xmlParser.Close()
		m.xmlParser = nil
	}
}
