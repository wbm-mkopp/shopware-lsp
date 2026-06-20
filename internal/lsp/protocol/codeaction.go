package protocol

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// CodeActionParams represents the parameters for a textDocument/codeAction request
type CodeActionParams struct {
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	Range   Range             `json:"range"`
	Context CodeActionContext `json:"context"`

	Node            *tree_sitter.Node `json:"-"`
	DocumentContent []byte            `json:"-"`
}

// CodeActionContext represents the context for a code action request
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Only        []string     `json:"only,omitempty"`
}

// CodeActionKind represents the kind of a code action
type CodeActionKind string

const (
	// CodeActionQuickFix represents a quick fix action
	CodeActionQuickFix CodeActionKind = "quickfix"
	// CodeActionRefactor represents a refactoring action
	CodeActionRefactor CodeActionKind = "refactor"
	// CodeActionRefactorExtract represents an extract refactoring action
	CodeActionRefactorExtract CodeActionKind = "refactor.extract"
	// CodeActionRefactorInline represents an inline refactoring action
	CodeActionRefactorInline CodeActionKind = "refactor.inline"
	// CodeActionRefactorRewrite represents a rewrite refactoring action
	CodeActionRefactorRewrite CodeActionKind = "refactor.rewrite"
	// CodeActionSource represents a source action
	CodeActionSource CodeActionKind = "source"
	// CodeActionSourceOrganizeImports represents an organize imports action
	CodeActionSourceOrganizeImports CodeActionKind = "source.organizeImports"
)

// CodeAction represents a code action
type CodeAction struct {
	Title       string         `json:"title"`
	Kind        CodeActionKind `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Command     *CommandAction `json:"command,omitempty"`
	Data        interface{}    `json:"data,omitempty"`
}

// CommandAction represents a command to be executed
type CommandAction struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// TextEdit represents a text edit operation
type TextEdit struct {
	Range            Range            `json:"range"`
	NewText          string           `json:"newText"`
	InsertTextFormat InsertTextFormat `json:"insertTextFormat,omitempty"`
}

// WorkspaceEdit represents a workspace edit operation.
//
// DocumentChanges is a heterogeneous list: it may contain DocumentChange
// (a TextDocumentEdit) entries as well as resource operations such as
// CreateFile. Entries are applied in order, so a CreateFile must precede the
// TextDocumentEdit that fills the new file. Use TextDocumentEdits to retrieve
// only the text edits regardless of position.
type WorkspaceEdit struct {
	Changes           map[string][]TextEdit       `json:"changes,omitempty"`
	DocumentChanges   []any                       `json:"documentChanges,omitempty"`
	ChangeAnnotations map[string]ChangeAnnotation `json:"changeAnnotations,omitempty"`
}

// TextDocumentEdits returns the TextDocumentEdit (DocumentChange) entries from
// DocumentChanges, skipping resource operations like CreateFile.
func (w *WorkspaceEdit) TextDocumentEdits() []DocumentChange {
	var edits []DocumentChange
	for _, change := range w.DocumentChanges {
		if dc, ok := change.(DocumentChange); ok {
			edits = append(edits, dc)
		}
	}
	return edits
}

// DocumentChange represents a TextDocumentEdit within DocumentChanges.
type DocumentChange struct {
	TextDocument OptionalVersionedTextDocumentIdentifier `json:"textDocument"`
	Edits        []TextEdit                              `json:"edits"`
	AnnotationID string                                  `json:"annotationId,omitempty"`
}

// CreateFile represents a create file resource operation within DocumentChanges.
type CreateFile struct {
	Kind         string             `json:"kind"` // always "create"
	URI          string             `json:"uri"`
	Options      *CreateFileOptions `json:"options,omitempty"`
	AnnotationID string             `json:"annotationId,omitempty"`
}

// CreateFileOptions holds options for a CreateFile resource operation.
type CreateFileOptions struct {
	Overwrite      bool `json:"overwrite,omitempty"`
	IgnoreIfExists bool `json:"ignoreIfExists,omitempty"`
}

// ChangeAnnotation represents an annotation for a change
type ChangeAnnotation struct {
	Label             string `json:"label"`
	NeedsConfirmation bool   `json:"needsConfirmation,omitempty"`
	Description       string `json:"description,omitempty"`
}

// OptionalVersionedTextDocumentIdentifier represents a text document identifier with an optional version
type OptionalVersionedTextDocumentIdentifier struct {
	URI string `json:"uri"`
	// Version is required by the LSP spec for a TextDocumentEdit and may be null
	// (no omitempty): a nil version must serialize as "version": null, otherwise
	// strict clients reject the whole workspace edit / code-action response.
	Version *int `json:"version"`
}
