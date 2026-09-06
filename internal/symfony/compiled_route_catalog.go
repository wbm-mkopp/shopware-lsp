package symfony

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type CompiledRouteCatalog struct {
	projectRoot string

	mu       sync.RWMutex
	path     string
	routes   map[string]Route
	revision uint64
}

func NewCompiledRouteCatalog(
	projectRoot string,
) (*CompiledRouteCatalog, error) {
	result := &CompiledRouteCatalog{
		projectRoot: projectRoot,
		routes:      make(map[string]Route),
	}

	return result, nil
}

func (w *CompiledRouteCatalog) Refresh() error {
	if w == nil {
		return nil
	}
	path, err := w.findRouteFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.publish("", nil)
		}
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	routes := ParseCompiledRoutes(path, content)
	w.publish(path, routes)
	return nil
}

func (w *CompiledRouteCatalog) Routes() ([]Route, uint64) {
	if w == nil {
		return nil, 0
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]Route, 0, len(w.routes))
	for _, route := range w.routes {
		result = append(result, route)
	}
	sort.Slice(result, func(left, right int) bool {
		if result[left].Name != result[right].Name {
			return result[left].Name < result[right].Name
		}
		return result[left].FilePath < result[right].FilePath
	})
	return result, w.revision
}

func (w *CompiledRouteCatalog) Route(name string) (Route, bool) {
	if w == nil || name == "" {
		return Route{}, false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	route, found := w.routes[name]
	return route, found
}

func (w *CompiledRouteCatalog) Close() error { return nil }

func (w *CompiledRouteCatalog) publish(
	path string,
	routes []Route,
) {
	next := make(map[string]Route, len(routes))
	for _, route := range routes {
		if route.Name != "" {
			next[route.Name] = route
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.path = path
	w.routes = next
	w.revision++
}

func (w *CompiledRouteCatalog) findRouteFile() (string, error) {
	type candidate struct {
		path           string
		environmentDev bool
		modern         bool
		modified       time.Time
	}
	var candidates []candidate
	for _, cacheDir := range w.cacheDirectories() {
		environmentEntries, err := os.ReadDir(cacheDir)
		if err != nil {
			continue
		}
		for _, environment := range environmentEntries {
			if !environment.IsDir() {
				continue
			}
			environmentDir := filepath.Join(cacheDir, environment.Name())
			files, readErr := os.ReadDir(environmentDir)
			if readErr != nil {
				continue
			}
			for _, file := range files {
				if file.IsDir() || !isCompiledRouteFileName(file.Name()) {
					continue
				}
				info, infoErr := file.Info()
				if infoErr != nil {
					continue
				}
				candidates = append(candidates, candidate{
					path: filepath.Join(
						environmentDir,
						file.Name(),
					),
					environmentDev: strings.HasPrefix(
						strings.ToLower(environment.Name()),
						"dev",
					),
					modern:   file.Name() == "url_generating_routes.php",
					modified: info.ModTime(),
				})
			}
		}
	}
	if len(candidates) == 0 {
		return "", os.ErrNotExist
	}
	sort.Slice(candidates, func(left, right int) bool {
		if candidates[left].environmentDev !=
			candidates[right].environmentDev {
			return candidates[left].environmentDev
		}
		if candidates[left].modern != candidates[right].modern {
			return candidates[left].modern
		}
		if !candidates[left].modified.Equal(candidates[right].modified) {
			return candidates[left].modified.After(
				candidates[right].modified,
			)
		}
		return candidates[left].path < candidates[right].path
	})
	return candidates[0].path, nil
}

func (w *CompiledRouteCatalog) cacheDirectories() []string {
	return []string{
		filepath.Join(w.projectRoot, "var", "cache"),
		filepath.Join(w.projectRoot, "app", "cache"),
	}
}

func isCompiledRouteFileName(name string) bool {
	return name == "url_generating_routes.php" ||
		name == "UrlGenerator.php" ||
		strings.HasSuffix(name, "UrlGenerator.php")
}

func (w *CompiledRouteCatalog) shouldWatchCreatedDirectory(
	path string,
) bool {
	path = filepath.Clean(path)
	for _, cacheDir := range w.cacheDirectories() {
		parent := filepath.Dir(cacheDir)
		if path == parent ||
			path == cacheDir ||
			filepath.Dir(path) == cacheDir {
			return true
		}
	}
	return false
}
