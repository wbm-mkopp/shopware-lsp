package extension

import (
	"context"
	"encoding/json"

	"github.com/shopware/shopware-lsp/internal/lsp"
)

type ExtensionCommandProvider struct {
	extensionIndex *ExtensionIndexer
}

func NewExtensionCommandProvider(lsp *lsp.Server) *ExtensionCommandProvider {
	extensionIndex, _ := lsp.GetIndexer("extension.indexer")

	return &ExtensionCommandProvider{
		extensionIndex: extensionIndex.(*ExtensionIndexer),
	}
}
func (e *ExtensionCommandProvider) GetCommands(ctx context.Context) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		"shopware/extension/all": e.allExtensions,
	}
}

func (e *ExtensionCommandProvider) allExtensions(ctx context.Context, args *json.RawMessage) (interface{}, error) {
	extensions, err := e.extensionIndex.GetAll()
	if err != nil {
		return nil, err
	}

	return extensions, nil
}
