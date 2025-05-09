package treesitterhelper

import (
	"bytes"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// GetTextForRange extracts text from the document content for the given range
func GetTextForRange(content []byte, rng protocol.Range) string {
	if len(content) == 0 {
		return ""
	}

	lines := bytes.Split(content, []byte("\n"))
	if len(lines) == 0 || int(rng.Start.Line) >= len(lines) || int(rng.End.Line) >= len(lines) {
		return ""
	}

	if rng.Start.Line == rng.End.Line {
		// Selection is on a single line
		line := lines[rng.Start.Line]
		if int(rng.Start.Character) >= len(line) || int(rng.End.Character) > len(line) {
			return ""
		}
		return string(line[rng.Start.Character:rng.End.Character])
	}

	// Selection spans multiple lines
	var result []string

	// First line from start character to end of line
	firstLine := lines[rng.Start.Line]
	if int(rng.Start.Character) < len(firstLine) {
		result = append(result, string(firstLine[rng.Start.Character:]))
	}

	// Middle lines (if any) in full
	for i := rng.Start.Line + 1; i < rng.End.Line; i++ {
		result = append(result, string(lines[i]))
	}

	// Last line from start of line to end character
	lastLine := lines[rng.End.Line]
	if int(rng.End.Character) <= len(lastLine) {
		result = append(result, string(lastLine[:rng.End.Character]))
	}

	return strings.Join(result, "\n")
}
