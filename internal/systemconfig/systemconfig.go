package systemconfig

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SystemConfigField represents a field in the system config XML
type SystemConfigField struct {
	Name      string
	Label     string
	Type      string
	Component string
	FilePath  string
	Line      uint32
}

// IsSystemConfigXML checks if the XML file is a system config XML file
func IsSystemConfigXML(content []byte) bool {
	return strings.Contains(string(content), "SystemConfig/Schema/config.xsd")
}

// SystemConfigPattern matches a system config field (input-field or component)
var SystemConfigPattern = treesitterhelper.Or(
	treesitterhelper.And(
		treesitterhelper.NodeKind("element"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("STag"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("input-field"),
			)),
		)),
	),
	treesitterhelper.And(
		treesitterhelper.NodeKind("element"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("STag"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("component"),
			)),
		)),
	),
)

// GetSystemConfigFieldName extracts the name from a system config field node
func GetSystemConfigFieldName(node *tree_sitter.Node, content []byte) string {
	// Find the content node
	contentNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("content"), content)
	if contentNode == nil {
		return ""
	}

	// Look for the 'n' or 'name' element in the content
	for i := 0; i < int(contentNode.NamedChildCount()); i++ {
		child := contentNode.NamedChild(uint(i))
		if child.Kind() == "element" {
			// Check if this is the 'n' element
			nNameNode := treesitterhelper.FindFirst(child, treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("n"),
			), content)

			// Check if this is the 'name' element
			nameNode := treesitterhelper.FindFirst(child, treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("name"),
			), content)

			if nNameNode != nil || nameNode != nil {
				// Find the CharData within the content of this element
				childContentNode := treesitterhelper.FindFirst(child, treesitterhelper.NodeKind("content"), content)
				if childContentNode != nil {
					charDataNode := treesitterhelper.FindFirst(childContentNode, treesitterhelper.NodeKind("CharData"), content)
					if charDataNode != nil {
						return strings.TrimSpace(string(charDataNode.Utf8Text(content)))
					}
				}
			}
		}
	}
	return ""
}

// GetSystemConfigFieldLabel extracts the label from a system config field node
func GetSystemConfigFieldLabel(node *tree_sitter.Node, content []byte) string {
	// Find the content node
	contentNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("content"), content)
	if contentNode == nil {
		return ""
	}

	// Look for all label elements in the content
	for i := 0; i < int(contentNode.NamedChildCount()); i++ {
		child := contentNode.NamedChild(uint(i))
		if child.Kind() == "element" {
			// Check if this is a 'label' element
			nameNode := treesitterhelper.FindFirst(child, treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("label"),
			), content)

			if nameNode != nil {
				// Check if it has a lang attribute
				sTagNode := treesitterhelper.FindFirst(child, treesitterhelper.NodeKind("STag"), content)
				if sTagNode != nil {
					langAttr := treesitterhelper.FindFirst(sTagNode, treesitterhelper.And(
						treesitterhelper.NodeKind("Attribute"),
						treesitterhelper.HasChild(treesitterhelper.And(
							treesitterhelper.NodeKind("Name"),
							treesitterhelper.NodeText("lang"),
						)),
					), content)

					// Skip if it has a lang attribute
					if langAttr != nil {
						continue
					}
				}

				// Find the CharData within the content of this element
				childContentNode := treesitterhelper.FindFirst(child, treesitterhelper.NodeKind("content"), content)
				if childContentNode != nil {
					charDataNode := treesitterhelper.FindFirst(childContentNode, treesitterhelper.NodeKind("CharData"), content)
					if charDataNode != nil {
						return strings.TrimSpace(string(charDataNode.Utf8Text(content)))
					}
				}
			}
		}
	}
	return ""
}

// GetSystemConfigFieldType extracts the type from an input-field node
func GetSystemConfigFieldType(node *tree_sitter.Node, content []byte) string {
	// For input-field, get the type attribute
	if node.Kind() == "document" {
		// Get the element node
		elementNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("element"), content)
		if elementNode == nil {
			return ""
		}
		node = elementNode
	}

	if node.Kind() == "element" {
		// Check if this is an input-field element
		sTagNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("STag"), content)
		if sTagNode == nil {
			return ""
		}

		nameNode := treesitterhelper.FindFirst(sTagNode, treesitterhelper.And(
			treesitterhelper.NodeKind("Name"),
			treesitterhelper.NodeText("input-field"),
		), content)

		if nameNode != nil {
			// Look for all children in the STag
			for i := 0; i < int(sTagNode.ChildCount()); i++ {
				child := sTagNode.Child(uint(i))
				if child.Kind() == "Attribute" {
					// Check if this is the type attribute
					for j := 0; j < int(child.ChildCount()); j++ {
						attrChild := child.Child(uint(j))
						if attrChild.Kind() == "Name" && string(attrChild.Utf8Text(content)) == "type" {
							// Find the AttValue node
							for k := 0; k < int(child.ChildCount()); k++ {
								valueChild := child.Child(uint(k))
								if valueChild.Kind() == "AttValue" {
									// Remove quotes from attribute value
									value := string(valueChild.Utf8Text(content))
									return strings.Trim(value, "\"'")
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// GetSystemConfigComponent extracts the component name from a component node
func GetSystemConfigComponent(node *tree_sitter.Node, content []byte) string {
	// For component, get the name attribute
	if node.Kind() == "document" {
		// Get the element node
		elementNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("element"), content)
		if elementNode == nil {
			return ""
		}
		node = elementNode
	}

	if node.Kind() == "element" {
		// Check if this is a component element
		sTagNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("STag"), content)
		if sTagNode == nil {
			return ""
		}

		nameNode := treesitterhelper.FindFirst(sTagNode, treesitterhelper.And(
			treesitterhelper.NodeKind("Name"),
			treesitterhelper.NodeText("component"),
		), content)

		if nameNode != nil {
			// Look for all children in the STag
			for i := 0; i < int(sTagNode.ChildCount()); i++ {
				child := sTagNode.Child(uint(i))
				if child.Kind() == "Attribute" {
					// Check if this is the name attribute
					for j := 0; j < int(child.ChildCount()); j++ {
						attrChild := child.Child(uint(j))
						if attrChild.Kind() == "Name" && string(attrChild.Utf8Text(content)) == "name" {
							// Find the AttValue node
							for k := 0; k < int(child.ChildCount()); k++ {
								valueChild := child.Child(uint(k))
								if valueChild.Kind() == "AttValue" {
									// Remove quotes from attribute value
									value := string(valueChild.Utf8Text(content))
									return strings.Trim(value, "\"'")
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

// ParseSystemConfigField parses a system config field node and returns a SystemConfigField
func ParseSystemConfigField(node *tree_sitter.Node, content []byte, filePath string) SystemConfigField {
	field := SystemConfigField{
		FilePath: filePath,
	}

	// Get the node type (input-field or component)
	sTagNode := treesitterhelper.FindFirst(node, treesitterhelper.NodeKind("STag"), content)
	if sTagNode != nil {
		// Store the line number for LSP navigation
		field.Line = uint32(sTagNode.StartPosition().Row) + 1 // Line numbers start from 1

		// Check if it's an input-field
		inputFieldNode := treesitterhelper.FindFirst(sTagNode, treesitterhelper.And(
			treesitterhelper.NodeKind("Name"),
			treesitterhelper.NodeText("input-field"),
		), content)

		// Check if it's a component
		componentNode := treesitterhelper.FindFirst(sTagNode, treesitterhelper.And(
			treesitterhelper.NodeKind("Name"),
			treesitterhelper.NodeText("component"),
		), content)

		// Get field name
		field.Name = GetSystemConfigFieldName(node, content)

		// Get field label
		field.Label = GetSystemConfigFieldLabel(node, content)

		if inputFieldNode != nil {
			field.Type = GetSystemConfigFieldType(node, content)
		} else if componentNode != nil {
			field.Component = GetSystemConfigComponent(node, content)
		}
	}

	return field
}

// FindAllSystemConfigFields finds all system config fields in the given XML content
func FindAllSystemConfigFields(root *tree_sitter.Node, content []byte, filePath string) []SystemConfigField {
	// Find all input-field elements
	inputFieldNodes := treesitterhelper.FindAll(root, treesitterhelper.And(
		treesitterhelper.NodeKind("element"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("STag"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("input-field"),
			)),
		)),
	), content)

	// Find all component elements
	componentNodes := treesitterhelper.FindAll(root, treesitterhelper.And(
		treesitterhelper.NodeKind("element"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("STag"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("Name"),
				treesitterhelper.NodeText("component"),
			)),
		)),
	), content)

	// Combine all nodes
	allNodes := append(inputFieldNodes, componentNodes...)

	// Parse each node into a SystemConfigField
	fields := make([]SystemConfigField, 0, len(allNodes))
	for _, node := range allNodes {
		field := ParseSystemConfigField(node, content, filePath)
		if field.Name != "" {
			fields = append(fields, field)
		}
	}

	return fields
}
