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
	"github.com/shopware/shopware-lsp/internal/twig"
)

func main() {
	log.SetFlags(0)

	// Get the current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	cacheDir, err := getProjectCacheFolder(projectRoot)
	if err != nil {
		log.Fatalf("Failed to get project config directory: %v", err)
	}

	log.Printf("Using cache directory: %s", cacheDir)

	filescanner, err := indexer.NewFileScanner(projectRoot, filepath.Join(cacheDir, "file_scanner.db"))
	if err != nil {
		log.Fatalf("Failed to create file scanner: %v", err)
	}

	server := lsp.NewServer(filescanner)

	server.RegisterIndexer(symfony.NewServiceIndex(projectRoot, cacheDir))
	server.RegisterIndexer(symfony.NewRouteIndexer(cacheDir))
	server.RegisterIndexer(php.NewPHPIndex(cacheDir))
	server.RegisterIndexer(twig.NewTwigIndexer(cacheDir))

	server.RegisterCompletionProvider(completion.NewServiceCompletionProvider(server))
	server.RegisterCompletionProvider(completion.NewTwigCompletionProvider(server))

	server.RegisterDefinitionProvider(definition.NewServiceXMLPHPDefinitionProvider(server))
	server.RegisterDefinitionProvider(definition.NewServiceXMLDefinitionProvider(server))
	server.RegisterDefinitionProvider(definition.NewTwigDefinitionProvider(server))

	server.RegisterCodeLensProvider(codelens.NewPHPCodeLensProvider(server))
	server.RegisterCodeLensProvider(codelens.NewTwigCodeLensProvider(server))

	if err := server.Start(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
