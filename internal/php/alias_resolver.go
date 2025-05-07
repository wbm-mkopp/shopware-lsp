package php

import (
	"strings"
)

// AliasResolver handles the resolution of PHP type aliases to their fully qualified class names (FQCN).
// It provides methods to resolve PHP types based on namespace, use statements, and aliases.
type AliasResolver struct {
	// Map of alias name to fully qualified class name
	aliases map[string]string
	// Map of class name to fully qualified class name
	useStatements map[string]string
	// Current namespace
	currentNamespace string
}

// NewAliasResolver creates a new alias resolver with the given namespace, use statements, and aliases.
//
// Parameters:
//   - namespace: The current PHP namespace (e.g., "Shopware\Core\Content\Product")
//   - useStatements: Map of class name to fully qualified class name from PHP use statements
//   - aliases: Map of alias name to fully qualified class name from PHP use statements with aliases
//
// Returns:
//   - A new AliasResolver instance configured with the provided parameters
func NewAliasResolver(namespace string, useStatements, aliases map[string]string) *AliasResolver {
	return &AliasResolver{
		aliases:          aliases,
		useStatements:    useStatements,
		currentNamespace: namespace,
	}
}

// ResolveType resolves a PHP type name to its fully qualified class name (FQCN).
// It handles various PHP type resolution scenarios including:
// - Primitive types (string, int, etc.)
// - Special types (self, static, etc.)
// - Fully qualified names (already containing namespace separators)
// - Aliased types (from "use X as Y" statements)
// - Imported types (from "use X" statements)
// - Types in the current namespace
//
// Parameters:
//   - typeName: The PHP type name to resolve
//
// Returns:
//   - The fully qualified class name (FQCN) for the given type
func (r *AliasResolver) ResolveType(typeName string) string {
	// Skip resolution for primitive types and special types
	if isPrimitiveType(typeName) || isSpecialType(typeName) {
		return typeName
	}

	// Check if the type contains a namespace separator
	if strings.Contains(typeName, "\\") {
		// If it's already a fully qualified name, return it as is
		return typeName
	}

	// First check if the type is an alias
	if fqcn, ok := r.aliases[typeName]; ok {
		return fqcn
	}

	// Then check if it's a use statement
	if fqcn, ok := r.useStatements[typeName]; ok {
		return fqcn
	}

	// If not found in aliases or use statements, assume it's in the current namespace
	if r.currentNamespace != "" {
		fqcn := r.currentNamespace + "\\" + typeName
		return fqcn
	}

	// If no namespace, return the type name as is
	return typeName
}

// isPrimitiveType checks if the given type is a PHP primitive type.
// PHP primitive types don't need to be resolved to FQCNs.
//
// Parameters:
//   - typeName: The PHP type name to check
//
// Returns:
//   - true if the type is a PHP primitive type, false otherwise
func isPrimitiveType(typeName string) bool {
	switch typeName {
	case "string", "int", "integer", "float", "double", "bool", "boolean",
		"array", "object", "callable", "iterable", "void", "null",
		"mixed", "never", "resource", "false", "true", "number":
		return true
	default:
		return false
	}
}

// isSpecialType checks if the given type is a PHP special type.
// PHP special types are keywords that refer to the current class context.
//
// Parameters:
//   - typeName: The PHP type name to check
//
// Returns:
//   - true if the type is a PHP special type, false otherwise
func isSpecialType(typeName string) bool {
	switch typeName {
	case "self", "static", "parent", "$this", "class-string", "array-key":
		return true
	default:
		return false
	}
}
