package protocol

// InsertTextFormat defines how inserted text should be interpreted
type InsertTextFormat int

const (
	// PlainTextFormat indicates the inserted text is interpreted as plain text
	PlainTextFormat InsertTextFormat = 1
	// SnippetTextFormat indicates the inserted text is interpreted as a snippet
	SnippetTextFormat InsertTextFormat = 2
)
