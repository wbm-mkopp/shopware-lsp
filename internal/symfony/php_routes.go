package symfony

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// extractRoutes extracts routes from a tree-sitter node
func extractRoutes(filePath string, node *tree_sitter.Node, content []byte) []Route {
	var routes []Route

	// Define the pattern for finding Route attributes
	routeAttributePattern := treesitterhelper.And(
		treesitterhelper.NodeKind("attribute"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("name"),
			treesitterhelper.NodeText("Route"),
		)),
	)

	// Find all class declarations
	classPattern := treesitterhelper.NodeKind("class_declaration")
	classNodes := treesitterhelper.FindAll(node, classPattern, content)

	for _, classNode := range classNodes {
		// Get namespace from file
		namespace := extractNamespace(classNode.Parent(), content)

		// Get the class name
		className := extractClassName(classNode, content)

		// Get class-level Route attribute (if any) - but only to extract base path
		classRoutes := extractClassRoutes(classNode, content, namespace, className)

		// Get the base path from class route (if any) for method routes
		basePath := ""
		if len(classRoutes) > 0 {
			basePath = classRoutes[0].Path
		}

		// Find all method route attributes within the class
		methodRoutes := extractMethodRoutes(classNode, content, basePath)
		routes = append(routes, methodRoutes...)
	}

	// Find any top-level routes not associated with classes (fallback)
	attributeNodes := treesitterhelper.FindAll(node, routeAttributePattern, content)
	for _, attributeNode := range attributeNodes {
		// Only process attributes that are not part of a class
		if isTopLevelAttribute(attributeNode) {
			route := extractRouteFromAttribute(attributeNode, content)
			if route.Name != "" || route.Path != "" {
				routes = append(routes, route)
			}
		}
	}

	for i := range routes {
		routes[i].FilePath = filePath
	}

	return routes
}

// extractClassRoutes extracts routes from a class-level Route attribute
func extractClassRoutes(classNode *tree_sitter.Node, content []byte, namespace, className string) []Route {
	var routes []Route

	// Look for attribute_list in class
	attrListNode := treesitterhelper.GetFirstNodeOfKind(classNode, "attribute_list")
	if attrListNode == nil {
		return routes
	}

	// Find Route attribute
	routeAttributePattern := treesitterhelper.And(
		treesitterhelper.NodeKind("attribute"),
		treesitterhelper.HasChild(treesitterhelper.And(
			treesitterhelper.NodeKind("name"),
			treesitterhelper.NodeText("Route"),
		)),
	)

	attrNodes := treesitterhelper.FindAll(attrListNode, routeAttributePattern, content)
	if len(attrNodes) == 0 {
		return routes
	}

	// Extract route info from attribute
	for _, attrNode := range attrNodes {
		route := extractRouteFromAttribute(attrNode, content)

		// Set controller info if provided via "controller" param
		if route.Controller == "" && className != "" {
			// Build a default controller string with the class name
			if namespace != "" {
				route.Controller = namespace + "\\" + className
			} else {
				route.Controller = className
			}
		}

		if route.Name != "" || route.Path != "" {
			routes = append(routes, route)
		}
	}

	return routes
}

// extractMethodRoutes extracts routes from methods within a class
func extractMethodRoutes(classNode *tree_sitter.Node, content []byte, basePath string) []Route {
	var routes []Route

	// Get namespace from file
	namespace := extractNamespace(classNode.Parent(), content)

	// Get the class name
	className := extractClassName(classNode, content)

	// Get the declaration list (methods are inside here)
	declList := treesitterhelper.GetFirstNodeOfKind(classNode, "declaration_list")
	if declList == nil {
		return routes
	}

	// Define method pattern
	methodPattern := treesitterhelper.NodeKind("method_declaration")

	// Find all methods
	methodNodes := treesitterhelper.FindAll(declList, methodPattern, content)
	for _, methodNode := range methodNodes {
		// Get method name
		methodName := extractMethodName(methodNode, content)
		if methodName == "" {
			continue
		}

		// Get attribute list
		attrListNode := treesitterhelper.GetFirstNodeOfKind(methodNode, "attribute_list")
		if attrListNode == nil {
			continue
		}

		// Find Route attribute
		routeAttributePattern := treesitterhelper.And(
			treesitterhelper.NodeKind("attribute"),
			treesitterhelper.HasChild(treesitterhelper.And(
				treesitterhelper.NodeKind("name"),
				treesitterhelper.NodeText("Route"),
			)),
		)

		attrNodes := treesitterhelper.FindAll(attrListNode, routeAttributePattern, content)
		for _, attrNode := range attrNodes {
			route := extractRouteFromAttribute(attrNode, content)

			// If there's a base path and a route path, combine them
			if basePath != "" && route.Path != "" {
				// Ensure proper path combination
				if !strings.HasSuffix(basePath, "/") && !strings.HasPrefix(route.Path, "/") {
					route.Path = basePath + "/" + route.Path
				} else {
					route.Path = basePath + route.Path
				}
			}

			// Build controller string in format "Namespace\ClassName::methodName"
			controllerString := className + "::" + methodName
			if namespace != "" {
				controllerString = namespace + "\\" + controllerString
			}
			route.Controller = controllerString

			if route.Name != "" || route.Path != "" {
				routes = append(routes, route)
			}
		}
	}

	return routes
}

// isTopLevelAttribute checks if an attribute is defined outside a class (top-level)
func isTopLevelAttribute(node *tree_sitter.Node) bool {
	// Walk up the tree
	current := node
	for current != nil {
		if current.Kind() == "class_declaration" || current.Kind() == "method_declaration" {
			return false
		}
		current = current.Parent()
	}
	return true
}

// extractNamespace extracts the namespace from a PHP file
func extractNamespace(rootNode *tree_sitter.Node, content []byte) string {
	namespacePattern := treesitterhelper.And(
		treesitterhelper.NodeKind("namespace_definition"),
		treesitterhelper.HasChild(treesitterhelper.NodeKind("namespace_name")),
	)

	namespaceNodes := treesitterhelper.FindAll(rootNode, namespacePattern, content)
	if len(namespaceNodes) == 0 {
		return ""
	}

	// Get namespace name node
	namespaceNode := namespaceNodes[0]
	namespaceNameNode := treesitterhelper.GetFirstNodeOfKind(namespaceNode, "namespace_name")
	if namespaceNameNode == nil {
		return ""
	}

	return string(namespaceNameNode.Utf8Text(content))
}

// extractClassName extracts the name of a class
func extractClassName(classNode *tree_sitter.Node, content []byte) string {
	nameNode := treesitterhelper.GetFirstNodeOfKind(classNode, "name")
	if nameNode == nil {
		return ""
	}
	return string(nameNode.Utf8Text(content))
}

// extractMethodName extracts the name of a method
func extractMethodName(methodNode *tree_sitter.Node, content []byte) string {
	nameNode := treesitterhelper.GetFirstNodeOfKind(methodNode, "name")
	if nameNode == nil {
		return ""
	}
	return string(nameNode.Utf8Text(content))
}

// extractRouteFromAttribute extracts route data from an attribute node
func extractRouteFromAttribute(node *tree_sitter.Node, content []byte) Route {
	var route Route

	// Set the line number from the node's range
	route.Line = int(node.Range().StartPoint.Row) + 1

	// Find the arguments list
	argumentsList := treesitterhelper.GetFirstNodeOfKind(node, "arguments")
	if argumentsList == nil {
		return route
	}

	// Find named arguments in the list
	for i := 0; i < int(argumentsList.NamedChildCount()); i++ {
		argNode := argumentsList.NamedChild(uint(i))

		// Check if it's a named argument
		if argNode.Kind() == "argument" {
			nameNode := treesitterhelper.GetFirstNodeOfKind(argNode, "name")

			// Try to get a string node (encapsed_string in PHP tree-sitter)
			stringNode := treesitterhelper.GetFirstNodeOfKind(argNode, "encapsed_string")

			if nameNode != nil && stringNode != nil {
				argName := string(nameNode.Utf8Text(content))

				// Get string_content node inside the string node
				stringContentNode := treesitterhelper.GetFirstNodeOfKind(stringNode, "string_content")
				if stringContentNode != nil {
					stringValue := string(stringContentNode.Utf8Text(content))

					// Set the appropriate field based on argument name
					switch argName {
					case "name":
						route.Name = stringValue
					case "path":
						route.Path = stringValue
					case "controller":
						route.Controller = stringValue
					}
				}
			}
		}
	}

	return route
}
