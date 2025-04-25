package protocol

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// ReferenceParams represents the parameters for a references request
type ReferenceParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
	Context struct {
		IncludeDeclaration bool `json:"includeDeclaration"`
	} `json:"context"`
	// Custom fields for internal use (not part of LSP spec)
	// These fields are used to pass document content to reference providers
	DocumentContent []byte            `json:"-"`
	Node            *tree_sitter.Node `json:"-"`
}
