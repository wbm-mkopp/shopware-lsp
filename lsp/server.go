package lsp

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"

	"github.com/shopware/shopware-lsp/symfony"
	"github.com/sourcegraph/jsonrpc2"
)

// Server represents the LSP server
type Server struct {
	serviceIndex *symfony.ServiceIndex
	rootPath     string
	conn         *jsonrpc2.Conn
}

// NewServer creates a new LSP server
func NewServer() *Server {
	return &Server{}
}

// InitializeParams represents the parameters for the 'initialize' request
type InitializeParams struct {
	RootPath        string             `json:"rootPath,omitempty"`
	RootURI         string             `json:"rootUri,omitempty"`
	WorkspaceFolders []WorkspaceFolder `json:"workspaceFolders,omitempty"`
}

// WorkspaceFolder represents a workspace folder
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// CompletionParams represents the parameters for the 'textDocument/completion' request
type CompletionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Position struct {
		Line      int `json:"line"`
		Character int `json:"character"`
	} `json:"position"`
}

// CompletionItem represents a completion item
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"`
	Documentation struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"documentation"`
}

// CompletionList represents a list of completion items
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// RunServer starts the LSP server
func RunServer(in io.Reader, out io.Writer) error {
	server := NewServer()
	
	// Create a new JSON-RPC connection
	stream := jsonrpc2.NewBufferedStream(rwc{in, out}, jsonrpc2.VSCodeObjectCodec{})
	conn := jsonrpc2.NewConn(context.Background(), stream, jsonrpc2.HandlerWithError(server.handle))
	server.conn = conn
	
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
		conn.Close()
		return nil, nil
	}

	switch req.Method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: err.Error()}
		}
		return s.initialize(ctx, &params), nil

	case "initialized":
		// Build the index when the client is initialized
		if s.serviceIndex != nil {
			go func() {
				if err := s.serviceIndex.BuildIndex(); err != nil {
					log.Printf("Error building index: %v", err)
				}
				// Send service count notification after indexing
				s.sendServiceCountNotification()
			}()
		}
		return nil, nil

	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(*req.Params, &params); err != nil {
			return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeParseError, Message: err.Error()}
		}
		return s.completion(ctx, &params), nil

	case "shutdown":
		// Clean up resources
		if s.serviceIndex != nil {
			_ = s.serviceIndex.Close()
		}
		
		log.Println("Received shutdown request, waiting for exit notification")
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
func (s *Server) initialize(ctx context.Context, params *InitializeParams) interface{} {
	// Extract root path from params
	s.extractRootPath(params)

	// Initialize the service indexer
	var err error
	s.serviceIndex, err = symfony.NewServiceIndex(s.rootPath)
	if err != nil {
		log.Printf("Error initializing service index: %v", err)
	}

	// Define server capabilities
	return map[string]interface{}{
		"capabilities": map[string]interface{}{
			"textDocumentSync": map[string]interface{}{
				"openClose": true,
				"change":    1, // Full sync
			},
			"completionProvider": map[string]interface{}{
				"triggerCharacters": []string{"@", "'", "\""},
			},
		},
	}
}

// completion handles the LSP completion request
func (s *Server) completion(ctx context.Context, params *CompletionParams) interface{} {
	if s.serviceIndex == nil {
		return CompletionList{
			IsIncomplete: false,
			Items:        []CompletionItem{},
		}
	}

	// Get all services from the index
	serviceIDs := s.serviceIndex.GetAllServices()

	// Convert to completion items
	items := make([]CompletionItem, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		item := CompletionItem{
			Label: serviceID,
			Kind:  6, // 6 = Class
		}
		
		// Try to get detailed service information
		if service, found := s.serviceIndex.GetServiceByID(serviceID); found {
			// Add class information to documentation
			documentation := "Symfony service ID\n\n"
			
			// Add class information
			if service.Class != "" {
				documentation += "**Class:** `" + service.Class + "`\n\n"
			}
			
			// Add tags information if available
			if len(service.Tags) > 0 {
				documentation += "**Tags:**\n"
				for tag := range service.Tags {
					documentation += "- " + tag + "\n"
				}
			}
			
			item.Documentation.Kind = "markdown"
			item.Documentation.Value = documentation
		} else {
			// Default documentation
			item.Documentation.Kind = "markdown"
			item.Documentation.Value = "Symfony service ID"
		}
		
		items = append(items, item)
	}

	return CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}

// extractRootPath extracts the root path from the initialize params
func (s *Server) extractRootPath(params *InitializeParams) {
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

// sendServiceCountNotification sends a notification to the client with service count information
func (s *Server) sendServiceCountNotification() {
	if s.conn == nil || s.serviceIndex == nil {
		return
	}
	
	// Get service and alias counts
	serviceCount, aliasCount := s.serviceIndex.GetCounts()
	totalCount := serviceCount + aliasCount
	
	// Create notification params
	params := map[string]interface{}{
		"serviceCount": serviceCount,
		"aliasCount":   aliasCount,
		"total":        totalCount,
	}
	
	// Send the notification
	err := s.conn.Notify(context.Background(), "symfony/serviceCount", params)
	if err != nil {
		log.Printf("Error sending service count notification: %v", err)
		return
	}
	
	log.Printf("Sent service count notification: %d services, %d aliases, %d total", 
		serviceCount, aliasCount, totalCount)
}
