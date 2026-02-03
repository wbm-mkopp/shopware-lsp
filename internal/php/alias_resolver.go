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
	// Cache for resolved types to avoid repeated string concatenation
	resolveCache map[string]string
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
		resolveCache:     make(map[string]string, 16),
	}
}

// Reset clears the resolver state for reuse (e.g., when pooling).
// This must be called before reusing a resolver with different namespace/imports.
func (r *AliasResolver) Reset(namespace string, useStatements, aliases map[string]string) {
	r.currentNamespace = namespace
	r.useStatements = useStatements
	r.aliases = aliases
	// Clear the cache to avoid stale entries from previous use
	for k := range r.resolveCache {
		delete(r.resolveCache, k)
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

	// Check the cache first to avoid repeated lookups and string concatenation
	if cached, ok := r.resolveCache[typeName]; ok {
		return cached
	}

	// First check if the type is an alias
	if fqcn, ok := r.aliases[typeName]; ok {
		r.resolveCache[typeName] = fqcn
		return fqcn
	}

	// Then check if it's a use statement
	if fqcn, ok := r.useStatements[typeName]; ok {
		r.resolveCache[typeName] = fqcn
		return fqcn
	}

	// If not found in aliases or use statements, assume it's in the current namespace
	if r.currentNamespace != "" {
		fqcn := r.currentNamespace + "\\" + typeName
		r.resolveCache[typeName] = fqcn
		return fqcn
	}

	// If no namespace, return the type name as is
	r.resolveCache[typeName] = typeName
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
