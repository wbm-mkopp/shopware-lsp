package codeaction

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	twigsyntax "github.com/shopware/shopware-lsp/internal/parser/twig/syntax"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/translation"
	"github.com/shopware/shopware-lsp/internal/uriutil"
)

const (
	extractTwigTranslationAction = "shopware.symfony.extractTwigTranslation"

	prepareTwigTranslationExtractionCommand  = "shopware/symfony/translation/extract/prepare"
	generateTwigTranslationExtractionCommand = "shopware/symfony/translation/extract/generate"
)

// TwigTranslationExtractProvider ports the reference plugin's Twig
// Extract Translation action. The language server owns selection validation,
// domain inference, escaping, and resource edits; clients only prompt for the
// desired key, domain, and target locale files.
type TwigTranslationExtractProvider struct {
	index *translation.Index
	host  lsp.WorkspaceEditHost
}

func NewTwigTranslationExtractProvider(
	index *translation.Index,
	host lsp.WorkspaceEditHost,
) *TwigTranslationExtractProvider {
	return &TwigTranslationExtractProvider{index: index, host: host}
}

func (p *TwigTranslationExtractProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{protocol.CodeActionRefactorExtract}
}

func (p *TwigTranslationExtractProvider) GetCodeActions(
	ctx context.Context,
	request *lsp.CodeActionRequest,
) []protocol.CodeAction {
	if ctx.Err() != nil || p == nil || p.index == nil ||
		request == nil || request.CodeActionParams == nil ||
		request.Document == nil || request.Document.SyntaxTree == nil ||
		request.Document.SyntaxLanguage != language.Twig {
		return nil
	}
	if _, ok := twigTranslationExtraction(
		request.Document.Text,
		request.Document.SyntaxTree.Root,
		request.Document.LineIndex,
		request.Range,
	); !ok {
		return nil
	}
	return []protocol.CodeAction{{
		Title: "Symfony: Extract Twig translation",
		Kind:  protocol.CodeActionRefactorExtract,
		Command: &protocol.CommandAction{
			Title:   "Symfony: Extract Twig translation",
			Command: extractTwigTranslationAction,
			Arguments: []any{
				request.TextDocument.URI,
				request.Range,
			},
		},
	}}
}

func (p *TwigTranslationExtractProvider) GetCommands(
	_ context.Context,
) map[string]lsp.CommandFunc {
	return map[string]lsp.CommandFunc{
		prepareTwigTranslationExtractionCommand:  p.prepare,
		generateTwigTranslationExtractionCommand: p.generate,
	}
}

type twigTranslationExtractionRequest struct {
	Version    *int           `json:"version,omitempty"`
	Preview    bool           `json:"preview,omitempty"`
	TargetURIs []string       `json:"targetUris,omitempty"`
	FileURI    string         `json:"fileUri"`
	Source     string         `json:"source"`
	Range      protocol.Range `json:"range"`
	Key        string         `json:"key,omitempty"`
	Domain     string         `json:"domain,omitempty"`
}

type twigTranslationExtractionPreparation struct {
	WorkspaceEdits bool           `json:"workspaceEdits"`
	Text           string         `json:"text"`
	Range          protocol.Range `json:"range"`
	DefaultKey     string         `json:"defaultKey,omitempty"`
	DefaultDomain  string         `json:"defaultDomain"`
	Domains        []string       `json:"domains"`
}

type twigTranslationExtractionTarget struct {
	FileURI   string `json:"fileUri"`
	File      string `json:"file"`
	Locale    string `json:"locale,omitempty"`
	Format    string `json:"format"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
	NewText   string `json:"newText"`
}

type twigTranslationExtractionEdits struct {
	Edit        *protocol.WorkspaceEdit           `json:"edit,omitempty"`
	Replacement string                            `json:"replacement"`
	Range       protocol.Range                    `json:"range"`
	Targets     []twigTranslationExtractionTarget `json:"targets"`
}

type twigTranslationSelection struct {
	text         string
	start        uint32
	end          uint32
	activeDomain string
}

func (p *TwigTranslationExtractProvider) prepare(
	ctx context.Context,
	raw *json.RawMessage,
) (interface{}, error) {
	params, selection, err := p.decodeTwigTranslationExtraction(ctx, raw)
	if err != nil {
		return nil, err
	}
	domains, err := p.index.GetDomains()
	if err != nil {
		return nil, err
	}
	domains = appendUniqueFold(domains, selection.activeDomain)
	sort.Slice(domains, func(left, right int) bool {
		return strings.ToLower(domains[left]) <
			strings.ToLower(domains[right])
	})
	return twigTranslationExtractionPreparation{
		WorkspaceEdits: true,
		Text:           selection.text,
		Range:          extractionProtocolRange(params.Source, selection),
		DefaultKey:     defaultTranslationKey(selection.text),
		DefaultDomain:  selection.activeDomain,
		Domains:        domains,
	}, nil
}

func (p *TwigTranslationExtractProvider) generate(
	ctx context.Context,
	raw *json.RawMessage,
) (interface{}, error) {
	params, selection, err := p.decodeTwigTranslationExtraction(ctx, raw)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(params.Key)
	domain := strings.TrimSpace(params.Domain)
	if key == "" {
		return nil, fmt.Errorf("translation key must not be empty")
	}
	if strings.ContainsAny(key, "\r\n\x00") {
		return nil, fmt.Errorf("translation key contains invalid characters")
	}
	if domain == "" || strings.ContainsAny(domain, "\r\n\x00") {
		return nil, fmt.Errorf("translation domain must not be empty")
	}
	params.Key = key
	params.Domain = domain
	exists, err := p.index.HasMessage(domain, key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf(
			"translation key %q already exists in domain %q",
			key,
			domain,
		)
	}
	insertions, err := p.index.InsertionTargets(domain)
	if err != nil {
		return nil, err
	}
	if len(insertions) == 0 {
		return nil, fmt.Errorf(
			"domain %q has no writable YAML or XLIFF resources",
			domain,
		)
	}
	targets := make(
		[]twigTranslationExtractionTarget,
		0,
		len(insertions),
	)
	for _, insertion := range insertions {
		// Older clients still receive preview positions, calculated from current snapshots.
		if !params.Preview && len(params.TargetURIs) == 0 {
			snapshot, err := p.host.ResolveDocument(ctx, uriutil.FileURI(insertion.File))
			if err != nil {
				return nil, err
			}
			computed, ok := translation.InsertionForSource(insertion.File, snapshot.Document.Source, key, selection.text)
			if !ok {
				continue
			}
			computed.Locale = insertion.Locale
			insertion = computed
		}
		targets = append(targets, twigTranslationExtractionTarget{
			FileURI:   uriutil.FileURI(insertion.File),
			File:      filepath.Base(insertion.File),
			Locale:    insertion.Locale,
			Format:    insertion.Format,
			Line:      insertion.Line,
			Character: insertion.Character,
			NewText:   insertion.NewText,
		})
	}
	replacement := "{{ '" + escapeTwigSingleQuoted(key) + "'|trans"
	if !strings.EqualFold(domain, selection.activeDomain) {
		replacement += "({}, '" + escapeTwigSingleQuoted(domain) + "')"
	}
	replacement += " }}"
	var edit *protocol.WorkspaceEdit
	if len(params.TargetURIs) != 0 {
		edit, err = p.extractionEdit(ctx, params, selection, replacement, targets)
		if err != nil {
			return nil, err
		}
	}
	return twigTranslationExtractionEdits{
		Edit:        edit,
		Replacement: replacement,
		Range:       extractionProtocolRange(params.Source, selection),
		Targets:     targets,
	}, nil
}

func (p *TwigTranslationExtractProvider) decodeTwigTranslationExtraction(
	ctx context.Context,
	raw *json.RawMessage,
) (twigTranslationExtractionRequest, twigTranslationSelection, error) {
	var params twigTranslationExtractionRequest
	if err := decodeSymfonyGeneratorRequest(raw, &params); err != nil {
		return params, twigTranslationSelection{}, err
	}
	if err := ctx.Err(); err != nil {
		return params, twigTranslationSelection{}, err
	}
	if strings.TrimSpace(params.FileURI) == "" {
		return params, twigTranslationSelection{}, fmt.Errorf(
			"missing Twig file URI",
		)
	}
	extension := strings.ToLower(filepath.Ext(params.FileURI))
	if extension != ".twig" && extension != ".html" {
		return params, twigTranslationSelection{}, fmt.Errorf(
			"translation extraction requires a Twig template",
		)
	}
	if p.host == nil {
		return params, twigTranslationSelection{}, fmt.Errorf("workspace edit host is unavailable")
	}
	snapshot, err := p.host.ResolveDocument(ctx, params.FileURI)
	if err != nil {
		return params, twigTranslationSelection{}, err
	}
	if snapshot.Document.Source != params.Source || params.Version != nil && (snapshot.Version == nil || *params.Version != *snapshot.Version) {
		return params, twigTranslationSelection{}, rewrite.ErrStaleHandle
	}
	tree := snapshot.Document.SyntaxTree
	if tree == nil || tree.Root == nil {
		return params, twigTranslationSelection{}, fmt.Errorf("parse Twig template")
	}
	lineIndex := snapshot.Document.LineIndex
	selection, ok := twigTranslationExtraction(
		[]byte(params.Source),
		tree.Root,
		lineIndex,
		params.Range,
	)
	if !ok {
		return params, twigTranslationSelection{}, fmt.Errorf(
			"select static Twig text or an HTML attribute value",
		)
	}
	selection.activeDomain = translation.TwigDefaultDomainBefore(
		[]byte(params.Source),
		selection.start,
	)
	if selection.activeDomain == "" {
		selection.activeDomain = "messages"
	}
	return params, selection, nil
}

func twigTranslationExtraction(
	source []byte,
	root *cst.Node,
	lineIndex *cst.LineIndex,
	rng protocol.Range,
) (twigTranslationSelection, bool) {
	if root == nil || lineIndex == nil {
		return twigTranslationSelection{}, false
	}
	start := lineIndex.OffsetUTF16(
		uint32(rng.Start.Line),
		uint32(rng.Start.Character),
	)
	end := lineIndex.OffsetUTF16(
		uint32(rng.End.Line),
		uint32(rng.End.Character),
	)
	if end < start || start > uint32(len(source)) ||
		end > uint32(len(source)) {
		return twigTranslationSelection{}, false
	}
	if start == end {
		node := root.NodeAtOffset(start)
		container := twigExtractionContainer(node)
		if container == nil {
			return twigTranslationSelection{}, false
		}
		containerRange := container.Range()
		start, end = containerRange.Start, containerRange.End
	} else {
		startNode := root.NodeAtOffset(start)
		endNode := root.NodeAtOffset(end - 1)
		container := twigExtractionContainer(startNode)
		if container == nil || twigExtractionContainer(endNode) != container ||
			start < container.Range().Start || end > container.Range().End {
			return twigTranslationSelection{}, false
		}
	}
	rawText := source[start:end]
	trimmed := bytes.TrimSpace(rawText)
	if len(trimmed) == 0 ||
		bytes.Contains(trimmed, []byte("{{")) ||
		bytes.Contains(trimmed, []byte("{%")) ||
		bytes.Contains(trimmed, []byte("{#")) {
		return twigTranslationSelection{}, false
	}
	leading := bytes.Index(rawText, trimmed)
	start += uint32(leading)
	end = start + uint32(len(trimmed))
	return twigTranslationSelection{
		text:  string(trimmed),
		start: start,
		end:   end,
	}, true
}

func twigExtractionContainer(node *cst.Node) *cst.Node {
	if node == nil {
		return nil
	}
	for current := range node.Ancestors() {
		switch current.Kind() {
		case twigsyntax.HtmlText, twigsyntax.HtmlStringInner:
			return current
		}
	}
	return nil
}

func extractionProtocolRange(
	source string,
	selection twigTranslationSelection,
) protocol.Range {
	lineIndex := cst.NewLineIndex(source)
	startLine, startCharacter := lineIndex.PositionUTF16(selection.start)
	endLine, endCharacter := lineIndex.PositionUTF16(selection.end)
	return protocol.Range{
		Start: protocol.Position{
			Line:      int(startLine),
			Character: int(startCharacter),
		},
		End: protocol.Position{
			Line:      int(endLine),
			Character: int(endCharacter),
		},
	}
}

func defaultTranslationKey(value string) string {
	if utf8.RuneCountInString(value) >= 15 {
		return ""
	}
	return strings.ToLower(strings.ReplaceAll(value, " ", "."))
}

func appendUniqueFold(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	for _, candidate := range values {
		if strings.EqualFold(candidate, value) {
			return values
		}
	}
	return append(values, value)
}

var _ lsp.ActionProvider = (*TwigTranslationExtractProvider)(nil)
var _ lsp.CommandProvider = (*TwigTranslationExtractProvider)(nil)
