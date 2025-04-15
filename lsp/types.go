package lsp

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// CompletionProvider is an interface for providing completion items
type CompletionProvider interface {
	// GetCompletions returns completion items for the given parameters
	GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem
	// GetTriggerCharacters returns the characters that trigger this completion provider
	GetTriggerCharacters() []string
	// Index builds or updates the provider's index
	Index() error
}

// IndexerProvider is an interface for indexers that can be registered with the server
type IndexerProvider interface {
	// ID returns a unique identifier for this indexer
	ID() string
	// Name returns a human-readable name for this indexer
	Name() string
	// Index builds or updates the index
	Index() error
	// Close cleans up resources used by the indexer
	Close() error
}

// IndexerRegistry provides access to registered indexers
type IndexerRegistry interface {
	// RegisterIndexer adds an indexer to the registry
	RegisterIndexer(indexer IndexerProvider)
	// GetIndexer retrieves an indexer by ID
	GetIndexer(id string) (IndexerProvider, bool)
	// GetAllIndexers returns all registered indexers
	GetAllIndexers() []IndexerProvider
	// IndexAll builds or updates all registered indexes
	IndexAll() error
	// CloseAll closes all registered indexers
	CloseAll() error
}
