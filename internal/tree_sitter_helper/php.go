package treesitterhelper

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func IsStaticPHPMethodCall(className, methodName string) Pattern {

	return And(
		Or(
			NodeKind("string_content"),
			NodeKind("encapsed_string"),
			NodeKind("string"),
		),
		Ancestor(
			And(
				NodeKind("scoped_call_expression"),
				HasChild(
					And(
						NodeKind("name"),
						NodeText(methodName),
					),
				),
				HasChild(
					And(
						NodeKind("name"),
						NodeText(className),
					),
				),
			),
			4,
		),
	)
}

func IsPHPThisMethodCall(methodName string) Pattern {
	return And(
		Or(
			NodeKind("string_content"),
			NodeKind("encapsed_string"),
			NodeKind("string"),
		),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText(methodName),
				)),
			),
			4,
		),
	)
}

// IsThisMethodCall checks if the node represents a $this->method() call
func IsThisMethodCall(node *tree_sitter.Node, fileContent []byte) bool {
	// Check that this is a member call expression
	if node == nil || node.Kind() != "member_call_expression" {
		return false
	}

	// Find the object node (should be 'this')
	varNode := GetFirstNodeOfKind(node, "variable_name")
	if varNode == nil {
		return false
	}

	// Check if the variable is 'this'
	return string(varNode.Utf8Text(fileContent)) == "this"
}

// GetMethodNameFromThisCall extracts the method name from a $this->method() call
// Returns empty string if not a $this->method() call
func GetMethodNameFromThisCall(node *tree_sitter.Node, fileContent []byte) string {
	if !IsThisMethodCall(node, fileContent) {
		return ""
	}

	// Get the method name
	nameNode := GetFirstNodeOfKind(node, "name")
	if nameNode == nil {
		return ""
	}

	return string(nameNode.Utf8Text(fileContent))
}

func GetMethodFQCN(node *tree_sitter.Node, content []byte) string {
	nsCapture := Capture("namespace", NodeKind("namespace_name"))
	classNameCapture := Capture("class_name", NodeKind("name"))

	if !And(
		NodeKind("name"),
		Ancestor(
			And(
				NodeKind("method_declaration"),
				Ancestor(
					And(
						NodeKind("class_declaration"),
						HasChild(
							classNameCapture,
						),
						Ancestor(
							And(
								NodeKind("program"),
								HasChild(
									And(
										NodeKind("namespace_definition"),
										HasChild(
											nsCapture,
										),
									),
								),
							),
							20,
						),
					),
					5,
				),
			),
			1,
		),
	).Matches(node, content) {
		return ""
	}

	className := string(classNameCapture.GetCapturedNode().Utf8Text(content))
	ns := string(nsCapture.GetCapturedNode().Utf8Text(content))

	return fmt.Sprintf("%s\\%s::%s", ns, className, string(node.Utf8Text(content)))
}

func GetClassName(node *tree_sitter.Node, content []byte) string {
	nsCapture := Capture("namespace", NodeKind("namespace_name"))
	classNameCapture := Capture("class_name", NodeKind("name"))

	if !Ancestor(
		And(
			NodeKind("class_declaration"),
			HasChild(classNameCapture),
			Ancestor(
				And(
					NodeKind("program"),
					HasChild(
						And(
							NodeKind("namespace_definition"),
							HasChild(
								nsCapture,
							),
						),
					),
				),
				5,
			),
		),
		50,
	).Matches(node, content) {
		return ""
	}

	className := string(classNameCapture.GetCapturedNode().Utf8Text(content))
	ns := string(nsCapture.GetCapturedNode().Utf8Text(content))

	return fmt.Sprintf("%s\\%s", ns, className)
}
