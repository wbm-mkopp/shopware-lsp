package lsp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDocumentColorProvider struct {
	colors        []protocol.ColorInformation
	presentations []protocol.ColorPresentation
	colorRequest  **DocumentColorRequest
	presentation  **ColorPresentationRequest
}

func (p testDocumentColorProvider) GetDocumentColors(
	_ context.Context,
	request *DocumentColorRequest,
) ([]protocol.ColorInformation, error) {
	if p.colorRequest != nil {
		*p.colorRequest = request
	}
	return p.colors, nil
}

func (p testDocumentColorProvider) GetColorPresentations(
	_ context.Context,
	request *ColorPresentationRequest,
) ([]protocol.ColorPresentation, error) {
	if p.presentation != nil {
		*p.presentation = request
	}
	return p.presentations, nil
}

func TestDocumentColorsAggregateDeduplicateValidateAndSort(t *testing.T) {
	server := NewServer(nil, "", "test")
	t.Cleanup(func() { require.NoError(t, server.CloseAll()) })
	uri := "file:///workspace/component.scss"
	server.documentManager.OpenDocument(uri, "$a: #000;\n$b: #fff;", 7)
	first := protocol.ColorInformation{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 4},
			End:   protocol.Position{Line: 0, Character: 8},
		},
		Color: protocol.Color{Alpha: 1},
	}
	second := protocol.ColorInformation{
		Range: protocol.Range{
			Start: protocol.Position{Line: 1, Character: 4},
			End:   protocol.Position{Line: 1, Character: 8},
		},
		Color: protocol.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}
	invalid := protocol.ColorInformation{
		Range: protocol.Range{
			Start: protocol.Position{Line: 2},
			End:   protocol.Position{Line: 1},
		},
	}
	server.RegisterDocumentColorProvider(testDocumentColorProvider{
		colors: []protocol.ColorInformation{second, invalid, first},
	})
	var received *DocumentColorRequest
	server.RegisterDocumentColorProvider(testDocumentColorProvider{
		colors: []protocol.ColorInformation{first}, colorRequest: &received,
	})

	colors, err := server.documentColors(context.Background(), &protocol.DocumentColorParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
	})
	require.NoError(t, err)
	require.Equal(t, []protocol.ColorInformation{first, second}, colors)
	require.NotNil(t, received)
	require.Equal(t, 7, received.Document.Version)
}

func TestColorPresentationsUseLiveDocumentAndDeduplicate(t *testing.T) {
	server := NewServer(nil, "", "test")
	t.Cleanup(func() { require.NoError(t, server.CloseAll()) })
	uri := "file:///workspace/component.scss"
	server.documentManager.OpenDocument(uri, "$a: #000;", 9)
	presentation := protocol.ColorPresentation{Label: "#ffffff"}
	var received *ColorPresentationRequest
	server.RegisterDocumentColorProvider(testDocumentColorProvider{
		presentations: []protocol.ColorPresentation{presentation}, presentation: &received,
	})
	server.RegisterDocumentColorProvider(testDocumentColorProvider{
		presentations: []protocol.ColorPresentation{presentation},
	})
	params := &protocol.ColorPresentationParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri},
		Color:        protocol.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}
	presentations, err := server.colorPresentations(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, []protocol.ColorPresentation{presentation}, presentations)
	require.NotNil(t, received)
	require.Same(t, params, received.ColorPresentationParams)
	require.Equal(t, 9, received.Document.Version)
}

// documentColor and colorPresentation are specified as arrays, with no null
// permitted, unlike most requests. A nil slice marshals to null, which strict
// clients reject outright: Zed logs "invalid type: null, expected a sequence"
// and drops the response.
func TestColorResponsesMarshalEmptyResultsAsArrays(t *testing.T) {
	server := NewServer(nil, "", "test")
	t.Cleanup(func() { require.NoError(t, server.CloseAll()) })
	uri := "file:///workspace/component.scss"
	server.documentManager.OpenDocument(uri, "$a: #000;", 1)
	absent := protocol.TextDocumentIdentifier{URI: "file:///workspace/absent.scss"}
	open := protocol.TextDocumentIdentifier{URI: uri}

	requireEmptyJSONArray := func(t *testing.T, result any, err error) {
		t.Helper()
		require.NoError(t, err)
		payload, marshalErr := json.Marshal(result)
		require.NoError(t, marshalErr)
		assert.Equal(t, "[]", string(payload))
	}

	t.Run("colors with no provider registered", func(t *testing.T) {
		result, err := server.documentColors(context.Background(),
			&protocol.DocumentColorParams{TextDocument: open})
		requireEmptyJSONArray(t, result, err)
	})

	t.Run("colors for a document that is not open", func(t *testing.T) {
		result, err := server.documentColors(context.Background(),
			&protocol.DocumentColorParams{TextDocument: absent})
		requireEmptyJSONArray(t, result, err)
	})

	t.Run("colors without params", func(t *testing.T) {
		result, err := server.documentColors(context.Background(), nil)
		requireEmptyJSONArray(t, result, err)
	})

	t.Run("presentations with no provider registered", func(t *testing.T) {
		result, err := server.colorPresentations(context.Background(),
			&protocol.ColorPresentationParams{TextDocument: open})
		requireEmptyJSONArray(t, result, err)
	})

	t.Run("presentations for a document that is not open", func(t *testing.T) {
		result, err := server.colorPresentations(context.Background(),
			&protocol.ColorPresentationParams{TextDocument: absent})
		requireEmptyJSONArray(t, result, err)
	})

	t.Run("presentations without params", func(t *testing.T) {
		result, err := server.colorPresentations(context.Background(), nil)
		requireEmptyJSONArray(t, result, err)
	})
}
