package twig

import (
	"testing"
	"unsafe"

	treesitter "github.com/tree-sitter/go-tree-sitter"
)

func TestLanguage(t *testing.T) {
	lang := Language()
	if lang == nil {
		t.Error("Expected language binding to return a valid pointer")
	}
	if lang == unsafe.Pointer(nil) {
		t.Error("Expected language binding to return a non-nil unsafe pointer")
	}
}

func TestParseTwig(t *testing.T) {
	parser := treesitter.NewParser()
	if err := parser.SetLanguage(treesitter.NewLanguage(Language())); err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}

	source := []byte(`{% extends "base.html" %}`)
	tree := parser.Parse(source, nil)

	root := tree.RootNode()
	if root.Kind() != "template" {
		t.Errorf("Expected root node to be 'template', got %s", root.Kind())
	}
}

func TestParseHTMLTags(t *testing.T) {
	parser := treesitter.NewParser()
	if err := parser.SetLanguage(treesitter.NewLanguage(Language())); err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}
	defer parser.Close()

	source := []byte(`<sw-button variant="ghost">Click me</sw-button>`)
	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	if root.Kind() != "template" {
		t.Errorf("Expected root node to be 'template', got %s", root.Kind())
	}

	// Print tree structure for debugging
	t.Logf("Root: %s, children: %d", root.Kind(), root.ChildCount())
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		t.Logf("  Child %d: %s = %s", i, child.Kind(), string(child.Utf8Text(source)))
	}

	// Check if we have an html_tag node
	foundHTMLTag := false
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() == "html_tag" || child.Kind() == "html_start_tag" {
			foundHTMLTag = true
			break
		}
	}

	if !foundHTMLTag {
		t.Error("Expected to find an html_tag or html_start_tag node")
	}
}

func TestParseHTMLWithTwig(t *testing.T) {
	parser := treesitter.NewParser()
	if err := parser.SetLanguage(treesitter.NewLanguage(Language())); err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}
	defer parser.Close()

	source := []byte(`{% block test %}<sw-button @click="doSomething">{{ label }}</sw-button>{% endblock %}`)
	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	t.Logf("Root: %s, children: %d", root.Kind(), root.ChildCount())

	var printNode func(node *treesitter.Node, depth int)
	printNode = func(node *treesitter.Node, depth int) {
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		text := ""
		if node.ChildCount() == 0 {
			text = string(node.Utf8Text(source))
		}
		t.Logf("%s%s: %s", indent, node.Kind(), text)
		for i := uint(0); i < node.ChildCount(); i++ {
			printNode(node.Child(i), depth+1)
		}
	}
	printNode(root, 0)
}

func TestParseSlotShorthand(t *testing.T) {
	parser := treesitter.NewParser()
	if err := parser.SetLanguage(treesitter.NewLanguage(Language())); err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}
	defer parser.Close()

	source := []byte(`<sw-page><template #default>Content</template></sw-page>`)
	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	t.Logf("=== Full AST ===")

	var printNode func(node *treesitter.Node, depth int)
	printNode = func(node *treesitter.Node, depth int) {
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		text := ""
		if node.ChildCount() == 0 {
			text = string(node.Utf8Text(source))
		}
		t.Logf("%s%s: %q", indent, node.Kind(), text)
		for i := uint(0); i < node.ChildCount(); i++ {
			printNode(node.Child(i), depth+1)
		}
	}
	printNode(root, 0)

	// Look for vue_directive with #default
	foundSlotDirective := false
	var findSlot func(node *treesitter.Node)
	findSlot = func(node *treesitter.Node) {
		if node.Kind() == "vue_directive" {
			text := string(node.Utf8Text(source))
			t.Logf("Found vue_directive: %s", text)
			if text == "#default" {
				foundSlotDirective = true
			}
		}
		for i := uint(0); i < node.ChildCount(); i++ {
			findSlot(node.Child(i))
		}
	}
	findSlot(root)

	if !foundSlotDirective {
		t.Error("Expected to find vue_directive with #default")
	}
}

func TestParseHTMLAttributes(t *testing.T) {
	parser := treesitter.NewParser()
	if err := parser.SetLanguage(treesitter.NewLanguage(Language())); err != nil {
		t.Fatalf("Failed to set language: %v", err)
	}
	defer parser.Close()

	source := []byte(`<sw-button variant="primary" :disabled="true" @click="onClick"></sw-button>`)
	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	t.Logf("=== Full AST ===")

	var printNode func(node *treesitter.Node, depth int)
	printNode = func(node *treesitter.Node, depth int) {
		indent := ""
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		text := ""
		if node.ChildCount() == 0 {
			text = string(node.Utf8Text(source))
		}
		t.Logf("%s%s: %q", indent, node.Kind(), text)
		for i := uint(0); i < node.ChildCount(); i++ {
			printNode(node.Child(i), depth+1)
		}
	}
	printNode(root, 0)
}
