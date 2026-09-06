package lsp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/stretchr/testify/require"
)

func TestRequestCancellationPreservesDocumentOrdering(t *testing.T) {
	server := NewServer(nil, t.TempDir(), "test")
	entered := make(chan struct{})
	server.commandMap["test/block"] = func(ctx context.Context, _ *json.RawMessage) (interface{}, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	server.commandMap["test/version"] = func(_ context.Context, _ *json.RawMessage) (interface{}, error) {
		doc, ok := server.documentManager.GetDocument("file:///test.php")
		if !ok {
			return -1, nil
		}
		return doc.Version, nil
	}
	client, _ := startDiagnosticLifecycleClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	// Supply a known request ID so the cancellation exercises the wire path.
	go func() { result <- client.Call(ctx, "test/block", nil, nil, jsonrpc2.PickID(jsonrpc2.ID{Num: 42})) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, client.Notify(ctx, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": "file:///test.php", "text": "<?php", "version": 7, "languageId": "php"}}))
	require.NoError(t, client.Notify(ctx, "$/cancelRequest", map[string]any{"id": 42}))
	select {
	case err := <-result:
		var rpcErr *jsonrpc2.Error
		require.ErrorAs(t, err, &rpcErr)
		require.Equal(t, requestCancelledCode, int(rpcErr.Code))
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var version int
	require.NoError(t, client.Call(ctx, "test/version", nil, &version))
	require.Equal(t, 7, version)
}

func TestDisconnectCancelsRunningRequestBeforeTeardown(t *testing.T) {
	server := NewServer(nil, t.TempDir(), "test")
	entered, finished := make(chan struct{}), make(chan struct{})
	server.commandMap["test/block"] = func(ctx context.Context, _ *json.RawMessage) (interface{}, error) {
		close(entered)
		<-ctx.Done()
		close(finished)
		return nil, ctx.Err()
	}
	client, _ := startDiagnosticLifecycleClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	returned := make(chan struct{})
	go func() { _ = client.Call(ctx, "test/block", nil, nil); close(returned) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, client.Close())
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("disconnect did not cancel the active request")
	}
	select {
	case <-returned:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
