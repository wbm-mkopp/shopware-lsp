package symfony

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseYAMLRoutes parses Symfony YAML route definitions using tree-sitter
func ParseYAMLRoutes(filePath string, rootNode *tree_sitter.Node, content []byte) ([]Route, error) {
	// Pre-allocate with reasonable capacity
	routes := make([]Route, 0, 20)

	// Get the document node
	documentNode := getDocumentNode(rootNode)
	if documentNode == nil {
		return routes, nil
	}

	// Get the root mapping node
	rootMappingNode := getRootMappingNode(documentNode)
	if rootMappingNode == nil {
		return routes, nil
	}

	// Process all route definitions (top-level mapping pairs)
	for i := 0; i < int(rootMappingNode.NamedChildCount()); i++ {
		pair := rootMappingNode.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		// Get route name key node
		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		// Get route name from the key node
		routeName := extractNodeText(keyNode, content)
		if routeName == "" || strings.HasPrefix(routeName, "_") {
			continue // Skip empty names or internal entries
		}

		// Create a route with default values
		route := Route{
			Name:     routeName,
			FilePath: filePath,
			Line:     int(keyNode.StartPosition().Row) + 1, // Line numbers start from 1
		}

		// Get route definition value node
		valueNode := pair.NamedChild(1)
		if valueNode == nil {
			continue
		}

		// Process the route definition
		processRouteDefinition(&route, valueNode, content)

		// Only include valid routes (must have a path)
		if route.Path != "" {
			routes = append(routes, route)
		}
	}

	return routes, nil
}

// getDocumentNode gets the document node from the root node
func getDocumentNode(rootNode *tree_sitter.Node) *tree_sitter.Node {
	if rootNode == nil {
		return nil
	}

	// In YAML tree-sitter, the root is a "stream" with multiple children
	if rootNode.Kind() == "stream" {
		// Look for document node in children
		for i := 0; i < int(rootNode.NamedChildCount()); i++ {
			child := rootNode.NamedChild(uint(i))
			if child == nil {
				continue
			}

			if child.Kind() == "document" {
				return child
			}
		}

		// If no document node found but we have a child, use the second child (skip comment)
		if rootNode.NamedChildCount() > 1 {
			child := rootNode.NamedChild(1) // Try second child, first might be comment
			return child
		} else if rootNode.NamedChildCount() > 0 {
			child := rootNode.NamedChild(0)
			return child
		}
	} else if rootNode.Kind() == "document" {
		return rootNode // Root is already a document
	}

	// Try to find any document node
	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(uint(i))
		if child != nil && child.Kind() == "document" {
			return child
		}
	}

	// If no document node found, return the root node as a fallback
	return rootNode
}

// getRootMappingNode gets the root mapping node from the document node
func getRootMappingNode(documentNode *tree_sitter.Node) *tree_sitter.Node {
	if documentNode == nil || documentNode.NamedChildCount() == 0 {
		return nil
	}

	// Try different approaches to find the block_mapping

	// Approach 1: First named child
	firstChild := documentNode.NamedChild(0)
	if firstChild != nil {
		// Direct block_mapping
		if firstChild.Kind() == "block_mapping" {
			return firstChild
		}

		// Block_node with block_mapping child
		if firstChild.Kind() == "block_node" && firstChild.NamedChildCount() > 0 {
			blockChild := firstChild.NamedChild(0)
			if blockChild != nil && blockChild.Kind() == "block_mapping" {
				return blockChild
			}
		}
	}

	// Approach 2: Look for block_mapping directly
	for i := 0; i < int(documentNode.NamedChildCount()); i++ {
		child := documentNode.NamedChild(uint(i))
		if child != nil && child.Kind() == "block_mapping" {
			return child
		}
	}

	// Approach 3: Look for block_mapping in deeper levels
	for i := 0; i < int(documentNode.NamedChildCount()); i++ {
		child := documentNode.NamedChild(uint(i))
		if child == nil {
			continue
		}

		// Search one level deeper for block_mapping
		for j := 0; j < int(child.NamedChildCount()); j++ {
			grandchild := child.NamedChild(uint(j))
			if grandchild != nil && grandchild.Kind() == "block_mapping" {
				return grandchild
			}
		}
	}

	// Document node itself might be a block_mapping (rare but possible)
	if documentNode.Kind() == "block_mapping" {
		return documentNode
	}

	return nil
}

// processRouteDefinition extracts route configuration from a route definition node
func processRouteDefinition(route *Route, node *tree_sitter.Node, content []byte) {
	// Get mapping node from the route definition
	var mappingNode *tree_sitter.Node

	// The value might be a block_node containing a block_mapping
	if node.Kind() == "block_node" && node.NamedChildCount() > 0 {
		child := node.NamedChild(0)
		if child != nil && child.Kind() == "block_mapping" {
			mappingNode = child
		}
	} else if node.Kind() == "block_mapping" {
		mappingNode = node
	}

	if mappingNode == nil {
		return
	}

	// Process each attribute in the route definition
	for i := 0; i < int(mappingNode.NamedChildCount()); i++ {
		pair := mappingNode.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		// Get attribute key node
		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		// Get attribute name
		attrName := extractNodeText(keyNode, content)
		if attrName == "" {
			continue
		}

		// Get attribute value node
		valueNode := pair.NamedChild(1)
		if valueNode == nil {
			continue
		}

		// Process based on attribute name
		switch attrName {
		case "path":
			route.Path = extractScalarValue(valueNode, content)
		case "controller":
			route.Controller = extractScalarValue(valueNode, content)
		}
	}
}

// extractNodeText extracts text from a node, handling common YAML node types
func extractNodeText(node *tree_sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Handle flow_node (the most common case)
	if node.Kind() == "flow_node" {
		// Try to find plain_scalar child
		plainScalar := treesitterhelper.GetFirstNodeOfKind(node, "plain_scalar")
		if plainScalar != nil {
			return strings.TrimSpace(string(plainScalar.Utf8Text(content)))
		}
		// Fallback to direct text
		return strings.TrimSpace(string(node.Utf8Text(content)))
	}

	// Direct text for other node types
	return strings.TrimSpace(string(node.Utf8Text(content)))
}

// extractScalarValue extracts a scalar value from a node, handling different node types
func extractScalarValue(node *tree_sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Handle different node types
	nodeKind := node.Kind()

	// Handle flow_node (direct scalar value)
	if nodeKind == "flow_node" {
		// Try to find plain_scalar child
		plainScalar := treesitterhelper.GetFirstNodeOfKind(node, "plain_scalar")
		if plainScalar != nil {
			return strings.Trim(string(plainScalar.Utf8Text(content)), "\"'")
		}
		// Fallback to direct text
		return strings.Trim(string(node.Utf8Text(content)), "\"'")
	}

	// Handle block_node with flow_node child
	if nodeKind == "block_node" && node.NamedChildCount() > 0 {
		firstChild := node.NamedChild(0)
		if firstChild != nil && firstChild.Kind() == "flow_node" {
			// Try to find plain_scalar child
			plainScalar := treesitterhelper.GetFirstNodeOfKind(firstChild, "plain_scalar")
			if plainScalar != nil {
				return strings.Trim(string(plainScalar.Utf8Text(content)), "\"'")
			}
			// Fallback to direct text
			return strings.Trim(string(firstChild.Utf8Text(content)), "\"'")
		}
	}

	// For other cases, just return empty
	return ""
}
