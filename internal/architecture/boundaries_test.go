package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchitectureBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	require.NoError(t, filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		tree, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		imports := map[string]string{}
		adapter := strings.HasPrefix(relative, "internal/lsp/") || strings.HasPrefix(relative, "internal/app/") || strings.HasPrefix(relative, "internal/cli/")
		for _, item := range tree.Imports {
			imported, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			alias := filepath.Base(imported)
			if item.Name != nil {
				alias = item.Name.Name
			}
			imports[alias] = imported
			if !adapter && strings.HasPrefix(imported, "github.com/shopware/shopware-lsp/internal/lsp") {
				// WorkspacePlan's existing protocol conversion is the sole infrastructure exception.
				if relative != "internal/rewrite/rewrite.go" || imported != "github.com/shopware/shopware-lsp/internal/lsp/protocol" {
					t.Errorf("%s: protocol adapter belongs in internal/lsp, not a domain package", relative)
				}
			}
			if imported == "github.com/fsnotify/fsnotify" && !strings.HasPrefix(relative, "internal/indexer/") {
				t.Errorf("%s: FileScanner must own filesystem events", relative)
			}
		}
		if strings.HasPrefix(relative, "internal/lsp/") {
			ast.Inspect(tree, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				name, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				if (strings.HasPrefix(relative, "internal/lsp/commands/") || strings.HasPrefix(relative, "internal/lsp/codeaction/") || strings.HasPrefix(relative, "internal/lsp/inspections/")) && imports[name.Name] == "os" {
					switch selector.Sel.Name {
					case "WriteFile", "Create", "MkdirAll", "Remove", "RemoveAll", "Rename":
						t.Errorf("%s: editing commands must return validated workspace edits", relative)
					}
				}
				if imports[name.Name] == "path/filepath" && (selector.Sel.Name == "Walk" || selector.Sel.Name == "WalkDir") {
					t.Errorf("%s: request adapters must query indexed paths", relative)
				}
				return true
			})
		}
		return nil
	}))
}
