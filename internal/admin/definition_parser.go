package admin

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseComponentDefinition parses a Vue component definition file and extracts
// props, emits, methods, computed properties, and template path
func ParseComponentDefinition(root *tree_sitter.Node, content []byte) *ComponentDefinition {
	def := &ComponentDefinition{}

	// Find export default statement
	exportDefault := findExportDefault(root)
	if exportDefault == nil {
		return def
	}

	// Get the object being exported
	objNode := treesitterhelper.GetFirstNodeOfKind(exportDefault, "object")
	if objNode == nil {
		return def
	}

	// Parse the object properties
	for i := uint(0); i < objNode.ChildCount(); i++ {
		child := objNode.Child(i)

		switch child.Kind() {
		case "pair":
			parsePair(child, content, def)
		case "shorthand_property_identifier":
			// Handle shorthand like `template,`
			name := string(child.Utf8Text(content))
			if name == "template" {
				def.HasTemplate = true
			}
		}
	}

	// Find template import
	def.TemplatePath = findTemplateImport(root, content)

	return def
}

// ComponentDefinition holds the parsed component definition details
type ComponentDefinition struct {
	FilePath     string
	Props        []VueComponentProp
	Emits        []string
	Methods      []string
	Computed     []string
	Slots        []VueComponentSlot
	Blocks       []TwigBlock
	TemplatePath string
	HasTemplate  bool
}

// findExportDefault finds the export default statement in the AST
func findExportDefault(root *tree_sitter.Node) *tree_sitter.Node {
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() == "export_statement" {
			// Check if it has "default" keyword
			for j := uint(0); j < child.ChildCount(); j++ {
				if child.Child(j).Kind() == "default" {
					return child
				}
			}
		}
	}
	return nil
}

// parsePair parses a key-value pair in the component object
func parsePair(node *tree_sitter.Node, content []byte, def *ComponentDefinition) {
	// Get property name
	propIdent := treesitterhelper.GetFirstNodeOfKind(node, "property_identifier")
	if propIdent == nil {
		return
	}
	propName := string(propIdent.Utf8Text(content))

	// Get value node (second child after property_identifier and colon)
	var valueNode *tree_sitter.Node
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		kind := child.Kind()
		if kind == "object" || kind == "array" || kind == "identifier" {
			valueNode = child
			break
		}
	}

	if valueNode == nil {
		return
	}

	switch propName {
	case "props":
		def.Props = parseProps(valueNode, content)
	case "emits":
		def.Emits = parseEmits(valueNode, content)
	case "methods":
		def.Methods = parseMethods(valueNode, content)
	case "computed":
		def.Computed = parseMethods(valueNode, content) // Same structure as methods
	case "template":
		def.HasTemplate = true
	}
}

// parseProps parses the props object
func parseProps(node *tree_sitter.Node, content []byte) []VueComponentProp {
	var props []VueComponentProp

	if node.Kind() == "object" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "pair" {
				prop := parsePropDefinition(child, content)
				if prop != nil {
					props = append(props, *prop)
				}
			}
		}
	} else if node.Kind() == "array" {
		// Props can also be an array of strings
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "string" {
				propName := extractStringContent(child, content)
				if propName != "" {
					props = append(props, VueComponentProp{Name: propName})
				}
			}
		}
	}

	return props
}

// parsePropDefinition parses a single prop definition
func parsePropDefinition(node *tree_sitter.Node, content []byte) *VueComponentProp {
	// Get property name
	propIdent := treesitterhelper.GetFirstNodeOfKind(node, "property_identifier")
	if propIdent == nil {
		return nil
	}

	prop := &VueComponentProp{
		Name: string(propIdent.Utf8Text(content)),
		Line: int(propIdent.StartPosition().Row) + 1, // 1-based line number
	}

	// Get prop definition object
	propObj := treesitterhelper.GetFirstNodeOfKind(node, "object")
	if propObj == nil {
		// Prop might be defined with just a type: `title: String`
		typeIdent := treesitterhelper.GetFirstNodeOfKind(node, "identifier")
		if typeIdent != nil {
			prop.Type = string(typeIdent.Utf8Text(content))
		}
		return prop
	}

	// Parse prop options
	for i := uint(0); i < propObj.ChildCount(); i++ {
		child := propObj.Child(i)
		if child.Kind() == "pair" {
			optIdent := treesitterhelper.GetFirstNodeOfKind(child, "property_identifier")
			if optIdent == nil {
				continue
			}
			optName := string(optIdent.Utf8Text(content))

			switch optName {
			case "type":
				// Get the type identifier
				for j := uint(0); j < child.ChildCount(); j++ {
					c := child.Child(j)
					if c.Kind() == "identifier" {
						prop.Type = string(c.Utf8Text(content))
						break
					}
				}
			case "required":
				// Check if value is true
				for j := uint(0); j < child.ChildCount(); j++ {
					c := child.Child(j)
					if c.Kind() == "true" {
						prop.Required = true
						break
					}
				}
			case "default":
				// Get default value
				for j := uint(0); j < child.ChildCount(); j++ {
					c := child.Child(j)
					kind := c.Kind()
					if kind != "property_identifier" && kind != ":" {
						prop.Default = string(c.Utf8Text(content))
						break
					}
				}
			}
		}
	}

	return prop
}

// parseEmits parses the emits array
func parseEmits(node *tree_sitter.Node, content []byte) []string {
	var emits []string

	if node.Kind() != "array" {
		return emits
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "string" {
			emit := extractStringContent(child, content)
			if emit != "" {
				emits = append(emits, emit)
			}
		}
	}

	return emits
}

// parseMethods parses the methods/computed object
func parseMethods(node *tree_sitter.Node, content []byte) []string {
	var methods []string

	if node.Kind() != "object" {
		return methods
	}

	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		switch child.Kind() {
		case "method_definition":
			// Get method name
			propIdent := treesitterhelper.GetFirstNodeOfKind(child, "property_identifier")
			if propIdent != nil {
				methods = append(methods, string(propIdent.Utf8Text(content)))
			}
		case "pair":
			// Methods can also be defined as pairs with arrow functions
			propIdent := treesitterhelper.GetFirstNodeOfKind(child, "property_identifier")
			if propIdent != nil {
				methods = append(methods, string(propIdent.Utf8Text(content)))
			}
		}
	}

	return methods
}

// extractStringContent extracts the content from a string node
func extractStringContent(node *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == "string_fragment" {
			return string(child.Utf8Text(content))
		}
	}
	// Fallback: trim quotes
	text := string(node.Utf8Text(content))
	return strings.Trim(text, "\"'")
}

// findTemplateImport finds the template import statement
func findTemplateImport(root *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() == "import_statement" {
			// Check if it imports "template"
			importClause := treesitterhelper.GetFirstNodeOfKind(child, "import_clause")
			if importClause != nil {
				ident := treesitterhelper.GetFirstNodeOfKind(importClause, "identifier")
				if ident != nil && string(ident.Utf8Text(content)) == "template" {
					// Get the import path
					stringNode := treesitterhelper.GetFirstNodeOfKind(child, "string")
					if stringNode != nil {
						return extractStringContent(stringNode, content)
					}
				}
			}
		}
	}
	return ""
}
