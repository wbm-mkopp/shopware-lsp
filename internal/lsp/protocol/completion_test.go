package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An empty documentation object is not a valid Documentation value: kind is a
// MarkupKind, so "" matches neither permitted variant. Clients that decode
// into a typed union reject the enclosing response, discarding every item in
// the list rather than just the documentation of one.
func TestCompletionItemOmitsUnsetDocumentation(t *testing.T) {
	payload, err := json.Marshal(CompletionItem{
		Label: "BREADCRUMB_REWORK",
		Kind:  int(FunctionCompletion),
	})
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "documentation")
	assert.JSONEq(t, `{"label":"BREADCRUMB_REWORK","kind":3}`, string(payload))
}

func TestCompletionItemKeepsPopulatedDocumentation(t *testing.T) {
	item := CompletionItem{Label: "isActive", Kind: int(MethodCompletion)}
	item.Documentation.Kind = string(Markdown)
	item.Documentation.Value = "Determines whether a feature is active."

	payload, err := json.Marshal(item)
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"label":"isActive","kind":2,"documentation":{"kind":"markdown",`+
			`"value":"Determines whether a feature is active."}}`,
		string(payload),
	)
}

// A list mixing documented and undocumented items is the common case, and the
// one that used to fail: a single item without a docblock was enough to make
// the whole response undecodable.
func TestCompletionListMixesDocumentedAndUndocumentedItems(t *testing.T) {
	documented := CompletionItem{Label: "isActive"}
	documented.Documentation.Kind = string(PlainText)
	documented.Documentation.Value = "Whether a feature is active."

	payload, err := json.Marshal(CompletionList{
		Items: []CompletionItem{{Label: "CACHE_REWORK"}, documented},
	})
	require.NoError(t, err)

	var decoded struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Len(t, decoded.Items, 2)

	_, undocumentedHasField := decoded.Items[0]["documentation"]
	assert.False(t, undocumentedHasField, "undocumented item must omit the field entirely")
	assert.Contains(t, decoded.Items[1], "documentation")
}
