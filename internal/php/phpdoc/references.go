package phpdoc

import "strings"

// VisitReferenceNames retains lexical PHPDoc names, including annotation
// classes and newer type syntax which the type algebra might not understand.
// Unknown tags are handled conservatively rather than deleting their imports.
func VisitReferenceNames(source string, visit func(string)) {
	lines := newLogicalLineScanner(source)
	for {
		line, ok := lines.next()
		if !ok {
			return
		}
		if !strings.HasPrefix(line, "@") {
			continue
		}
		tag, value := splitTag(line)
		annotation := strings.Contains(tag, "\\") || strings.Contains(tag, "(")
		switch tag {
		case "@param", "@phpstan-param", "@psalm-param", "@var", "@phpstan-var", "@psalm-var", "@property", "@property-read", "@property-write":
			value, _, _ = splitTypeAndVariable(value)
		case "@return", "@phpstan-return", "@psalm-return", "@throws", "@extends", "@implements", "@use", "@phpstan-extends", "@phpstan-implements":
			value, _ = splitTypeAndDescription(value)
		}
		visitDocNames(strings.TrimPrefix(tag, "@"), visit, annotation)
		visitDocNames(value, visit, annotation)
	}
}

func visitDocNames(text string, visit func(string), quotedNames bool) {
	for i := 0; i < len(text); {
		if text[i] == '\'' || text[i] == '"' {
			quote := text[i]
			i++
			start := i
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == quote {
					if quotedNames {
						visitDocNames(text[start:i], visit, false)
					}
					i++
					break
				}
				i++
			}
			continue
		}
		if text[i] == '$' {
			i++
			for i < len(text) && docNameByte(text[i]) {
				i++
			}
			continue
		}
		if !docNameByte(text[i]) || (text[i] >= '0' && text[i] <= '9') {
			i++
			continue
		}
		start := i
		for i < len(text) && docNameByte(text[i]) {
			i++
		}
		visit(text[start:i])
	}
}
func docNameByte(b byte) bool {
	return b == '\\' || b == '_' || b >= 128 || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
