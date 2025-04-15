package main

import (
	"log"
	"os"

	"github.com/shopware/shopware-lsp/lsp"
	"github.com/shopware/shopware-lsp/symfony"
)

func main() {
	log.SetFlags(0)
	server := lsp.NewServer()

	// Get the current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Create and register the Symfony service indexer
	serviceIndexer, err := symfony.NewServiceIndexer(projectRoot)
	if err != nil {
		log.Printf("Warning: Failed to create Symfony service indexer: %v", err)
	} else {
		server.RegisterIndexer(serviceIndexer)

		// Register completion providers that use the service indexer
		server.RegisterCompletionProvider(symfony.NewServiceCompletionProvider(serviceIndexer.GetServiceIndex(), server))
		server.RegisterCompletionProvider(symfony.NewTagCompletionProvider(serviceIndexer.GetServiceIndex(), server))
	}

	if err := server.Start(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
