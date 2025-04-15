package main

import (
	"log"
	"os"

	"github.com/shopware/shopware-lsp/lsp"
)

func main() {
	log.SetFlags(0)
	if err := lsp.RunServer(os.Stdin, os.Stdout); err != nil {
		log.Fatalf("LSP server error: %v", err)
	}
}
