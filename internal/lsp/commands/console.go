package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/shopware/shopware-lsp/internal/console"
	"github.com/shopware/shopware-lsp/internal/lsp"
)

type ConsoleCatalogProvider struct{ *console.CatalogProvider }

func NewConsoleCatalogProvider(index *console.Index, roots ...string) *ConsoleCatalogProvider {
	return &ConsoleCatalogProvider{console.NewCatalogProvider(index, roots...)}
}
func (p *ConsoleCatalogProvider) GetCommands(
	_ context.Context,
) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		console.ListCatalogCommand: p.list,
	}
}

func (p *ConsoleCatalogProvider) list(
	ctx context.Context,
	raw *json.RawMessage,
) (interface{}, error) {
	if p == nil || p.CatalogProvider == nil {
		return nil, fmt.Errorf("symfony console catalog is unavailable")
	}
	var request console.CatalogRequest
	if raw != nil && len(*raw) != 0 && string(*raw) != "null" {
		if err := json.Unmarshal(*raw, &request); err != nil {
			return nil, fmt.Errorf("invalid console catalog request: %w", err)
		}
	}
	return p.CatalogWithRequest(ctx, request)
}
