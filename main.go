package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/codelens"
	"github.com/shopware/shopware-lsp/internal/lsp/completion"
	"github.com/shopware/shopware-lsp/internal/lsp/definition"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/symfony"
)

func main() {
	log.SetFlags(0)

	// Get the current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	configDir, err := getProjectConfigFolder(projectRoot)
	if err != nil {
		log.Fatalf("Failed to get project config directory: %v", err)
	}

	filescanner, err := indexer.NewFileScanner(projectRoot, filepath.Join(configDir, "file_scanner.db"))
	if err != nil {
		log.Fatalf("Failed to create file scanner: %v", err)
	}

	server := lsp.NewServer(filescanner)

	server.RegisterIndexer(symfony.NewServiceIndex(projectRoot, configDir))
	server.RegisterIndexer(php.NewPHPIndex(projectRoot, configDir))

	server.RegisterCompletionProvider(completion.NewServiceCompletionProvider(server))

	server.RegisterDefinitionProvider(definition.NewServiceXMLPHPDefinitionProvider(server))
	server.RegisterDefinitionProvider(definition.NewServiceXMLDefinitionProvider(server))

	server.RegisterCodeLensProvider(codelens.NewPHPCodeLensProvider(server))

	if err := server.Start(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
