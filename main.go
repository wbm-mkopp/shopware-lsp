package main

import (
	"log"
	"os"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/symfony"
)

func main() {
	log.SetFlags(0)
	server := lsp.NewServer()

	// Get the current working directory as project root
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	server.RegisterIndexer(symfony.NewServiceIndex(projectRoot))
	server.RegisterCompletionProvider(symfony.NewServiceCompletionProvider(server))

	if err := server.Start(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
