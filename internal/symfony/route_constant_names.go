package symfony

import (
	"strings"

	"github.com/shopware/shopware-lsp/internal/php/semantic"
	"github.com/shopware/shopware-lsp/internal/php/types"
)

// ClassConstantLookup is the slice of the PHP index needed to resolve a route
// name written as a class constant. *php.PHPIndex satisfies it.
type ClassConstantLookup interface {
	Constants(className string) []semantic.Symbol
}

// ResolveConstantRouteName returns the literal route name behind a
// `Class::CONSTANT` reference, or "" when it cannot be resolved.
//
// Symfony accepts any constant expression for a route's name, and Shopware
// core uses that for most of its routes:
//
//	#[Route(name: ProductPageSeoUrlRoute::ROUTE_NAME, ...)]
//
// The constant lives in a different file from the controller, and indexers see
// one file at a time with no second pass, so the name cannot be resolved while
// indexing. It is resolved here instead, by callers that hold the PHP index.
func ResolveConstantRouteName(
	reference string,
	lookup ClassConstantLookup,
) string {
	if lookup == nil {
		return ""
	}
	className, constantName, found := strings.Cut(reference, "::")
	if !found || className == "" || constantName == "" {
		return ""
	}
	for _, constant := range lookup.Constants(className) {
		if constant.Name != constantName {
			continue
		}
		// Only a literal is usable. A constant computed at runtime, or one
		// whose type widened to plain string, has no name to match against.
		if constant.Type.Kind() != types.LiteralStringKind {
			return ""
		}
		return constant.Type.Name()
	}
	return ""
}

// ResolveConstantRouteNames fills in Name for every route that carries only a
// constant reference, dropping those that stay unresolved. Routes with a
// literal name pass through untouched.
func ResolveConstantRouteNames(
	routes []Route,
	lookup ClassConstantLookup,
) []Route {
	resolved := make([]Route, 0, len(routes))
	for _, route := range routes {
		if route.Name == "" && route.NameConstant != "" {
			route.Name = ResolveConstantRouteName(route.NameConstant, lookup)
		}
		if route.Name == "" {
			continue
		}
		resolved = append(resolved, route)
	}
	return resolved
}
