package symfony

import (
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/stretchr/testify/require"
)

func TestRouteIndexerFindsAbsoluteAndPartialRequestURLs(t *testing.T) {
	routeIndex, err := NewRouteIndexer(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, routeIndex.Close()) })
	require.NoError(t, routeIndex.Index(indexer.NewParsedFile(
		"/project/config/routes.yaml",
		[]byte(`product.show:
    path: /edit/{id}
car.show:
    path: /car/{edit}/foobar
static.show:
    path: /class-like-route
`),
	)))

	for searchPath, routeName := range map[string]string{
		"https://shop.example/edit/foo.bar?preview=1#details": "product.show",
		"https://shop.example/class-like-route":               "static.show",
		"ar/12/foo":                                           "car.show",
		"/edit/{id}":                                          "product.show",
	} {
		routes, findErr := routeIndex.FindRoutesByPath(searchPath)
		require.NoError(t, findErr)
		requireRouteNamed(t, routes, routeName)
	}

	routes, err := routeIndex.FindRoutesByPath("/edit/foo/bar")
	require.NoError(t, err)
	for _, route := range routes {
		require.NotEqual(t, "product.show", route.Name)
	}
}

func TestRouteIndexerRanksExactPathMatchesBeforePartialMatches(t *testing.T) {
	routeIndex, err := NewRouteIndexer(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, routeIndex.Close()) })
	require.NoError(t, routeIndex.Index(indexer.NewParsedFile(
		"/project/config/routes.yaml",
		[]byte(`partial:
    path: /store-api/sitemap/{filePath}
exact:
    path: /sitemap/{filePath}
`),
	)))

	routes, err := routeIndex.FindRoutesByPath("/sitemap/shop.xml.gz")
	require.NoError(t, err)
	require.Len(t, routes, 2)
	require.Equal(t, "exact", routes[0].Name)
	require.Equal(t, "partial", routes[1].Name)
}

func TestSortRoutesPrefersSourceDefinitionsOverGeneratedCatalogs(
	t *testing.T,
) {
	t.Parallel()

	routes := []Route{
		{
			Name:     "generated",
			Path:     "/sitemap/{filePath}",
			FilePath: "/project/var/cache/dev/url_generating_routes.php",
		},
		{
			Name:     "source",
			Path:     "/sitemap/{filePath}",
			FilePath: "/project/src/SitemapController.php",
		},
	}
	sortRoutes(routes)

	require.Equal(t, "source", routes[0].Name)
	require.Equal(t, "generated", routes[1].Name)
}

func TestRouteIndexerIndexesRoutingConfiguratorRoutes(t *testing.T) {
	routeIndex, err := NewRouteIndexer(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, routeIndex.Close()) })
	require.NoError(t, routeIndex.Index(indexer.NewParsedFile(
		"/project/config/routes.php",
		[]byte(`<?php
use Symfony\Component\Routing\Loader\Configurator\RoutingConfigurator;

return static function (RoutingConfigurator $routes): void {
    $routes->namePrefix('api_')->prefix('/api')
        ->add('product.show', '/products/{id}')
        ->controller([\App\Controller\ProductController::class, 'show'])
        ->methods(['GET', 'HEAD']);
};`),
	)))

	routes, err := routeIndex.GetRoute("api_product.show")
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.Equal(t, "/api/products/{id}", routes[0].Path)
	require.Equal(
		t,
		"App\\Controller\\ProductController::show",
		routes[0].Controller,
	)
	require.Equal(t, []string{"GET", "HEAD"}, routes[0].Methods)
	require.Equal(t, []string{"id"}, routes[0].Parameters())
}

func requireRouteNamed(t *testing.T, routes []Route, name string) {
	t.Helper()
	for _, route := range routes {
		if route.Name == name {
			return
		}
	}
	t.Fatalf("route %q not found in %#v", name, routes)
}

func TestRouteIndexerKeepsUnresolvedRoutesOutOfGetRoutes(t *testing.T) {
	routeIndex, err := NewRouteIndexer(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, routeIndex.Close()) })
	require.NoError(t, routeIndex.Index(indexer.NewParsedFile(
		"/project/src/Controller/ProductController.php",
		[]byte(`<?php
namespace App\Controller;
use App\Seo\ProductPageSeoUrlRoute;
class ProductController
{
    #[Route(path: '/detail/{productId}', name: ProductPageSeoUrlRoute::ROUTE_NAME)]
    public function detail(): void {}

    #[Route(path: '/address', name: 'frontend.account.address.page')]
    public function address(): void {}
}`),
	)))

	// A route whose name is still a constant would reach completion and
	// workspace symbols as a blank entry, so only the literal one is offered.
	routes, err := routeIndex.GetRoutes()
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.Equal(t, "frontend.account.address.page", routes[0].Name)

	resolved, err := routeIndex.ResolvedRoutes(stubConstantLookup{
		"App\\Seo\\ProductPageSeoUrlRoute": {
			literalConstant("ROUTE_NAME", "frontend.detail.page"),
		},
	})
	require.NoError(t, err)
	requireRouteNamed(t, resolved, "frontend.detail.page")
	requireRouteNamed(t, resolved, "frontend.account.address.page")

	// Without a lookup the constant stays unresolved and is dropped, rather
	// than being offered under an empty name.
	unresolved, err := routeIndex.ResolvedRoutes(nil)
	require.NoError(t, err)
	require.Len(t, unresolved, 1)
	require.Equal(t, "frontend.account.address.page", unresolved[0].Name)
}
