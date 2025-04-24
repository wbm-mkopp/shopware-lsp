package symfony

import (
	"bytes"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseYAMLServices parses Symfony YAML service definitions and returns a list of services and parameters.
// It accepts a file path, root node from tree-sitter, and file content.
func ParseYAMLServices(path string, rootNode *tree_sitter.Node, data []byte) ([]Service, []Parameter, error) {
	// Pre-allocate with reasonable capacity
	services := make([]Service, 0, 50)
	parameters := make([]Parameter, 0, 20)

	// Handle different YAML node structures
	var documentNode *tree_sitter.Node

	// In YAML, the root node might be a "stream" with a "document" child
	if rootNode.Kind() == "stream" && rootNode.NamedChildCount() > 0 {
		// First child is the document
		documentNode = rootNode.NamedChild(0)
	} else if rootNode.Kind() == "document" {
		// Root node is already a document
		documentNode = rootNode
	} else {
		// Unrecognized structure
		return services, parameters, nil
	}

	// Get the root mapping
	if documentNode.NamedChildCount() == 0 {
		return services, parameters, nil
	}

	// Find the root mapping node - structure may vary based on YAML tree-sitter parsing
	rootMappingNode := documentNode.NamedChild(0)
	if rootMappingNode == nil {
		return services, parameters, nil
	}

	// Handle case where the first node is a "block_node" containing a "block_mapping"
	if rootMappingNode.Kind() == "block_node" && rootMappingNode.NamedChildCount() > 0 {
		// Use the first child which should be the block_mapping
		blockMapping := rootMappingNode.NamedChild(0)
		if blockMapping != nil && blockMapping.Kind() == "block_mapping" {
			rootMappingNode = blockMapping
		}
	}

	// Check that we have a block_mapping
	if rootMappingNode.Kind() != "block_mapping" {
		return services, parameters, nil
	}

	// Process all block_mapping_pair nodes to find services and parameters sections
	var servicesNode, parametersNode *tree_sitter.Node

	// Find services and parameters sections
	for i := 0; i < int(rootMappingNode.NamedChildCount()); i++ {
		pair := rootMappingNode.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		keyText := string(keyNode.Utf8Text(data))
		valueNode := pair.NamedChild(1)

		if keyText == "services" && valueNode != nil {
			servicesNode = valueNode
		} else if keyText == "parameters" && valueNode != nil {
			parametersNode = valueNode
		}
	}

	if servicesNode != nil {
		parsedServices := processYAMLServicesNode(servicesNode, data, path)
		services = append(services, parsedServices...)
	}

	if parametersNode != nil {
		parsedParams := processYAMLParametersNode(parametersNode, data, path)
		parameters = append(parameters, parsedParams...)
	}

	return services, parameters, nil
}

// processYAMLServicesNode extracts services from the services mapping in YAML
func processYAMLServicesNode(node *tree_sitter.Node, data []byte, path string) []Service {
	services := make([]Service, 0, 50)

	// Handle the case where the value is a block_node containing a block_mapping
	if node.Kind() == "block_node" && node.NamedChildCount() > 0 {
		// Try to get the block_mapping from the block_node
		blockMapping := node.NamedChild(0)
		if blockMapping != nil && blockMapping.Kind() == "block_mapping" {
			node = blockMapping
		}
	}

	// Skip if not a block_mapping
	if node == nil || node.Kind() != "block_mapping" {
		return services
	}

	// Process each service definition
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		// Get service ID (key)
		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		serviceID := string(keyNode.Utf8Text(data))

		// Skip services with special configurations that start with "_"
		if strings.HasPrefix(serviceID, "_") {
			continue
		}

		// Create service with default values
		service := Service{
			ID:   serviceID,
			Tags: make(map[string]string),
			Path: path,
			Line: 1 + bytes.Count(data[:keyNode.StartByte()], []byte{'\n'}),
		}

		// Get service definition (value)
		valueNode := pair.NamedChild(1)
		if valueNode == nil {
			// If no value node, use ID as class (this shouldn't happen in well-formed YAML)
			service.Class = serviceID
			services = append(services, service)
			continue
		}

		// Handle different types of service definitions
		if valueNode.Kind() == "block_mapping" {
			// Service with configuration
			processServiceConfig(&service, valueNode, data)
		} else if valueNode.Kind() == "block_node" && valueNode.NamedChildCount() > 0 {
			// Handle block_node with a block_mapping child
			blockMapping := valueNode.NamedChild(0)
			if blockMapping != nil && blockMapping.Kind() == "block_mapping" {
				processServiceConfig(&service, blockMapping, data)
			}
		} else if valueNode.Kind() == "flow_node" {
			// Simple string value - might be an alias
			aliasText := string(valueNode.Utf8Text(data))
			if strings.HasPrefix(aliasText, "@") || strings.HasPrefix(aliasText, "'@") || strings.HasPrefix(aliasText, "\"@") {
				// It's a service reference/alias
				service.AliasTarget = strings.Trim(strings.TrimPrefix(strings.TrimPrefix(aliasText, "'"), "\""), "@'\"")
			} else {
				// It's a class name
				service.Class = strings.Trim(aliasText, "'\"")
			}
		}

		// If service has no class or alias target, use ID as class (Symfony default behavior)
		if service.Class == "" && service.AliasTarget == "" {
			service.Class = service.ID
		}

		services = append(services, service)
	}

	return services
}

// processServiceConfig extracts configuration from a service definition
func processServiceConfig(service *Service, node *tree_sitter.Node, data []byte) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		// Get configuration key
		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		configKey := string(keyNode.Utf8Text(data))

		// Get configuration value
		valueNode := pair.NamedChild(1)
		if valueNode == nil {
			continue
		}

		switch configKey {
		case "class":
			if valueNode.Kind() == "flow_node" {
				service.Class = strings.Trim(string(valueNode.Utf8Text(data)), "'\"")
			}
		case "alias":
			if valueNode.Kind() == "flow_node" {
				service.AliasTarget = strings.Trim(string(valueNode.Utf8Text(data)), "'\"@")
			}
		case "tags":
			// Handle block_node containing tags
			if valueNode.Kind() == "block_node" && valueNode.NamedChildCount() > 0 {
				child := valueNode.NamedChild(0)
				if child != nil {
					processTags(service, child, data)
				}
			} else {
				// Process tags directly
				processTags(service, valueNode, data)
			}
		}
	}
}

// processTags extracts service tags from tags configuration
func processTags(service *Service, node *tree_sitter.Node, data []byte) {
	// Tags can be in different formats in YAML
	if node.Kind() == "block_sequence" {
		// Sequence of tags (most common)
		for i := 0; i < int(node.NamedChildCount()); i++ {
			item := node.NamedChild(uint(i))
			if item.Kind() != "block_sequence_item" {
				continue
			}

			valueNode := item.NamedChild(0)
			if valueNode == nil {
				continue
			}

			// Handle block_node (common in YAML tree-sitter parsing)
			if valueNode.Kind() == "block_node" && valueNode.NamedChildCount() > 0 {
				blockChild := valueNode.NamedChild(0)
				if blockChild != nil {
					if blockChild.Kind() == "block_mapping" {
						// Handle block mapping for tags with attributes
						processTagBlockMapping(service, blockChild, data)
					} else if blockChild.Kind() == "flow_mapping" {
						// Handle flow mapping for tags with attributes
						processTagFlowMapping(service, blockChild, data)
					}
				}
			} else if valueNode.Kind() == "flow_node" {
				// Simple tag name as string
				tag := strings.Trim(string(valueNode.Utf8Text(data)), "'\"")
				service.Tags[tag] = ""
			} else if valueNode.Kind() == "flow_mapping" {
				// Tag with attributes like { name: tag_name }
				processTagFlowMapping(service, valueNode, data)
			} else if valueNode.Kind() == "block_mapping" {
				// Tag with attributes as block mapping
				processTagBlockMapping(service, valueNode, data)
			}
		}
	} else if node.Kind() == "flow_sequence" {
		// Inline sequence of tags [tag1, tag2]
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if i%2 == 1 {
				// Skip commas
				continue
			}

			valueNode := node.NamedChild(uint(i))
			if valueNode == nil {
				continue
			}

			if valueNode.Kind() == "flow_node" {
				// Simple tag name
				tag := strings.Trim(string(valueNode.Utf8Text(data)), "'\"")
				service.Tags[tag] = ""
			} else if valueNode.Kind() == "flow_mapping" {
				// Tag with attributes
				processTagFlowMapping(service, valueNode, data)
			}
		}
	}
}

// processTagFlowMapping processes a tag with attributes in flow style { name: value }
func processTagFlowMapping(service *Service, node *tree_sitter.Node, data []byte) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if node.NamedChild(uint(i)).Kind() != "flow_pair" {
			continue
		}

		pair := node.NamedChild(uint(i))
		keyNode := pair.NamedChild(0)
		valueNode := pair.NamedChild(1)

		if keyNode == nil || valueNode == nil {
			continue
		}

		keyText := string(keyNode.Utf8Text(data))
		if keyText == "name" {
			tagName := strings.Trim(string(valueNode.Utf8Text(data)), "'\"")
			service.Tags[tagName] = ""
		}
	}
}

// processTagBlockMapping processes a tag with attributes in block style
func processTagBlockMapping(service *Service, node *tree_sitter.Node, data []byte) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		keyNode := pair.NamedChild(0)
		valueNode := pair.NamedChild(1)

		if keyNode == nil || valueNode == nil {
			continue
		}

		keyText := string(keyNode.Utf8Text(data))
		if keyText == "name" {
			tagName := strings.Trim(string(valueNode.Utf8Text(data)), "'\"")
			service.Tags[tagName] = ""
		}
	}
}

// processYAMLParametersNode extracts parameters from the parameters mapping in YAML
func processYAMLParametersNode(node *tree_sitter.Node, data []byte, path string) []Parameter {
	parameters := make([]Parameter, 0, 20)

	// Handle the case where the value is a block_node containing a block_mapping
	if node.Kind() == "block_node" && node.NamedChildCount() > 0 {
		// Try to get the block_mapping from the block_node
		blockMapping := node.NamedChild(0)
		if blockMapping != nil && blockMapping.Kind() == "block_mapping" {
			node = blockMapping
		}
	}

	// Skip if not a block_mapping
	if node == nil || node.Kind() != "block_mapping" {
		return parameters
	}

	// Process each parameter definition
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))
		if pair.Kind() != "block_mapping_pair" {
			continue
		}

		// Get parameter name (key)
		keyNode := pair.NamedChild(0)
		if keyNode == nil {
			continue
		}

		paramName := strings.Trim(string(keyNode.Utf8Text(data)), "'\"")

		// Create parameter with default values
		parameter := Parameter{
			Name: paramName,
			Path: path,
			Line: 1 + bytes.Count(data[:keyNode.StartByte()], []byte{'\n'}),
		}

		// Get parameter value
		valueNode := pair.NamedChild(1)
		if valueNode == nil {
			// No value defined (shouldn't happen in well-formed YAML)
			parameters = append(parameters, parameter)
			continue
		}

		// Process parameter value based on node kind
		if valueNode.Kind() == "flow_node" {
			paramValue := string(valueNode.Utf8Text(data))
			// Service reference
			if strings.HasPrefix(paramValue, "@") || strings.HasPrefix(paramValue, "'@") || strings.HasPrefix(paramValue, "\"@") {
				// Clean up service reference format - remove quotes
				parameter.Value = "@" + strings.Trim(strings.TrimPrefix(strings.TrimPrefix(paramValue, "'@"), "\"@"), "'\"")
			} else {
				// Regular value
				parameter.Value = strings.Trim(paramValue, "'\"")
			}
		} else if valueNode.Kind() == "block_scalar" {
			// Multiline value
			parameter.Value = string(valueNode.Utf8Text(data))
		}

		parameters = append(parameters, parameter)
	}

	return parameters
}
