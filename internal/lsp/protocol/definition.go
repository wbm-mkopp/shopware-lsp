package protocol

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// DefinitionParams represents the parameters for a definition request
type DefinitionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
	// Custom fields for internal use (not part of LSP spec)
	// These fields are used to pass document content to definition providers
	DocumentContent []byte            `json:"-"`
	Node            *tree_sitter.Node `json:"-"`
}

// Location represents a location in a document
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// Range represents a range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a position in a document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
