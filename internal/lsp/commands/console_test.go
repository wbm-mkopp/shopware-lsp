package commands

import (
	"context"
	"encoding/json"
	"github.com/shopware/shopware-lsp/internal/console"
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleCatalogProvidesCommandsAliasesAndInputs(t *testing.T) {
	index, err := console.NewIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	path := "/project/src/Command/CreateUserCommand.php"
	require.NoError(t, index.Index(indexer.NewParsedFile(
		path,
		[]byte(`<?php
namespace App\Command;

#[AsCommand(
    name: 'app:user:create',
    description: 'Creates a user',
    aliases: ['app:user:add'],
)]
final class CreateUserCommand
{
    public function __invoke(
        #[Argument(description: 'User ID')] string $userId,
        #[Option(name: 'force', shortcut: 'f')] bool $force = false,
    ): int {
        return 0;
    }
}
`),
	)))

	provider := NewConsoleCatalogProvider(index, "/project")
	entries, err := provider.Catalog(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "app:user:add", entries[0].Name)
	assert.Equal(t, "app:user:create", entries[0].Canonical)
	assert.Equal(t, "app:user:create", entries[1].Name)
	assert.Equal(t, "Creates a user", entries[1].Description)
	assert.Equal(t, "App\\Command\\CreateUserCommand", entries[1].Class)
	assert.Equal(t, uriutil.FileURI(path), entries[1].FileURI)
	assert.Equal(
		t,
		"src/Command/CreateUserCommand.php",
		entries[1].FilePath,
	)
	assert.Equal(t, []console.CatalogInput{{
		Name:        "userId",
		Description: "User ID",
	}}, entries[1].Arguments)
	assert.Equal(t, []console.CatalogInput{{
		Name:     "force",
		Shortcut: "f",
		Default:  "false",
	}}, entries[1].Options)
}

func TestConsoleCatalogCommandFiltersCaseInsensitively(t *testing.T) {
	index, err := console.NewIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	require.NoError(t, index.Index(indexer.NewParsedFile(
		"/project/src/Commands.php",
		[]byte(`<?php
namespace App\Command;
#[AsCommand('app:user:create')]
final class CreateUser {}
#[AsCommand('app:cache:warm')]
final class WarmCache {}
`),
	)))
	provider := NewConsoleCatalogProvider(index, "/project")
	raw, err := json.Marshal(console.CatalogRequest{
		Query:    "USER",
		FileGlob: "src/**/Commands.php",
	})
	require.NoError(t, err)
	message := json.RawMessage(raw)
	value, err := provider.GetCommands(context.Background())[console.ListCatalogCommand](context.Background(), &message)
	require.NoError(t, err)
	assert.Equal(t, []console.CatalogEntry{{
		Name:      "app:user:create",
		Canonical: "app:user:create",
		Class:     "App\\Command\\CreateUser",
		FileURI:   "file:///project/src/Commands.php",
		FilePath:  "src/Commands.php",
	}}, value)

	entries, err := provider.CatalogWithRequest(
		context.Background(),
		console.CatalogRequest{FileGlob: "tests/**"},
	)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
