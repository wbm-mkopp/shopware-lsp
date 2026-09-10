package commands

import (
	"context"
	"encoding/json"
	"github.com/shopware/shopware-lsp/internal/extension"

	"github.com/shopware/shopware-lsp/internal/lsp"
)

type ExtensionCommandProvider struct {
	extensionIndex *extension.ExtensionIndexer
}

func NewExtensionCommandProvider(extensionIndex *extension.ExtensionIndexer) *ExtensionCommandProvider {
	return &ExtensionCommandProvider{extensionIndex: extensionIndex}
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
