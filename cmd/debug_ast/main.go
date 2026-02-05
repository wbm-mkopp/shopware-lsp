package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func main() {
	lang := flag.String("lang", "", "Language to parse (php, js, twig, json). Auto-detected from extension if not specified.")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: go run cmd/debug_ast/main.go [-lang=php|js|twig|json] <file_path>")
		fmt.Println("       go run cmd/debug_ast/main.go [-lang=php|js|twig|json] - < input.txt")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  -lang    Language to parse (php, js, twig, json)")
		fmt.Println("           Auto-detected from file extension if not specified")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  go run cmd/debug_ast/main.go example.php")
		fmt.Println("  go run cmd/debug_ast/main.go -lang=js example.vue")
		fmt.Println("  echo \"this.\\$tc('key')\" | go run cmd/debug_ast/main.go -lang=js -")
		os.Exit(1)
	}

	filePath := args[0]

	var fileContent []byte
	var err error

	if filePath == "-" {
		// Read from stdin
		fileContent, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Printf("Error reading stdin: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Analyzing AST from stdin")
	} else {
		fileContent, err = os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Analyzing AST for file: %s\n\n", filePath)
	}

	// Determine language
	language := *lang
	if language == "" && filePath != "-" {
		language = detectLanguage(filePath)
	}

	if language == "" {
		fmt.Println("Error: Could not detect language. Please specify -lang flag.")
		os.Exit(1)
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	var langErr error
	switch strings.ToLower(language) {
	case "php":
		langErr = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
	case "js", "javascript":
		langErr = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language()))
	case "twig":
		langErr = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))
	case "json":
		langErr = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_json.Language()))
	default:
		fmt.Printf("Error: Unsupported language '%s'. Supported: php, js, twig, json\n", language)
		os.Exit(1)
	}

	if langErr != nil {
		fmt.Printf("Error setting language: %v\n", langErr)
		os.Exit(1)
	}

	tree := parser.Parse(fileContent, nil)
	if tree == nil {
		fmt.Println("Error: Failed to parse content")
		os.Exit(1)
	}
	defer tree.Close()

	fmt.Printf("Language: %s\n", language)
	fmt.Printf("Content:\n---\n%s\n---\n\n", string(fileContent))

	printNodeStructure(tree.RootNode(), fileContent, 0)
}

func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".php":
		return "php"
	case ".js", ".mjs", ".cjs":
		return "js"
	case ".twig":
		return "twig"
	case ".json":
		return "json"
	default:
		return ""
	}
}

func printNodeStructure(node *tree_sitter.Node, fileContent []byte, depth int) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	// Get position info
	startPos := node.StartPosition()
	endPos := node.EndPosition()

	nodeText := ""
	if node.NamedChildCount() == 0 {
		text := string(node.Utf8Text(fileContent))
		// Truncate long text
		if len(text) > 50 {
			text = text[:47] + "..."
		}
		// Escape newlines for display
		text = strings.ReplaceAll(text, "\n", "\\n")
		text = strings.ReplaceAll(text, "\r", "\\r")
		text = strings.ReplaceAll(text, "\t", "\\t")
		nodeText = fmt.Sprintf(" = %q", text)
	}

	fmt.Printf("%s%s [%d:%d-%d:%d]%s\n",
		indent,
		node.Kind(),
		startPos.Row, startPos.Column,
		endPos.Row, endPos.Column,
		nodeText,
	)

	// Recursively print child nodes
	for i := uint(0); i < node.NamedChildCount(); i++ {
		printNodeStructure(node.NamedChild(i), fileContent, depth+1)
	}
}
