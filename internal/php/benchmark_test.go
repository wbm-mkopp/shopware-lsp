package php

import (
	"os"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// Benchmark helper to parse a file and return the tree
func parseFile(b *testing.B, path string) (*tree_sitter.Parser, *tree_sitter.Tree, []byte) {
	b.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		b.Fatalf("Failed to read file: %v", err)
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		b.Fatalf("Failed to set language: %v", err)
	}

	tree := parser.Parse(content, nil)
	return parser, tree, content
}

// BenchmarkTreeSitterParsing measures raw tree-sitter parsing time
func BenchmarkTreeSitterParsing(b *testing.B) {
	content, err := os.ReadFile("testdata/01.php")
	if err != nil {
		b.Fatalf("Failed to read file: %v", err)
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		b.Fatalf("Failed to set language: %v", err)
	}
	defer parser.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree := parser.Parse(content, nil)
		tree.Close()
	}
}

// BenchmarkGetClassesOfFileWithParser measures full class extraction
func BenchmarkGetClassesOfFileWithParser(b *testing.B) {
	parser, tree, content := parseFile(b, "testdata/01.php")
	defer parser.Close()
	defer tree.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetClassesOfFileWithParser("testdata/01.php", tree.RootNode(), content)
	}
}

// BenchmarkGetClassesOfFileWithParser_LargeFile measures with a larger file
func BenchmarkGetClassesOfFileWithParser_LargeFile(b *testing.B) {
	parser, tree, content := parseFile(b, "testdata/03.php")
	defer parser.Close()
	defer tree.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetClassesOfFileWithParser("testdata/03.php", tree.RootNode(), content)
	}
}

// BenchmarkAliasResolver measures alias resolution performance
func BenchmarkAliasResolver(b *testing.B) {
	useStatements := map[string]string{
		"Request":  "Symfony\\Component\\HttpFoundation\\Request",
		"Response": "Symfony\\Component\\HttpFoundation\\Response",
		"TreeItem": "Shopware\\Core\\Content\\Category\\Tree\\TreeItem",
		"Package":  "Shopware\\Core\\Framework\\Log\\Package",
		"Criteria": "Shopware\\Core\\Framework\\DataAbstractionLayer\\Search\\Criteria",
	}
	aliases := map[string]string{
		"SymfonyRequest": "Symfony\\Component\\HttpFoundation\\Request",
	}
	resolver := NewAliasResolver("Shopware\\Core\\Content\\Product", useStatements, aliases)

	testCases := []string{
		"Request",          // in useStatements
		"SymfonyRequest",   // in aliases
		"string",           // primitive
		"self",             // special
		"UnknownClass",     // not found, should use namespace
		"Fully\\Qualified", // already qualified
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_ = resolver.ResolveType(tc)
		}
	}
}

// BenchmarkIsPrimitiveType measures primitive type checking
func BenchmarkIsPrimitiveType(b *testing.B) {
	types := []string{"string", "int", "array", "bool", "mixed", "void", "SomeClass", "AnotherClass"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			_ = isPrimitiveType(t)
		}
	}
}

// BenchmarkNewPHPType measures PHPType creation
func BenchmarkNewPHPType(b *testing.B) {
	types := []string{
		"string",
		"int",
		"Shopware\\Core\\Content\\Product\\ProductEntity",
		"array",
		"mixed",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range types {
			_ = NewPHPType(t)
		}
	}
}

// BenchmarkPHPIndex_GetClass measures database lookup performance
func BenchmarkPHPIndex_GetClass(b *testing.B) {
	idx, err := NewPHPIndex(b.TempDir())
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Index a file first
	parser, tree, content := parseFile(b, "testdata/01.php")
	defer parser.Close()
	defer tree.Close()

	err = idx.Index("testdata/01.php", tree.RootNode(), content)
	if err != nil {
		b.Fatalf("Failed to index: %v", err)
	}

	className := "Shopware\\Core\\Content\\Category\\Service\\NavigationLoader"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.GetClass(className)
	}
}

// BenchmarkPHPIndex_GetClassNames measures getting all class names
func BenchmarkPHPIndex_GetClassNames(b *testing.B) {
	idx, err := NewPHPIndex(b.TempDir())
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Index multiple files
	files := []string{"testdata/01.php", "testdata/02.php", "testdata/03.php"}
	for _, file := range files {
		parser, tree, content := parseFile(b, file)
		err = idx.Index(file, tree.RootNode(), content)
		parser.Close()
		tree.Close()
		if err != nil {
			b.Fatalf("Failed to index %s: %v", file, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.GetClassNames()
	}
}

// BenchmarkFindFirstNodeOfKind measures recursive node search
func BenchmarkFindFirstNodeOfKind(b *testing.B) {
	parser, tree, _ := parseFile(b, "testdata/01.php")
	defer parser.Close()
	defer tree.Close()

	root := tree.RootNode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = findFirstNodeOfKind(root, "class_declaration")
	}
}

// BenchmarkFindDirectChildOfKind measures direct child search
func BenchmarkFindDirectChildOfKind(b *testing.B) {
	parser, tree, _ := parseFile(b, "testdata/01.php")
	defer parser.Close()
	defer tree.Close()

	root := tree.RootNode()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = findDirectChildOfKind(root, "class_declaration")
	}
}

// BenchmarkFullIndexingWorkflow measures the complete indexing workflow
func BenchmarkFullIndexingWorkflow(b *testing.B) {
	content, err := os.ReadFile("testdata/01.php")
	if err != nil {
		b.Fatalf("Failed to read file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate the full workflow: parse + extract classes
		parser := tree_sitter.NewParser()
		_ = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
		tree := parser.Parse(content, nil)
		_ = GetClassesOfFileWithParser("testdata/01.php", tree.RootNode(), content)
		tree.Close()
		parser.Close()
	}
}
