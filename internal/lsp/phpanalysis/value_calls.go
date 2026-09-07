package phpanalysis

import (
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/php/semantic"
)

// CallReturnsValue uses linked reference targets rather than guessing whether
// a call can provide a reference to its caller.
func (s *State) CallReturnsValue(call *cst.Node) bool {
	var name *cst.Node
	for child := range call.ChildNodes() {
		if child.Kind() == syntax.PhpName {
			name = child
		}
		if child.Kind() == syntax.PhpArgumentList {
			break
		}
	}
	if name == nil {
		return false
	}
	for _, ref := range s.Document.References {
		if ref.Range.Start != name.RangeTrimmedTrivia().Start {
			continue
		}
		ids := ref.CandidateIDs()
		if ref.Resolved != "" {
			ids = append([]semantic.SymbolID{ref.Resolved}, ids...)
		}
		if len(ids) == 0 {
			return false
		}
		for _, id := range ids {
			symbol, ok := s.Snapshot.SymbolView(id)
			if !ok || (symbol.Kind() != semantic.MethodSymbol && symbol.Kind() != semantic.FunctionSymbol) || symbol.Materialize().Flags.Has(semantic.ByReferenceFlag) {
				return false
			}
		}
		return true
	}
	return false
}
