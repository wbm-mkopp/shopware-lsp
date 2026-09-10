package refactoring

import (
	"slices"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

func reindentExtraction(source string, rng cst.TextRange, from, to string, statements []*cst.Node) string {
	text, _ := reindentExtractionOffsets(source, rng, from, to, statements, nil)
	return text
}

// Offset mapping and literal preservation share a single pass. Return rewrites
// therefore do not repeatedly rebuild every preceding line in a large selection.
func reindentExtractionOffsets(source string, rng cst.TextRange, from, to string, statements []*cst.Node, positions []uint32) (string, map[uint32]uint32) {
	var literals []cst.TextRange
	for _, statement := range statements {
		for element := range statement.Descendants() {
			if token, ok := element.(*cst.Token); ok && token.Kind() == syntax.TkString {
				literals = append(literals, token.Range())
			}
		}
	}
	slices.Sort(positions)
	offsets := make(map[uint32]uint32, len(positions))
	var result strings.Builder
	result.Grow(int(rng.End - rng.Start))
	offset := rng.Start
	literalIndex, positionIndex := 0, 0
	for offset < rng.End {
		end := offset
		for end < rng.End && source[end] != '\n' {
			end++
		}
		for literalIndex < len(literals) && literals[literalIndex].End <= offset {
			literalIndex++
		}
		inside := literalIndex < len(literals) && offset > literals[literalIndex].Start && offset < literals[literalIndex].End
		line := source[offset:end]
		prefix := ""
		removed := 0
		if !inside {
			prefix = to
			if offset != rng.Start && strings.HasPrefix(line, from) {
				removed = len(from)
				line = line[removed:]
			}
		}
		for positionIndex < len(positions) && positions[positionIndex] <= end {
			pos := positions[positionIndex]
			if pos >= offset {
				offsets[pos] = uint32(result.Len()+len(prefix)) + uint32(max(0, int(pos-offset)-removed))
			}
			positionIndex++
		}
		result.WriteString(prefix)
		result.WriteString(line)
		if end < rng.End {
			result.WriteByte('\n')
			end++
		}
		offset = end
	}
	return result.String(), offsets
}
