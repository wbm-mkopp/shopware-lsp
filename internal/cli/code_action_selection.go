package cli

import (
	"fmt"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

func codeActionSelection(start protocol.Position, endLine, endColumn int) (protocol.Range, error) {
	if endLine < 0 || endColumn < 0 {
		return protocol.Range{}, fmt.Errorf("selection endpoints must be positive one-based positions")
	}
	end := start
	if endLine != 0 || endColumn != 0 {
		if endLine == 0 {
			endLine = start.Line + 1
		}
		if endColumn == 0 {
			endColumn = 1
		}
		end = protocol.Position{Line: endLine - 1, Character: endColumn - 1}
	}
	if end.Line < start.Line || end.Line == start.Line && end.Character < start.Character {
		return protocol.Range{}, fmt.Errorf("selection end precedes start")
	}
	return protocol.Range{Start: start, End: end}, nil
}
