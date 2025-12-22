package twig

import (
	"bytes"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// TwigFunction represents a function defined in a Twig extension
type TwigFunction struct {
	// Name of the function as used in Twig templates
	Name string

	Usage string
	// Method name in the PHP class
	Method string
	// Line number where the function is defined
	Line int
	// Parameters required by the function
	Parameters []TwigParameter
	// FilePath is the path to the file where the TwigFunction is defined.
	FilePath string
}

// TwigFilter represents a filter defined in a Twig extension
type TwigFilter struct {
	// Name of the filter as used in Twig templates
	Name string

	Usage string
	// Method name in the PHP class
	Method string
	// Line number where the filter is defined
	Line int
	// Parameters required by the filter
	Parameters []TwigParameter
	// FilePath is the path to the file where the TwigFilter is defined.
	FilePath string
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
func ParseTwigExtension(filePath string, rootNode *tree_sitter.Node, content []byte) ([]TwigFunction, []TwigFilter, error) {
	if !bytes.Contains(content, []byte("AbstractExtension")) {
		return nil, nil, nil
	}

	if !bytes.Contains(content, []byte("TwigFunction")) && !bytes.Contains(content, []byte("TwigFilter")) {
		return nil, nil, nil
	}

	ctx := newParseContext(rootNode)
	if ctx.classNode == nil {
		return nil, nil, nil
	}

	// Check if the class extends AbstractExtension
	if !classExtendsAbstractExtension(ctx.classNode, content) {
		return nil, nil, nil
	}

	// Get the class name
	namespace := getNamespace(rootNode, content)
	className := getClassNameFromNode(ctx.classNode, namespace, content)
	if className == "" {
		return nil, nil, nil
	}

	var functions []TwigFunction
	var filters []TwigFilter

	// Parse functions and filters
	parseFunctionsDirectly(filePath, ctx, content, &functions)
	parseFiltersDirectly(filePath, ctx, content, &filters)

	// Parse parameters for functions
	for i, function := range functions {
		functions[i].Parameters = ctx.methodParameters(content, function.Method)

		functions[i].Usage = function.Name + "("

		for _, param := range functions[i].Parameters {
			functions[i].Usage += param.Name + ", "
		}

		if len(functions[i].Parameters) > 0 {
			functions[i].Usage = functions[i].Usage[:len(functions[i].Usage)-2]
		}

		functions[i].Usage += ")"
	}

	// Parse parameters for filters that are class methods
	for i, filter := range filters {
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

		filters[i].Parameters = ctx.methodParameters(content, methodName)

		filters[i].Usage = filter.Name + "("

		for _, param := range filters[i].Parameters {
			filters[i].Usage += param.Name + ", "
		}

		if len(filters[i].Parameters) > 0 {
			filters[i].Usage = filters[i].Usage[:len(filters[i].Usage)-2]
		}

		filters[i].Usage += ")"
	}

	return functions, filters, nil
}

type parseContext struct {
	classNode      *tree_sitter.Node
	declList       *tree_sitter.Node
	paramsByMethod map[string][]TwigParameter
}

func newParseContext(rootNode *tree_sitter.Node) *parseContext {
	classNode := findClassNode(rootNode)
	if classNode == nil {
		return &parseContext{}
	}

	var declList *tree_sitter.Node
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		child := classNode.NamedChild(uint(i))
		if child.Kind() == "declaration_list" {
			declList = child
			break
		}
	}

	return &parseContext{
		classNode: classNode,
		declList:  declList,
	}
}

// classExtendsAbstractExtension checks if a class extends AbstractExtension
func classExtendsAbstractExtension(classNode *tree_sitter.Node, content []byte) bool {
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

func getNamespace(rootNode *tree_sitter.Node, content []byte) string {
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

	return namespace
}

// Find the class name
func getClassNameFromNode(classNode *tree_sitter.Node, namespace string, content []byte) string {
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
func parseFunctionsDirectly(filePath string, ctx *parseContext, content []byte, functions *[]TwigFunction) {
	if ctx.classNode == nil || ctx.declList == nil {
		return
	}

	// Find the getFunctions method
	var funcMethod *tree_sitter.Node
	for i := 0; i < int(ctx.declList.NamedChildCount()); i++ {
		child := ctx.declList.NamedChild(uint(i))
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

							// Check for member call with spread operator: e.g., $this->test(...)
							if method == "" {
								// For spread operator expressions like $this->methodName(...),
								// extract the method name from the text directly as tree-sitter parsing is complex for this construct
								text := string(secondArg.Utf8Text(content))
								if strings.Contains(text, "$this->") && strings.Contains(text, "...") {
									parts := strings.Split(text, "$this->")
									if len(parts) > 1 {
										methodParts := strings.Split(parts[1], "(")
										if len(methodParts) > 0 {
											methodName := strings.TrimSpace(methodParts[0])
											if methodName != "" {
												method = "$this->" + methodName
											}
										}
									}
								}
							}
						}

						if name != "" && method != "" {
							lineNum := int(objNode.Range().StartPoint.Row) + 1
							*functions = append(*functions, TwigFunction{
								Name:     name,
								Method:   method,
								Line:     lineNum,
								FilePath: filePath,
							})
						}
					}
				}
			}
		}
	}
}

// Similar logic for filters
func parseFiltersDirectly(filePath string, ctx *parseContext, content []byte, filters *[]TwigFilter) {
	if ctx.classNode == nil || ctx.declList == nil {
		return
	}

	// Find the getFilters method
	var funcMethod *tree_sitter.Node
	for i := 0; i < int(ctx.declList.NamedChildCount()); i++ {
		child := ctx.declList.NamedChild(uint(i))
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

							// Check for member call with spread operator: e.g., $this->methodName(...)
							if method == "" {
								// For spread operator expressions like $this->methodName(...),
								// extract the method name from the text directly as tree-sitter parsing is complex for this construct
								text := string(secondArg.Utf8Text(content))
								if strings.Contains(text, "$this->") && strings.Contains(text, "...") {
									parts := strings.Split(text, "$this->")
									if len(parts) > 1 {
										methodParts := strings.Split(parts[1], "(")
										if len(methodParts) > 0 {
											methodName := strings.TrimSpace(methodParts[0])
											if methodName != "" {
												method = "$this->" + methodName
											}
										}
									}
								}
							}
						}

						if name != "" && method != "" {
							lineNum := int(objNode.Range().StartPoint.Row) + 1
							*filters = append(*filters, TwigFilter{
								Name:     name,
								Method:   method,
								Line:     lineNum,
								FilePath: filePath,
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

func (ctx *parseContext) methodParameters(content []byte, methodName string) []TwigParameter {
	if ctx.declList == nil {
		return []TwigParameter{}
	}

	if ctx.paramsByMethod == nil {
		ctx.paramsByMethod = buildMethodParameterMap(ctx.declList, content)
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

	if params, ok := ctx.paramsByMethod[searchMethodName]; ok {
		return params
	}

	return []TwigParameter{}
}

func buildMethodParameterMap(declList *tree_sitter.Node, content []byte) map[string][]TwigParameter {
	paramsByMethod := make(map[string][]TwigParameter)
	for i := 0; i < int(declList.NamedChildCount()); i++ {
		child := declList.NamedChild(uint(i))
		if child.Kind() != "method_declaration" {
			continue
		}

		var methodName string
		var paramsNode *tree_sitter.Node
		for j := 0; j < int(child.NamedChildCount()); j++ {
			subChild := child.NamedChild(uint(j))
			if subChild.Kind() == "name" {
				methodName = string(subChild.Utf8Text(content))
			} else if subChild.Kind() == "formal_parameters" {
				paramsNode = subChild
			}
		}

		if methodName == "" || paramsNode == nil {
			continue
		}

		paramsByMethod[methodName] = parseParameters(paramsNode, content)
	}

	return paramsByMethod
}

func parseParameters(paramsNode *tree_sitter.Node, content []byte) []TwigParameter {
	// Find the parameters
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
