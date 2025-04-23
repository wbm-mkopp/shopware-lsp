package indexer

import (
	tree_sitter_twig "github.com/kaermorchen/tree-sitter-twig/bindings/go"
	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

var scannedFileTypes = []string{
	".php",
	".xml",
	".twig",
}

func createTreesitterParsers() map[string]*tree_sitter.Parser {
	parsers := make(map[string]*tree_sitter.Parser)

	parsers["php"] = tree_sitter.NewParser()
	parsers["php"].SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))

	parsers["xml"] = tree_sitter.NewParser()
	parsers["xml"].SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML()))

	parsers["twig"] = tree_sitter.NewParser()
	parsers["twig"].SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))

	return parsers
}
