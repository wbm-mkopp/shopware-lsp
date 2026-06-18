package lsp

import (
	"context"
	"log"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

func parseExtendBlockSuccess(result interface{}) (uri string, line int, ok bool) {
	if _, isErr := result.(*protocol.ShopwareLspError); isErr {
		return "", 0, false
	}

	m, isMap := result.(map[string]any)
	if !isMap {
		return "", 0, false
	}

	uri, _ = m["uri"].(string)
	if uri == "" {
		return "", 0, false
	}

	return uri, parseExtendBlockLine(m["line"]), true
}

func parseExtendBlockLine(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 1
	}
}

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
