package admin

import (
	"strings"

	jsquery "github.com/shopware/shopware-lsp/internal/parser/javascript/query"
	jssyntax "github.com/shopware/shopware-lsp/internal/parser/javascript/syntax"
)

// javaScriptLiteralSymbolAt recognizes declarations only after semantic
// references have been checked by JavaScriptSymbolAt.
func javaScriptLiteralSymbolAt(literal *jssyntax.Node) (AdminSymbolTarget, bool) {
	if literal == nil {
		return AdminSymbolTarget{}, false
	}
	call := jsquery.CallAt(literal)
	argument := jsquery.StringArgumentIndex(literal)
	if kind := javaScriptRegistrationArgumentKind(jsquery.CallName(literal), argument); kind != "" {
		return stringTarget(kind, literal)
	}
	if call != nil && argument == 0 {
		method := jsquery.CallMethodName(call)
		if method == "addServiceProvider" || method == "register" && strings.Contains(call.Text(), "Service()") {
			return stringTarget(AdminSymbolService, literal)
		}
	}
	return javaScriptRegistrationPropertySymbol(literal, call)
}

// Component.extend owns both the new name and its parent argument. All other
// supported registrations own only argument zero.
func javaScriptRegistrationArgumentKind(name string, argument int) AdminSymbolKind {
	if name == "Component.extend" || name == "Shopware.Component.extend" {
		if argument == 0 || argument == 1 {
			return AdminSymbolComponent
		}
		return ""
	}
	if argument != 0 {
		return ""
	}
	switch name {
	case "Component.register", "Shopware.Component.register", "Component.override", "Shopware.Component.override":
		return AdminSymbolComponent
	case "Mixin.register", "Shopware.Mixin.register":
		return AdminSymbolMixin
	case "Directive.register", "Shopware.Directive.register":
		return AdminSymbolDirective
	case "Filter.register", "Shopware.Filter.register":
		return AdminSymbolFilter
	case "Module.register", "Shopware.Module.register":
		return AdminSymbolModule
	case "Store.register", "Shopware.Store.register":
		return AdminSymbolStore
	default:
		return ""
	}
}

func javaScriptRegistrationPropertySymbol(literal, call *jssyntax.Node) (AdminSymbolTarget, bool) {
	if call == nil {
		return AdminSymbolTarget{}, false
	}
	name := jsquery.CallName(call)
	switch jsquery.PropertyName(jsquery.PropertyAt(literal)) {
	case "id":
		if name == "Shopware.Store.register" || name == "Store.register" {
			return stringTarget(AdminSymbolStore, literal)
		}
	case "component":
		if name == "Module.register" || name == "Shopware.Module.register" {
			return stringTarget(AdminSymbolComponent, literal)
		}
	}
	return AdminSymbolTarget{}, false
}
