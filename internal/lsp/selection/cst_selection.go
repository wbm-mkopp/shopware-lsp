package selection

import (
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
)

func selectionRangeAt(
	root *cst.Node,
	lineIndex *cst.LineIndex,
	position protocol.Position,
) protocol.SelectionRange {
	offset := lineIndex.OffsetUTF16(
		uint32(max(position.Line, 0)),
		uint32(max(position.Character, 0)),
	)
	cursor := offset
	token := root.TokenAtOffset(offset)
	node := root.NodeAtOffset(offset)
	if token == nil && offset > root.Range().Start {
		offset--
		token = root.TokenAtOffset(offset)
		node = root.NodeAtOffset(offset)
	}
	var ranges []cst.TextRange
	add := func(candidate cst.TextRange, allowEmpty bool) {
		if candidate.Start > cursor || candidate.End < cursor || candidate.End < candidate.Start ||
			(!allowEmpty && candidate.End == candidate.Start) {
			return
		}
		if len(ranges) == 0 {
			ranges = append(ranges, candidate)
			return
		}
		child := ranges[len(ranges)-1]
		if candidate.Start > child.Start || candidate.End < child.End ||
			candidate == child {
			return
		}
		ranges = append(ranges, candidate)
	}
	if token != nil && !token.Kind().IsTrivia() {
		add(token.Range(), false)
	}
	for current := node; current != nil; current = current.Parent() {
		add(current.RangeTrimmedTrivia(), false)
	}
	add(root.Range(), true)
	if len(ranges) == 0 {
		ranges = append(ranges, root.Range())
	}
	var parent *protocol.SelectionRange
	for index := len(ranges) - 1; index >= 0; index-- {
		current := protocol.SelectionRange{
			Range:  selectionProtocolRange(ranges[index], lineIndex),
			Parent: parent,
		}
		parent = &current
	}
	return *parent
}

func selectionProtocolRange(
	rangeValue cst.TextRange,
	lineIndex *cst.LineIndex,
) protocol.Range {
	startLine, startCharacter := lineIndex.PositionUTF16(rangeValue.Start)
	endLine, endCharacter := lineIndex.PositionUTF16(rangeValue.End)
	return protocol.Range{
		Start: protocol.Position{
			Line: int(startLine), Character: int(startCharacter),
		},
		End: protocol.Position{
			Line: int(endLine), Character: int(endCharacter),
		},
	}
}
