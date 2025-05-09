package extension

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ManifestMeta represents metadata from a Shopware app manifest.xml file
type ManifestMeta struct {
	Name        string
	Label       string
	Description string
	Author      string
	Copyright   string
	Version     string
	License     string
	Path        string // Source file path
}

// ParseManifestXml parses a Shopware app manifest.xml file and extracts metadata
func ParseManifestXml(path string, rootNode *tree_sitter.Node, data []byte) (*ManifestMeta, error) {
	// Create manifest with default values
	manifest := &ManifestMeta{
		Path: path,
	}

	// Process manifest node
	manifestNode := findManifestNode(rootNode, data)
	if manifestNode == nil {
		return nil, nil
	}

	// Find meta node inside manifest
	metaNode := findMetaNodeInManifest(manifestNode, data)
	if metaNode == nil {
		return nil, nil
	}

	// Process meta node to extract metadata
	processMetaNode(metaNode, data, manifest)

	return manifest, nil
}

// findManifestNode finds the manifest node in the XML tree
func findManifestNode(rootNode *tree_sitter.Node, data []byte) *tree_sitter.Node {
	// Find a direct manifest element
	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(uint(i))
		if child.Kind() != "element" {
			continue
		}

		// Get element's STag or EmptyElemTag
		elementTag := child.NamedChild(0)
		if elementTag == nil {
			continue
		}

		// Get element name
		nameNode := treesitterhelper.GetFirstNodeOfKind(elementTag, "Name")
		if nameNode == nil {
			continue
		}

		// Check if it's the manifest element
		elementName := nameNode.Utf8Text(data)
		if string(elementName) == "manifest" {
			return child
		}
	}

	return nil
}

// findMetaNodeInManifest finds the meta node inside the manifest element
func findMetaNodeInManifest(manifestNode *tree_sitter.Node, data []byte) *tree_sitter.Node {
	// Get content node of manifest
	if manifestNode.NamedChildCount() < 2 {
		return nil
	}

	contentNode := manifestNode.NamedChild(1)
	if contentNode == nil || contentNode.Kind() != "content" {
		return nil
	}

	// Process all elements in content
	childCount := int(contentNode.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := contentNode.NamedChild(uint(i))
		if child.Kind() != "element" {
			continue
		}

		// Get element's STag or EmptyElemTag
		elementTag := child.NamedChild(0)
		if elementTag == nil {
			continue
		}

		// Get element name
		nameNode := treesitterhelper.GetFirstNodeOfKind(elementTag, "Name")
		if nameNode == nil {
			continue
		}

		// Check if it's the meta element
		elementName := nameNode.Utf8Text(data)
		if string(elementName) == "meta" {
			return child
		}
	}

	return nil
}

// processMetaNode extracts metadata from the meta element
func processMetaNode(metaNode *tree_sitter.Node, data []byte, manifest *ManifestMeta) {
	// Get content node of meta
	if metaNode.NamedChildCount() < 2 {
		return
	}

	contentNode := metaNode.NamedChild(1)
	if contentNode == nil || contentNode.Kind() != "content" {
		return
	}

	// Process all meta elements
	childCount := int(contentNode.NamedChildCount())
	for i := 0; i < childCount; i++ {
		child := contentNode.NamedChild(uint(i))
		if child.Kind() != "element" {
			continue
		}

		// Get element's STag or EmptyElemTag
		elementTag := child.NamedChild(0)
		if elementTag == nil {
			continue
		}

		// Get element name
		nameNode := treesitterhelper.GetFirstNodeOfKind(elementTag, "Name")
		if nameNode == nil {
			continue
		}

		// Get element value
		elementName := string(nameNode.Utf8Text(data))
		value := extractElementTextContent(child, data)

		// Set appropriate field based on element name
		switch elementName {
		case "name":
			manifest.Name = value
		case "label":
			manifest.Label = value
		case "description":
			manifest.Description = value
		case "author":
			manifest.Author = value
		case "copyright":
			manifest.Copyright = value
		case "version":
			manifest.Version = value
		case "license":
			manifest.License = value
		}
	}
}

// extractElementTextContent extracts text content from an XML element
func extractElementTextContent(node *tree_sitter.Node, data []byte) string {
	// Check if the node has content
	if node.NamedChildCount() < 2 {
		return ""
	}

	contentNode := node.NamedChild(1)
	if contentNode == nil || contentNode.Kind() != "content" {
		return ""
	}

	// Find CharData node in content
	for i := 0; i < int(contentNode.NamedChildCount()); i++ {
		child := contentNode.NamedChild(uint(i))
		if child.Kind() == "CharData" {
			return strings.TrimSpace(string(child.Utf8Text(data)))
		}
	}

	return ""
}
