package main

import (
	"fmt"
	"os"

	"github.com/shopware/shopware-lsp/internal/php"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/debug_ast/main.go <php_file_path>")
		os.Exit(1)
	}

	filePath := os.Args[1]
	fmt.Printf("Analyzing AST for file: %s\n\n", filePath)

	php.DebugAST(filePath)
}
