package symfony

import (
	"testing"

	"github.com/shopware/shopware-lsp/internal/php/semantic"
	"github.com/shopware/shopware-lsp/internal/php/types"
	"github.com/stretchr/testify/assert"
)

type stubConstantLookup map[string][]semantic.Symbol

func (s stubConstantLookup) Constants(className string) []semantic.Symbol {
	return s[className]
}

func literalConstant(name, value string) semantic.Symbol {
	return semantic.Symbol{
		Kind: semantic.ClassConstantSymbol,
		Name: name,
		Type: types.LiteralString(value),
	}
}

func TestResolveConstantRouteName(t *testing.T) {
	lookup := stubConstantLookup{
		"App\\Seo\\ProductPageSeoUrlRoute": {
			literalConstant("ROUTE_NAME", "frontend.detail.page"),
			literalConstant("OTHER", "frontend.other"),
		},
		"App\\Computed": {
			// A constant the inference could not narrow to a literal has no
			// name to match a route against.
			{Kind: semantic.ClassConstantSymbol, Name: "ROUTE_NAME", Type: types.String()},
		},
	}

	for name, testCase := range map[string]struct {
		reference string
		expected  string
	}{
		"resolves a literal string constant": {
			reference: "App\\Seo\\ProductPageSeoUrlRoute::ROUTE_NAME",
			expected:  "frontend.detail.page",
		},
		"picks the named constant, not the first": {
			reference: "App\\Seo\\ProductPageSeoUrlRoute::OTHER",
			expected:  "frontend.other",
		},
		"unknown class": {
			reference: "App\\Missing::ROUTE_NAME",
			expected:  "",
		},
		"unknown constant on a known class": {
			reference: "App\\Seo\\ProductPageSeoUrlRoute::ABSENT",
			expected:  "",
		},
		"non-literal constant": {
			reference: "App\\Computed::ROUTE_NAME",
			expected:  "",
		},
		"not a class constant reference": {
			reference: "frontend.detail.page",
			expected:  "",
		},
		"empty": {
			reference: "",
			expected:  "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t,
				testCase.expected,
				ResolveConstantRouteName(testCase.reference, lookup),
			)
		})
	}
}

func TestResolveConstantRouteNameWithoutLookup(t *testing.T) {
	// The CLI and tests can reach this without a PHP index.
	assert.Equal(t, "", ResolveConstantRouteName("App\\Foo::BAR", nil))
}

func TestResolveConstantRouteNames(t *testing.T) {
	lookup := stubConstantLookup{
		"App\\Seo\\ProductPageSeoUrlRoute": {
			literalConstant("ROUTE_NAME", "frontend.detail.page"),
		},
	}

	resolved := ResolveConstantRouteNames([]Route{
		{Name: "frontend.account.address.page", Path: "/account/address"},
		{NameConstant: "App\\Seo\\ProductPageSeoUrlRoute::ROUTE_NAME", Path: "/detail"},
		{NameConstant: "App\\Missing::ROUTE_NAME", Path: "/gone"},
		{Path: "/nameless"},
	}, lookup)

	// A literal name passes through, a resolvable constant gains one, and
	// anything still nameless is dropped rather than matching "".
	assert.Equal(t, []Route{
		{Name: "frontend.account.address.page", Path: "/account/address"},
		{
			Name:         "frontend.detail.page",
			NameConstant: "App\\Seo\\ProductPageSeoUrlRoute::ROUTE_NAME",
			Path:         "/detail",
		},
	}, resolved)
}
