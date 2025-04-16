package protocol

// CodeLensParams represents the parameters for a code lens request
type CodeLensParams struct {
	// The document to request code lenses for
	TextDocument struct {
		URI string `json:"uri"`
	} `json:"textDocument"`
	// An optional token that a server can use to report work done progress
	WorkDoneToken interface{} `json:"workDoneToken,omitempty"`
	// An optional token that a server can use to report partial results
	PartialResultToken interface{} `json:"partialResultToken,omitempty"`
}

// CodeLens represents a command that should be shown along with source text
type CodeLens struct {
	// The range in which this code lens is valid. Should only span a single line
	Range Range `json:"range"`
	// The command this code lens represents
	Command *Command `json:"command,omitempty"`
	// A data entry field that is preserved on a code lens item between a code lens and a code lens resolve request
	Data interface{} `json:"data,omitempty"`
}

// Command represents a reference to a command
type Command struct {
	// Title of the command, like `save`
	Title string `json:"title"`
	// The identifier of the actual command handler
	Command string `json:"command"`
	// Arguments that the command handler should be invoked with
	Arguments []interface{} `json:"arguments,omitempty"`
}
