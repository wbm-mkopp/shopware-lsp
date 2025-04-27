package indexer

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

type Indexer interface {
	ID() string
	Index(path string, node *tree_sitter.Node, fileContent []byte) error
	RemovedFiles(paths []string) error
	Close() error
	Clear() error
}
