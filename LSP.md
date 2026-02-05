# Shopware LSP Custom Commands

This document lists the custom LSP commands and notifications provided by the Shopware Language Server. Each entry shows the method name, expected parameters and a short description of the action that is executed.

## Commands

### `shopware/forceReindex`
* **Parameters:** none
* **Action:** Forces a rebuild of all indexes by invoking `indexAll` with `forceReindex` set to `true`.
* **Returns:** `{ "message": "Force reindexing started" }`

### `shopware/extension/all`
* **Parameters:** none
* **Action:** Returns all detected Shopware extensions via the `ExtensionIndexer`.
* **Returns:** array of objects with `Name`, `Type` and `Path` fields.

### `shopware/snippet/storefront/getPossibleSnippetFiles`
* **Parameters:** `{ "fileUri": string }`
* **Action:** Searches the snippet directory for JSON files or creates a default `storefront.en-GB.json` if none exist.
* **Returns:** `{ "paths": [ { "path": string, "name": string, "value": string } ] }`

### `shopware/snippet/storefront/create`
* **Parameters:**
  ```json
  {
    "fileUri": string,
    "snippetKey": string,
    "snippets": [ { "path": string, "name": string, "value": string } ]
  }
  ```
* **Action:** Adds the provided snippet value to the given JSON files, reindexes them and publishes diagnostics for the original document.
* **Returns:** `null`

### `shopware/snippet/storefront/all`
* **Parameters:** none
* **Action:** Collects all storefront snippet keys from the indexed snippet files.
* **Returns:** array of objects `{ key, text, file }` sorted alphabetically.

### `shopware/snippet/admin/getPossibleSnippetFiles`
* **Parameters:** `{ "fileUri": string }`
* **Action:** Searches the administration snippet directory for JSON files or creates a default structure if none exist.
* **Returns:** `{ "paths": [ { "path": string, "name": string, "value": string } ] }`

### `shopware/snippet/admin/create`
* **Parameters:**
  ```json
  {
    "fileUri": string,
    "snippetKey": string,
    "snippets": [ { "path": string, "name": string, "value": string } ]
  }
  ```
* **Action:** Adds the provided snippet value to the given admin JSON files, reindexes them and publishes diagnostics for the original document.
* **Returns:** `null`

### `shopware/snippet/admin/all`
* **Parameters:** none
* **Action:** Collects all admin snippet keys from the indexed snippet files.
* **Returns:** array of objects `{ key, text, file }` sorted alphabetically.

### `shopware/twig/extendBlock`
* **Parameters:**
  ```json
  { "textUri": string, "blockName": string, "extension": string }
  ```
* **Action:** Creates or updates a Twig template in the selected extension so that it extends the given block. A new file is created if necessary and the block is inserted.
* **Returns:** on success `{ "uri": string, "line": number }`; otherwise an error object with `code` and `message`.

## Notifications

### `shopware/indexingStarted`
Sent when the server begins indexing. No parameters are required.

### `shopware/indexingCompleted`
Sent when indexing finishes. Parameters:
```json
{ "message": string, "timeInSeconds": number }
```

## Using with Neovim

The server produces a single binary. Build it using:

```bash
go build -o shopware-lsp
```

In Neovim, configure the language server using `lspconfig`:

```lua
require('lspconfig').shopware_lsp = {
  default_config = {
    cmd = { '/path/to/shopware-lsp' },
    filetypes = { 'php', 'twig', 'xml', 'yaml' },
    root_dir = vim.loop.cwd,
  },
}

require('lspconfig')['shopware_lsp'].setup{}
```

This registers the binary with Neovim’s built‑in LSP client and enables the custom commands above via `vim.lsp.buf.execute_command` or `client.request`.
