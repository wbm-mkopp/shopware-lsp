package twig

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TwigExtension represents a parsed Twig extension class
type TwigExtension struct {
	// Fully qualified class name
	ClassName string
	// Functions defined in the extension
	Functions []TwigFunction
	// Filters defined in the extension
	Filters []TwigFilter
}

// TwigFunction represents a function defined in a Twig extension
type TwigFunction struct {
	// Name of the function as used in Twig templates
	Name string
	// Method name in the PHP class
	Method string
	// Line number where the function is defined
	Line int
	// Parameters required by the function
	Parameters []TwigParameter
}

// TwigFilter represents a filter defined in a Twig extension
type TwigFilter struct {
	// Name of the filter as used in Twig templates
	Name string
	// Method name in the PHP class
	Method string
	// Line number where the filter is defined
	Line int
	// Parameters required by the filter
	Parameters []TwigParameter
}

// TwigParameter represents a parameter for a function or filter
type TwigParameter struct {
	// Parameter name including $ prefix
	Name string
	// Parameter type hint (string, int, etc.)
	Type string
	// Whether the parameter is optional (has a default value)
	Optional bool
}

// ParseTwigExtension parses a PHP file for Twig extension classes
func ParseTwigExtension(filePath string, rootNode *tree_sitter.Node, content []byte) (*TwigExtension, error) {
	// Check if the class extends AbstractExtension
	if !isAbstractExtension(rootNode, content) {
		return nil, nil
	}

	// Get the class name
	className := getClassName(rootNode, content)
	if className == "" {
		return nil, nil
	}

	extension := &TwigExtension{
		ClassName: className,
		Functions: []TwigFunction{},
		Filters:   []TwigFilter{},
	}

	// Parse functions and filters
	parseFunctionsDirectly(rootNode, content, extension)
	parseFiltersDirectly(rootNode, content, extension)

	// Parse parameters for functions
	for i, function := range extension.Functions {
		extension.Functions[i].Parameters = parseMethodParameters(rootNode, content, function.Method)
	}

	// Parse parameters for filters that are class methods
	for i, filter := range extension.Filters {
		// Skip PHP built-in functions
		if !strings.Contains(filter.Method, "::") && !strings.Contains(filter.Method, "->") {
			continue
		}
		
		methodName := filter.Method
		if strings.Contains(methodName, "::") {
			parts := strings.Split(methodName, "::")
			methodName = parts[1]
		} else if strings.Contains(methodName, "->") {
			parts := strings.Split(methodName, "->")
			methodName = parts[1]
		}
		
		extension.Filters[i].Parameters = parseMethodParameters(rootNode, content, methodName)
	}

	return extension, nil
}

// isAbstractExtension checks if a class extends AbstractExtension
func isAbstractExtension(rootNode *tree_sitter.Node, content []byte) bool {
	// Find the class declaration node
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return false
	}
	
	// Check if it extends AbstractExtension
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "base_clause" {
			baseText := string(child.Utf8Text(content))
			if strings.Contains(baseText, "AbstractExtension") {
				return true
			}
		}
	}
	
	return false
}

// Find the class name
func getClassName(rootNode *tree_sitter.Node, content []byte) string {
	// Find the namespace
	var namespace string
	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(uint(i))
		if child.Kind() == "namespace_definition" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				nsChild := child.NamedChild(uint(j))
				if nsChild.Kind() == "namespace_name" {
					namespace = string(nsChild.Utf8Text(content))
					break
				}
			}
		}
	}
	
	// Find the class name
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return ""
	}
	
	var className string
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "name" {
			className = string(child.Utf8Text(content))
			break
		}
	}
	
	if namespace != "" && className != "" {
		return namespace + "\\" + className
	}
	
	return className
}

// Direct traversal of the AST to find functions and filters
func parseFunctionsDirectly(rootNode *tree_sitter.Node, content []byte, extension *TwigExtension) {
	// Find the class
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return
	}
	
	// Find the declaration list
	var declList *tree_sitter.Node
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "declaration_list" {
			declList = child
			break
		}
	}
	
	if declList == nil {
		return
	}
	
	// Find the getFunctions method
	var funcMethod *tree_sitter.Node
	for i := 0; i < int(declList.NamedChildCount()); i++ {
		child := declList.NamedChild(uint(i))
		if child.Kind() == "method_declaration" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				nameNode := child.NamedChild(uint(j))
				if nameNode.Kind() == "name" {
					if string(nameNode.Utf8Text(content)) == "getFunctions" {
						funcMethod = child
						break
					}
				}
			}
			if funcMethod != nil {
				break
			}
		}
	}
	
	if funcMethod == nil {
		return
	}
	
	// Find the compound statement
	var compound *tree_sitter.Node
	for i := 0; i < int(funcMethod.NamedChildCount()); i++ {
		child := funcMethod.NamedChild(uint(i))
		if child.Kind() == "compound_statement" {
			compound = child
			break
		}
	}
	
	if compound == nil {
		return
	}
	
	// Find the return statement
	var returnStmt *tree_sitter.Node
	for i := 0; i < int(compound.NamedChildCount()); i++ {
		child := compound.NamedChild(uint(i))
		if child.Kind() == "return_statement" {
			returnStmt = child
			break
		}
	}
	
	if returnStmt == nil {
		return
	}
	
	// Find the array creation
	var arrayCreate *tree_sitter.Node
	for i := 0; i < int(returnStmt.NamedChildCount()); i++ {
		child := returnStmt.NamedChild(uint(i))
		if child.Kind() == "array_creation_expression" {
			arrayCreate = child
			break
		}
	}
	
	if arrayCreate == nil {
		return
	}
	
	// Process array elements
	for i := 0; i < int(arrayCreate.NamedChildCount()); i++ {
		element := arrayCreate.NamedChild(uint(i))
		
		if element.Kind() == "array_element_initializer" && element.NamedChildCount() > 0 {
			objNode := element.NamedChild(0)
			
			if objNode.Kind() == "object_creation_expression" {
				// Get class name
				var className string
				for j := 0; j < int(objNode.NamedChildCount()); j++ {
					child := objNode.NamedChild(uint(j))
					if child.Kind() == "name" {
						className = string(child.Utf8Text(content))
						break
					}
				}
				
				if className == "TwigFunction" {
					// Get arguments
					var argsNode *tree_sitter.Node
					for j := 0; j < int(objNode.NamedChildCount()); j++ {
						child := objNode.NamedChild(uint(j))
						if child.Kind() == "arguments" {
							argsNode = child
							break
						}
					}
					
					if argsNode != nil && argsNode.NamedChildCount() >= 2 {
						// First argument - name
						firstArg := argsNode.NamedChild(0)
						var name string
						if firstArg.Kind() == "argument" {
							stringNode := findNodeByKind(firstArg, "string")
							if stringNode != nil {
								contentNode := findNodeByKind(stringNode, "string_content")
								if contentNode != nil {
									name = string(contentNode.Utf8Text(content))
								}
							}
						}
						
						// Second argument - callback
						secondArg := argsNode.NamedChild(1)
						var method string
						if secondArg.Kind() == "argument" {
							// Check for string callback: e.g., 'abs'
							stringNode := findNodeByKind(secondArg, "string")
							if stringNode != nil {
								contentNode := findNodeByKind(stringNode, "string_content")
								if contentNode != nil {
									method = string(contentNode.Utf8Text(content))
								}
							}
							
							// Check for array callback: e.g., [$this, 'test']
							if method == "" {
								arrayNode := findNodeByKind(secondArg, "array_creation_expression")
								if arrayNode != nil && arrayNode.NamedChildCount() >= 2 {
									// Get first element (typically $this)
									firstElem := arrayNode.NamedChild(0)
									var thisRef string
									if firstElem.Kind() == "array_element_initializer" {
										varNameNode := findNodeByKind(firstElem, "variable_name")
										if varNameNode != nil {
											thisRef = string(varNameNode.Utf8Text(content))
										}
									}
									
									// Get second element (method name)
									secondElem := arrayNode.NamedChild(1)
									var methodName string
									if secondElem.Kind() == "array_element_initializer" {
										stringNode := findNodeByKind(secondElem, "string")
										if stringNode != nil {
											contentNode := findNodeByKind(stringNode, "string_content")
											if contentNode != nil {
												methodName = string(contentNode.Utf8Text(content))
											}
										}
									}
									
									if thisRef != "" && methodName != "" {
										method = thisRef + "->" + methodName
									} else if methodName != "" {
										method = methodName
									}
								}
							}
						}
						
						if name != "" && method != "" {
							lineNum := int(objNode.Range().StartPoint.Row) + 1
							extension.Functions = append(extension.Functions, TwigFunction{
								Name:   name,
								Method: method,
								Line:   lineNum,
							})
						} else {
						}
					}
				}
			}
		}
	}
}

// Similar logic for filters
func parseFiltersDirectly(rootNode *tree_sitter.Node, content []byte, extension *TwigExtension) {
	// Find the class
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return
	}
	
	// Find the declaration list
	var declList *tree_sitter.Node
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "declaration_list" {
			declList = child
			break
		}
	}
	
	if declList == nil {
		return
	}
	
	// Find the getFilters method
	var funcMethod *tree_sitter.Node
	for i := 0; i < int(declList.NamedChildCount()); i++ {
		child := declList.NamedChild(uint(i))
		if child.Kind() == "method_declaration" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				nameNode := child.NamedChild(uint(j))
				if nameNode.Kind() == "name" && string(nameNode.Utf8Text(content)) == "getFilters" {
					funcMethod = child
					break
				}
			}
			if funcMethod != nil {
				break
			}
		}
	}
	
	if funcMethod == nil {
		return
	}
	
	// Find the compound statement
	var compound *tree_sitter.Node
	for i := 0; i < int(funcMethod.NamedChildCount()); i++ {
		child := funcMethod.NamedChild(uint(i))
		if child.Kind() == "compound_statement" {
			compound = child
			break
		}
	}
	
	if compound == nil {
		return
	}
	
	// Find the return statement
	var returnStmt *tree_sitter.Node
	for i := 0; i < int(compound.NamedChildCount()); i++ {
		child := compound.NamedChild(uint(i))
		if child.Kind() == "return_statement" {
			returnStmt = child
			break
		}
	}
	
	if returnStmt == nil {
		return
	}
	
	// Find the array creation
	var arrayCreate *tree_sitter.Node
	for i := 0; i < int(returnStmt.NamedChildCount()); i++ {
		child := returnStmt.NamedChild(uint(i))
		if child.Kind() == "array_creation_expression" {
			arrayCreate = child
			break
		}
	}
	
	if arrayCreate == nil {
		return
	}
	
	// Process array elements
	for i := 0; i < int(arrayCreate.NamedChildCount()); i++ {
		element := arrayCreate.NamedChild(uint(i))
		
		if element.Kind() == "array_element_initializer" && element.NamedChildCount() > 0 {
			objNode := element.NamedChild(0)
			
			if objNode.Kind() == "object_creation_expression" {
				// Get class name
				var className string
				for j := 0; j < int(objNode.NamedChildCount()); j++ {
					child := objNode.NamedChild(uint(j))
					if child.Kind() == "name" {
						className = string(child.Utf8Text(content))
						break
					}
				}
				
				if className == "TwigFilter" {
					// Get arguments
					var argsNode *tree_sitter.Node
					for j := 0; j < int(objNode.NamedChildCount()); j++ {
						child := objNode.NamedChild(uint(j))
						if child.Kind() == "arguments" {
							argsNode = child
							break
						}
					}
					
					if argsNode != nil && argsNode.NamedChildCount() >= 2 {
						// First argument - name
						firstArg := argsNode.NamedChild(0)
						var name string
						if firstArg.Kind() == "argument" {
							stringNode := findNodeByKind(firstArg, "string")
							if stringNode != nil {
								contentNode := findNodeByKind(stringNode, "string_content")
								if contentNode != nil {
									name = string(contentNode.Utf8Text(content))
								}
							}
						}
						
						// Second argument - callback
						secondArg := argsNode.NamedChild(1)
						var method string
						if secondArg.Kind() == "argument" {
							// Check for string callback: e.g., 'abs'
							stringNode := findNodeByKind(secondArg, "string")
							if stringNode != nil {
								contentNode := findNodeByKind(stringNode, "string_content")
								if contentNode != nil {
									method = string(contentNode.Utf8Text(content))
								}
							}
							
							// Check for array callback: e.g., [$this, 'test']
							if method == "" {
								arrayNode := findNodeByKind(secondArg, "array_creation_expression")
								if arrayNode != nil && arrayNode.NamedChildCount() >= 2 {
									// Get first element (typically $this)
									firstElem := arrayNode.NamedChild(0)
									var thisRef string
									if firstElem.Kind() == "array_element_initializer" {
										varNameNode := findNodeByKind(firstElem, "variable_name")
										if varNameNode != nil {
											thisRef = string(varNameNode.Utf8Text(content))
										}
									}
									
									// Get second element (method name)
									secondElem := arrayNode.NamedChild(1)
									var methodName string
									if secondElem.Kind() == "array_element_initializer" {
										stringNode := findNodeByKind(secondElem, "string")
										if stringNode != nil {
											contentNode := findNodeByKind(stringNode, "string_content")
											if contentNode != nil {
												methodName = string(contentNode.Utf8Text(content))
											}
										}
									}
									
									if thisRef != "" && methodName != "" {
										method = thisRef + "->" + methodName
									} else if methodName != "" {
										method = methodName
									}
								}
							}
						}
						
						if name != "" && method != "" {
							lineNum := int(objNode.Range().StartPoint.Row) + 1
							extension.Filters = append(extension.Filters, TwigFilter{
								Name:   name,
								Method: method,
								Line:   lineNum,
							})
						}
					}
				}
			}
		}
	}
}

// findNodeByKind finds a child node of the given kind
func findNodeByKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		if child.Kind() == kind {
			return child
		}
	}
	return nil
}

// findClassNode finds the class declaration node
func findClassNode(rootNode *tree_sitter.Node) *tree_sitter.Node {
	for i := 0; i < int(rootNode.NamedChildCount()); i++ {
		child := rootNode.NamedChild(uint(i))
		if child.Kind() == "class_declaration" {
			return child
		}
		
		// Look in children too (for nested structures)
		if child.NamedChildCount() > 0 {
			classNode := findClassNodeInChildren(child)
			if classNode != nil {
				return classNode
			}
		}
	}
	
	return nil
}

// findClassNodeInChildren recursively searches for a class declaration
func findClassNodeInChildren(node *tree_sitter.Node) *tree_sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(uint(i))
		if child.Kind() == "class_declaration" {
			return child
		}
		
		if child.NamedChildCount() > 0 {
			classNode := findClassNodeInChildren(child)
			if classNode != nil {
				return classNode
			}
		}
	}
	
	return nil
}

// parseMethodParameters parses the parameters of a method
func parseMethodParameters(rootNode *tree_sitter.Node, content []byte, methodName string) []TwigParameter {
	// Find the class declaration
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return []TwigParameter{}
	}
	
	// Find the declaration list
	var declList *tree_sitter.Node
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "declaration_list" {
			declList = child
			break
		}
	}
	
	if declList == nil {
		return []TwigParameter{}
	}
	
	// Find the method
	// For $this->method format, extract just the method name
	searchMethodName := methodName
	if strings.Contains(methodName, "->") {
		parts := strings.Split(methodName, "->")
		if len(parts) == 2 {
			searchMethodName = parts[1]
		}
	}
	
	var methodNode *tree_sitter.Node
	for i := 0; i < int(declList.NamedChildCount()); i++ {
		child := declList.NamedChild(uint(i))
		if child.Kind() == "method_declaration" {
			// Check if this is the method we're looking for
			var nameNode *tree_sitter.Node
			for j := 0; j < int(child.NamedChildCount()); j++ {
				subChild := child.NamedChild(uint(j))
				if subChild.Kind() == "name" {
					nameNode = subChild
					break
				}
			}
			
			if nameNode != nil {
				foundName := string(nameNode.Utf8Text(content))
				if foundName == searchMethodName {
					methodNode = child
					break
				}
			}
		}
	}
	
	if methodNode == nil {
		return []TwigParameter{}
	}
	
	// Find the parameters
	var paramsNode *tree_sitter.Node
	for i := 0; i < int(methodNode.NamedChildCount()); i++ {
		child := methodNode.NamedChild(uint(i))
		if child.Kind() == "formal_parameters" {
			paramsNode = child
			break
		}
	}
	
	if paramsNode == nil {
		return []TwigParameter{}
	}
	
	// Process each parameter
	var params []TwigParameter
	for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
		child := paramsNode.NamedChild(uint(i))
		if child.Kind() == "simple_parameter" {
			param := TwigParameter{}
			
			// Get parameter name
			for j := 0; j < int(child.NamedChildCount()); j++ {
				subChild := child.NamedChild(uint(j))
				if subChild.Kind() == "variable_name" {
					param.Name = string(subChild.Utf8Text(content))
				} else if subChild.Kind() == "primitive_type" || subChild.Kind() == "union_type" || subChild.Kind() == "intersection_type" {
					param.Type = string(subChild.Utf8Text(content))
				}
			}
			
			// Check if the parameter is optional (has default value)
			for j := 0; j < int(child.NamedChildCount()); j++ {
				subChild := child.NamedChild(uint(j))
				if subChild.Kind() == "default_value" {
					param.Optional = true
					break
				}
			}
			
			if param.Name != "" {
				params = append(params, param)
			}
		}
	}
	
	return params
}