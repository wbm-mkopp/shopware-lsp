package lsp

import (
	"context"
	"log"
)

func (s *Server) showDocumentAtLine(ctx context.Context, uri string, line int) {
	if s.conn == nil {
		return
	}

	lineIndex := line - 1
	if lineIndex < 0 {
		lineIndex = 0
	}

	position := map[string]int{"line": lineIndex, "character": 0}
	params := map[string]interface{}{
		"uri":       uri,
		"takeFocus": true,
		"selection": map[string]interface{}{
			"start": position,
			"end":   position,
		},
	}

	var result struct {
		Success bool `json:"success"`
	}
	if err := s.conn.Call(ctx, "window/showDocument", params, &result); err != nil {
		log.Printf("window/showDocument failed for %s: %v", uri, err)
	}
}
