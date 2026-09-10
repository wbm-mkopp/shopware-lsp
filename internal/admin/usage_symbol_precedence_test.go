package admin

import (
	"strings"
	"testing"

	javascriptparser "github.com/shopware/shopware-lsp/internal/parser/javascript"
	"github.com/stretchr/testify/require"
)

func TestJavaScriptSymbolRegistrationContexts(t *testing.T) {
	for _, test := range []struct {
		source, needle string
		kind           AdminSymbolKind
		name           string
	}{
		{`Shopware.Component.extend('child', 'parent', {})`, "child", AdminSymbolComponent, "child"},
		{`Shopware.Component.extend('child', 'parent', {})`, "parent", AdminSymbolComponent, "parent"},
		{`Shopware.Component.register('card', { template: 'markup' })`, "card", AdminSymbolComponent, "card"},
		{`Shopware.Component.register('card', { template: 'markup' })`, "markup", "", ""},
		{`Shopware.Component.override('card', {})`, "card", AdminSymbolComponent, "card"},
		{`Mixin.register('listing', {})`, "listing", AdminSymbolMixin, "listing"},
		{`Directive.register('tooltip', {})`, "tooltip", AdminSymbolDirective, "tooltip"},
		{`Filter.register('currency', {})`, "currency", AdminSymbolFilter, "currency"},
		{`Module.register('product', { routes: { detail: { component: 'card' } } })`, "card", AdminSymbolComponent, "card"},
		{`Module.register('product', { routes: { detail: { component: 'card' } } })`, "product", AdminSymbolModule, "product"},
		{`Shopware.Store.register({ id: 'session' })`, "session", AdminSymbolStore, "session"},
		{`Shopware.Service().register('apiService', () => {})`, "apiService", AdminSymbolService, "apiService"},
		{`Shopware.Application.addServiceProvider('acl', factory)`, "acl", AdminSymbolService, "acl"},
		{`Shopware.Component.register('card', { component: 'ordinary' })`, "ordinary", "", ""},
		{`Shopware.Component.register('card', 'ordinary')`, "ordinary", "", ""},
		{`Shopware.Component.register('card', { nested:`, "card", AdminSymbolComponent, "card"},
	} {
		t.Run(test.source+"/"+test.needle, func(t *testing.T) {
			parsed := javascriptparser.Parse(test.source)
			require.Equal(t, test.source, parsed.Tree.Root.Text())
			offset := strings.Index(test.source, "'"+test.needle+"'") + 1
			require.Positive(t, offset)
			target, found := JavaScriptSymbolAt(parsed.Tree.Root.NodeAtOffset(uint32(offset)))
			require.Equal(t, test.kind != "", found)
			if found {
				require.Equal(t, test.kind, target.Kind)
				require.Equal(t, test.name, target.Name)
			}
		})
	}
	target, found := JavaScriptSymbolAt(nil)
	require.False(t, found)
	require.Equal(t, AdminSymbolTarget{}, target)
}

func BenchmarkJavaScriptSymbolRecognition(b *testing.B) {
	source := `Shopware.Module.register('product', { routes: { detail: { component: 'sw-product-detail' } } });`
	node := javascriptparser.Parse(source).Tree.Root.NodeAtOffset(uint32(strings.Index(source, "sw-product-detail")))
	b.ReportAllocs()
	for b.Loop() {
		target, found := JavaScriptSymbolAt(node)
		if !found || target.Name != "sw-product-detail" {
			b.Fatal("missing component target")
		}
	}
}
