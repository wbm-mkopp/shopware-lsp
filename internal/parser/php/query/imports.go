package query

import (
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

// UseItem retains exact source ranges while providing normalized text for the
// central import resolver. Trivia never participates in import names.
type UseItem struct {
	Text      string
	Range     cst.TextRange
	NameRange cst.TextRange
}

type UseList struct {
	Items  []UseItem
	Commas []cst.TextRange
}

// UseItems accepts complete native import declarations only. Closure captures
// and trait uses have distinct syntax kinds and cannot reach this path.
func UseItems(node *syntax.Node) (UseList, bool) {
	if node == nil || node.Kind() != syntax.PhpUseDeclaration {
		return UseList{}, false
	}
	var tokens []*syntax.Token
	for token := range node.ChildTokens() {
		if !token.Kind().IsTrivia() {
			tokens = append(tokens, token)
		}
	}
	if len(tokens) < 3 || !strings.EqualFold(tokens[0].Text(), "use") || tokens[len(tokens)-1].Kind() != syntax.TkSemicolon {
		return UseList{}, false
	}
	tokens = tokens[1 : len(tokens)-1]
	defaultKind := ""
	if len(tokens) > 0 && importKind(tokens[0].Text()) != "" {
		defaultKind = importKind(tokens[0].Text())
		tokens = tokens[1:]
	}
	prefix := ""
	for i, token := range tokens {
		if token.Kind() != syntax.TkOpenBrace {
			continue
		}
		if len(tokens) < i+3 || tokens[len(tokens)-1].Kind() != syntax.TkCloseBrace {
			return UseList{}, false
		}
		var ok bool
		prefix, ok = importName(tokens[:i], true)
		if !ok {
			return UseList{}, false
		}
		tokens = tokens[i+1 : len(tokens)-1]
		break
	}
	var result UseList
	start := 0
	for i := 0; i <= len(tokens); i++ {
		if i < len(tokens) && tokens[i].Kind() != syntax.TkComma {
			continue
		}
		if i == start {
			if i == len(tokens) && prefix != "" && len(result.Items) > 0 {
				break
			}
			return UseList{}, false
		}
		item, ok := useItem(tokens[start:i], prefix, defaultKind)
		if !ok {
			return UseList{}, false
		}
		result.Items = append(result.Items, item)
		if i < len(tokens) {
			result.Commas = append(result.Commas, tokens[i].Range())
		}
		start = i + 1
	}
	return result, len(result.Items) > 0
}

func useItem(tokens []*syntax.Token, prefix, kind string) (UseItem, bool) {
	rng := cst.TextRange{Start: tokens[0].Range().Start, End: tokens[len(tokens)-1].Range().End}
	if importKind(tokens[0].Text()) != "" {
		if prefix == "" || kind != "" {
			return UseItem{}, false
		}
		kind = importKind(tokens[0].Text())
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return UseItem{}, false
	}
	nameTokens := tokens
	alias := ""
	for i, token := range tokens {
		if !strings.EqualFold(token.Text(), "as") {
			continue
		}
		if i+2 != len(tokens) || !importIdentifier(tokens[i+1]) {
			return UseItem{}, false
		}
		alias = tokens[i+1].Text()
		nameTokens = tokens[:i]
		break
	}
	name, ok := importName(nameTokens, false)
	if !ok {
		return UseItem{}, false
	}
	if prefix != "" && strings.HasPrefix(name, "\\") {
		return UseItem{}, false
	}
	text := "use "
	if kind != "" {
		text += kind + " "
	}
	text += prefix + name
	if alias != "" {
		text += " as " + alias
	}
	text += ";"
	return UseItem{Text: text, Range: rng, NameRange: cst.TextRange{Start: nameTokens[0].Range().Start, End: nameTokens[len(nameTokens)-1].Range().End}}, true
}

func importKind(text string) string {
	if strings.EqualFold(text, "function") {
		return "function"
	}
	if strings.EqualFold(text, "const") {
		return "const"
	}
	return ""
}
func importIdentifier(token *syntax.Token) bool {
	return (token.Kind() == syntax.TkIdentifier || token.Kind() == syntax.TkKeyword) && importKind(token.Text()) == "" && !strings.EqualFold(token.Text(), "as")
}
func importName(tokens []*syntax.Token, trailingSlash bool) (string, bool) {
	if len(tokens) == 0 {
		return "", false
	}
	var name strings.Builder
	wantName := true
	for i, token := range tokens {
		if token.Kind() == syntax.TkBackslash {
			if wantName && i != 0 {
				return "", false
			}
			wantName = true
		} else {
			if !wantName || !importIdentifier(token) {
				return "", false
			}
			wantName = false
		}
		name.WriteString(token.Text())
	}
	if wantName != trailingSlash {
		return "", false
	}
	return name.String(), true
}
