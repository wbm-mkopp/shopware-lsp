package query

import (
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

// StringVariables reports lexical inputs of native PHP string tokens. Dynamic
// variable names and executable interpolation expressions remain unsupported.
func StringVariables(token *syntax.Token) ([]string, bool) {
	text := token.Text()
	if len(text) > 1 && (text[0] == 'b' || text[0] == 'B') && (text[1] == '\'' || text[1] == '"') {
		text = text[1:]
	}
	nowdoc := strings.HasPrefix(text, "<<<") && strings.HasPrefix(strings.TrimSpace(text[3:]), "'")
	if strings.HasPrefix(text, "'") || nowdoc {
		return nil, true
	}
	var names []string
	braced := false
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == '{' && i+1 < len(text) && text[i+1] == '$' {
			if braced {
				return nil, false
			}
			braced = true
		} else if braced && text[i] == '}' {
			braced = false
		} else if braced && (text[i] == '(' || text[i] == '{' || strings.HasPrefix(text[i:], "__FUNCTION__") || strings.HasPrefix(text[i:], "__METHOD__") || strings.HasPrefix(text[i:], "__LINE__")) {
			return nil, false
		}
		if text[i] != '$' {
			continue
		}
		start := i
		i++
		if i >= len(text) {
			break
		}
		if text[i] == '{' || text[i] == '$' {
			return nil, false
		}
		if !stringVariableStart(text[i]) {
			continue
		}
		for i < len(text) && (stringVariableStart(text[i]) || text[i] >= '0' && text[i] <= '9') {
			i++
		}
		names = append(names, text[start:i])
		i--
	}

	return names, true
}

func stringVariableStart(b byte) bool {
	return b == '_' || b >= 128 || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
