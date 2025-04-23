package lsp

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/sourcegraph/jsonrpc2"
)

// Server represents the LSP server
type Server struct {
	rootPath            string
	conn                *jsonrpc2.Conn
	completionProviders []CompletionProvider
	definitionProviders []GotoDefinitionProvider
	codeLensProviders   []CodeLensProvider
	indexers            map[string]indexer.Indexer
	indexerMu           sync.RWMutex
	documentManager     *DocumentManager
	FileScanner         *indexer.FileScanner
}

// NewServer creates a new LSP server
func NewServer(filescanner *indexer.FileScanner) *Server {
	return &Server{
		completionProviders: make([]CompletionProvider, 0),
		definitionProviders: make([]GotoDefinitionProvider, 0),
		codeLensProviders:   make([]CodeLensProvider, 0),
		indexers:            make(map[string]indexer.Indexer),
		documentManager:     NewDocumentManager(),
		FileScanner:         filescanner,
	}
}

// RegisterCompletionProvider registers a completion provider with the server
func (s *Server) RegisterCompletionProvider(provider CompletionProvider) {
	s.completionProviders = append(s.completionProviders, provider)
}

// RegisterDefinitionProvider registers a definition provider with the server
func (s *Server) RegisterDefinitionProvider(provider GotoDefinitionProvider) {
	s.definitionProviders = append(s.definitionProviders, provider)
}

// RegisterCodeLensProvider registers a code lens provider with the server
func (s *Server) RegisterCodeLensProvider(provider CodeLensProvider) {
	s.codeLensProviders = append(s.codeLensProviders, provider)
}

// RegisterIndexer adds an indexer to the registry
func (s *Server) RegisterIndexer(indexer indexer.Indexer, err error) {
	s.indexerMu.Lock()
	defer s.indexerMu.Unlock()
	s.indexers[indexer.ID()] = indexer
}

// GetIndexer retrieves an indexer by ID
func (s *Server) GetIndexer(id string) (indexer.Indexer, bool) {
	s.indexerMu.RLock()
	defer s.indexerMu.RUnlock()
	indexer, ok := s.indexers[id]
	return indexer, ok
}

// indexAll builds or updates all registered indexes
// If forceReindex is true, it will clear the existing index before rebuilding
func (s *Server) indexAll(ctx context.Context, forceReindex bool) error {
	startTime := time.Now()

	// Send notification that indexing has started
	if s.conn != nil {
		if err := s.conn.Notify(ctx, "shopware/indexingStarted", map[string]interface{}{
			"message": "Indexing started",
		}); err != nil {
			return err
		}
	}

	if forceReindex {
		if err := s.FileScanner.ClearHashes(); err != nil {
			return err
		}
	}

	if err := s.FileScanner.IndexAll(); err != nil {
		return err
	}

	elapsedTime := time.Since(startTime)

	// Send notification that indexing has completed
	if s.conn != nil {
		if err := s.conn.Notify(ctx, "shopware/indexingCompleted", map[string]interface{}{
			"message":       "Indexing completed",
			"timeInSeconds": elapsedTime.Seconds(),
		}); err != nil {
			return err
		}
	}

	return nil
}

// CloseAll closes all registered indexers and resources
func (s *Server) CloseAll() error {
	// Close document manager first
	if s.documentManager != nil {
		s.documentManager.Close()
	}

	// Then close all indexers
	s.indexerMu.RLock()
	defer s.indexerMu.RUnlock()

	for _, indexer := range s.indexers {
		if err := indexer.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) Start(in io.Reader, out io.Writer) error {
	// Create a new JSON-RPC connection
	stream := jsonrpc2.NewBufferedStream(rwc{in, out}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(context.Background(), stream, jsonrpc2.HandlerWithError(s.handle))
	s.conn = conn

	// Wait for the connection to close
	<-conn.DisconnectNotify()
	return nil
}

// rwc combines a reader and writer into a single ReadWriteCloser
type rwc struct {
	io.Reader
	io.Writer
}

// Close implements io.Closer
func (rwc) Close() error {
	return nil
}

// handle processes incoming JSON-RPC requests and notifications
func (s *Server) handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) (interface{}, error) {
	// Handle exit notification after shutdown
	if req.Method == "exit" {
		log.Println("Received exit notification, exiting")
		if err := conn.Close(); err != nil {
			log.Printf("error closing connection: %v", err)
		}
		return nil, nil
	}

	switch req.Method {
	case "initialize":
		var params protocol.InitializeParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: err.Error()}
		}
		return s.initialize(ctx, &params), nil

	case "initialized":
		// Build the index when the client is initialized
		go func() {
			// Index all registered indexers
			if err := s.indexAll(ctx, false); err != nil {
				log.Printf("Error indexing: %v", err)
			}
		}()
		return nil, nil

	case "textDocument/didOpen":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Text    string `json:"text"`
				Version int    `json:"version"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		s.documentManager.OpenDocument(params.TextDocument.URI, params.TextDocument.Text, params.TextDocument.Version)
		return nil, nil

	case "textDocument/didChange":
		var params struct {
			TextDocument struct {
				URI     string `json:"uri"`
				Version int    `json:"version"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		if len(params.ContentChanges) > 0 {
			s.documentManager.UpdateDocument(params.TextDocument.URI, params.ContentChanges[0].Text, params.TextDocument.Version)
		}
		return nil, nil

	case "textDocument/didClose":
		var params struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		s.documentManager.CloseDocument(params.TextDocument.URI)
		return nil, nil

	case "textDocument/completion":
		var params protocol.CompletionParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return s.completion(ctx, &params), nil

	case "textDocument/definition":
		var params protocol.DefinitionParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return s.definition(ctx, &params), nil

	case "textDocument/codeLens":
		var params protocol.CodeLensParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}
		return s.codeLens(ctx, &params), nil

	case "codeLens/resolve":
		var codeLens protocol.CodeLens
		if err := json.Unmarshal(*req.Params, &codeLens); err != nil {
			return nil, err
		}
		return s.resolveCodeLens(ctx, &codeLens)

	case "shopware/forceReindex":
		// Force reindex all indexers
		go func() {
			if err := s.indexAll(ctx, true); err != nil {
				log.Printf("Error force reindexing: %v", err)
			}
		}()
		return map[string]interface{}{
			"message": "Force reindexing started",
		}, nil

	case "shutdown":
		// Clean up resources
		if err := s.CloseAll(); err != nil {
			log.Printf("Error closing indexers: %v", err)
		}

		log.Println("Received shutdown request, waiting for exit notification")
		return nil, nil

	case "workspace/didCreateFiles":
		var params protocol.CreateFilesParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}

		files := make([]string, len(params.Files))
		for i, file := range params.Files {
			files[i] = file.URI
		}
		if err := s.FileScanner.IndexFiles(files); err != nil {
			log.Printf("Error indexing new files: %v", err)
		}
		return nil, nil

	case "workspace/didRenameFiles":
		var params protocol.RenameFilesParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}

		oldFiles := make([]string, len(params.Files))
		newFiles := make([]string, len(params.Files))
		for i, file := range params.Files {
			oldFiles[i] = file.OldURI
			newFiles[i] = file.NewURI
		}

		if err := s.FileScanner.IndexFiles(newFiles); err != nil {
			log.Printf("Error indexing new files: %v", err)
		}
		if err := s.FileScanner.RemoveFiles(oldFiles); err != nil {
			log.Printf("Error removing old files: %v", err)
		}

		return nil, nil

	case "workspace/didDeleteFiles":
		var params protocol.DeleteFilesParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}

		files := make([]string, len(params.Files))
		for i, file := range params.Files {
			files[i] = file.URI
		}
		if err := s.FileScanner.RemoveFiles(files); err != nil {
			log.Printf("Error removing old files: %v", err)
		}
		return nil, nil

	case "workspace/didChangeWatchedFiles":
		var params protocol.DidChangeWatchedFilesParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, err
		}

		createFiles := &protocol.CreateFilesParams{}
		deleteFiles := &protocol.DeleteFilesParams{}

		// Handle file change events
		for _, change := range params.Changes {
			switch change.Type {
			case int(protocol.FileCreated):
				createFiles.Files = append(createFiles.Files, protocol.FileCreate{URI: change.URI})
			case int(protocol.FileChanged):
				createFiles.Files = append(createFiles.Files, protocol.FileCreate{URI: change.URI})
			case int(protocol.FileDeleted):
				deleteFiles.Files = append(deleteFiles.Files, protocol.FileDelete{URI: change.URI})
			}
		}

		if createFiles.Files != nil {
			files := make([]string, len(createFiles.Files))
			for i, file := range createFiles.Files {
				files[i] = file.URI
			}
			if err := s.FileScanner.IndexFiles(files); err != nil {
				log.Printf("Error indexing new files: %v", err)
			}
		}
		if deleteFiles.Files != nil {
			files := make([]string, len(deleteFiles.Files))
			for i, file := range deleteFiles.Files {
				files[i] = file.URI
			}
			if err := s.FileScanner.RemoveFiles(files); err != nil {
				log.Printf("Error removing old files: %v", err)
			}
		}

		return nil, nil

	default:
		// Check if this is a notification (no ID)
		if req.ID == (jsonrpc2.ID{}) {
			// This is a notification, no response needed
			return nil, nil
		}
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "Method not implemented: " + req.Method}
	}
}

// initialize handles the LSP initialize request
func (s *Server) initialize(ctx context.Context, params *protocol.InitializeParams) interface{} {
	// Extract root path from params
	s.extractRootPath(params)

	// Collect all trigger characters from providers
	triggerChars := s.collectTriggerCharacters()

	// Define server capabilities
	return map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    1, // Full sync
			},
			"completionProvider": map[string]interface{}{
				"triggerCharacters": triggerChars,
			},
			"definitionProvider": true,
			"codeLensProvider": map[string]interface{}{
				"resolveProvider": true,
			},
			"workspace": map[string]interface{}{
				"fileOperations": map[string]interface{}{
					"didCreate": map[string]interface{}{
						"filters": []map[string]interface{}{
							{"pattern": map[string]interface{}{"glob": "**/*.xml"}},
							{"pattern": map[string]interface{}{"glob": "**/*.php"}},
						},
					},
					"didRename": map[string]interface{}{
						"filters": []map[string]interface{}{
							{"pattern": map[string]interface{}{"glob": "**/*.xml"}},
							{"pattern": map[string]interface{}{"glob": "**/*.php"}},
						},
					},
					"didDelete": map[string]interface{}{
						"filters": []map[string]interface{}{
							{"pattern": map[string]interface{}{"glob": "**/*.xml"}},
							{"pattern": map[string]interface{}{"glob": "**/*.php"}},
						},
					},
				},
			},
		},
	}
}

// completion handles textDocument/completion requests
func (s *Server) completion(ctx context.Context, params *protocol.CompletionParams) *protocol.CompletionList {
	node, docText, ok := s.documentManager.GetNodeAtPosition(params.TextDocument.URI, params.Position.Line, params.Position.Character)
	if ok {
		params.Node = node
		params.DocumentContent = docText.Text
	}

	// Collect completion items from all providers
	var items []protocol.CompletionItem
	for _, provider := range s.completionProviders {
		providerItems := provider.GetCompletions(ctx, params)
		items = append(items, providerItems...)
	}

	// Return the completion list
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}

// extractRootPath extracts the root path from the initialize params
func (s *Server) extractRootPath(params *protocol.InitializeParams) {
	// Try to get from RootPath
	if params.RootPath != "" {
		s.rootPath = params.RootPath
		return
	}

	// Try to get from RootURI
	if params.RootURI != "" {
		rootURI := params.RootURI
		s.rootPath = strings.TrimPrefix(rootURI, "file://")
		return
	}

	// Try to get from WorkspaceFolders
	if len(params.WorkspaceFolders) > 0 {
		folder := params.WorkspaceFolders[0]
		s.rootPath = strings.TrimPrefix(folder.URI, "file://")
		return
	}

	// Fall back to current directory
	s.rootPath, _ = os.Getwd()
}

// collectTriggerCharacters collects all trigger characters from registered providers
func (s *Server) collectTriggerCharacters() []string {
	// Use a map to deduplicate trigger characters
	triggerCharsMap := make(map[string]bool)

	for _, provider := range s.completionProviders {
		for _, char := range provider.GetTriggerCharacters() {
			triggerCharsMap[char] = true
		}
	}

	// Convert map keys to slice
	triggerChars := make([]string, 0, len(triggerCharsMap))
	for char := range triggerCharsMap {
		triggerChars = append(triggerChars, char)
	}

	return triggerChars
}

func (s *Server) DocumentManager() *DocumentManager {
	return s.documentManager
}
