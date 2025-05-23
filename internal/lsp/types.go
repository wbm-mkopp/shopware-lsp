package lsp

import (
	"context"
	"encoding/json"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// CompletionProvider is an interface for providing completion items
type CompletionProvider interface {
	// GetCompletions returns completion items for the given parameters
	GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem
	// GetTriggerCharacters returns the characters that trigger this completion provider
	GetTriggerCharacters() []string
}

// HoverProvider is an interface for providing hover information
type HoverProvider interface {
	// GetHover returns hover information for the given parameters
	GetHover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error)
}

// CodeLensProvider is an interface for providing code lenses
type CodeLensProvider interface {
	// GetCodeLenses returns code lenses for the given document
	GetCodeLenses(ctx context.Context, params *protocol.CodeLensParams) []protocol.CodeLens
	// ResolveCodeLens resolves the command for a given code lens item
	ResolveCodeLens(ctx context.Context, codeLens *protocol.CodeLens) (*protocol.CodeLens, error)
}

// IndexerProvider is an interface for indexers that can be registered with the server
type IndexerProvider interface {
	// ID returns a unique identifier for this indexer
	ID() string
	// Index builds or updates the index
	// If forceReindex is true, it will clear the existing index before rebuilding
	Index(forceReindex bool) error
	// Close cleans up resources used by the indexer
	Close() error

	FileCreated(ctx context.Context, params *protocol.CreateFilesParams) error
	FileRenamed(ctx context.Context, params *protocol.RenameFilesParams) error
	FileDeleted(ctx context.Context, params *protocol.DeleteFilesParams) error
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

type CommandFunc func(ctx context.Context, args *json.RawMessage) (interface{}, error)

type CommandProvider interface {
	GetCommands(ctx context.Context) map[string]CommandFunc
}
