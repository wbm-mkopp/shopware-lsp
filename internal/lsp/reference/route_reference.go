package reference

import (
	"context"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type RouteReferenceProvider struct {
	routeIndex      *symfony.RouteIndexer
	routeUsageIndex *symfony.RouteUsageIndexer
}

func NewRouteReferenceProvider(lspServer *lsp.Server) *RouteReferenceProvider {
	routeIndex, _ := lspServer.GetIndexer("symfony.route")
	routeUsageIndex, _ := lspServer.GetIndexer("symfony.route_usage")
	return &RouteReferenceProvider{
		routeIndex:      routeIndex.(*symfony.RouteIndexer),
		routeUsageIndex: routeUsageIndex.(*symfony.RouteUsageIndexer),
	}
}

func (r *RouteReferenceProvider) GetReferences(ctx context.Context, params *protocol.ReferenceParams) []protocol.Location {
	if params.Node == nil {
		return nil
	}

	switch filepath.Ext(params.TextDocument.URI) {
	case ".php":
		return r.getReferencesForPHP(ctx, params)
	default:
		return nil
	}
}

func (r *RouteReferenceProvider) getReferencesForPHP(ctx context.Context, params *protocol.ReferenceParams) []protocol.Location {
	methodFQCN := treesitterhelper.GetMethodFQCN(params.Node, []byte(params.DocumentContent))

	if methodFQCN != "" {
		routes, _ := r.routeIndex.GetRoutes()

		route := routes.GetByController(methodFQCN)

		if route != nil {
			locations, _ := r.routeUsageIndex.GetRoute(route.Name)

			var result []protocol.Location

			for _, location := range locations {
				result = append(result, protocol.Location{
					URI: location.File,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      location.Line - 1,
							Character: 0,
						},
						End: protocol.Position{
							Line:      location.Line - 1,
							Character: 0,
						},
					},
				})
			}

			return result
		}
	}

	return nil
}
