package diagnostics

import (
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/suggestion"
	"github.com/shopware/shopware-lsp/internal/twigcomponent"
)

func (r *twigComponentDiagnosticRun) collectLiveDiagnostics() error {
	components, componentErr := r.index.ComponentsForTemplate(r.path)
	if componentErr != nil {
		return componentErr
	}
	live := false
	for _, component := range components {
		if component.Live {
			live = true
			break
		}
	}
	if !live {
		return nil
	}
	actions, actionErr := r.index.LiveActionsForTemplate(r.path)
	if actionErr != nil {
		return actionErr
	}
	r.collectMissingLiveActions(actions)
	r.collectMissingLiveArguments(actions)
	return nil
}

func (r *twigComponentDiagnosticRun) collectMissingLiveActions(actions []twigcomponent.LiveAction) {
	actionNames := make([]string, 0, len(actions))
	for _, action := range actions {
		actionNames = append(actionNames, action.Name)
	}
	for _, reference := range twigcomponent.LiveActionReferencesInTwig(
		r.path,
		r.document.SyntaxTree.Root,
	) {
		if r.ctx.Err() != nil {
			return
		}
		if reference.Name == "" ||
			containsFold(actionNames, reference.Name) {
			continue
		}
		r.problems = append(r.problems, lsp.Problem{
			Range: reference.Range,
			Message: fmt.Sprintf(
				"Live Action '%s' not found on this component",
				reference.Name,
			),
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "twig",
			ID:       missingLiveActionCode,
			Payload: map[string]any{
				"suggestions": suggestion.Similar(
					reference.Name,
					actionNames,
				),
			},
		})
	}
}

func (r *twigComponentDiagnosticRun) collectMissingLiveArguments(actions []twigcomponent.LiveAction) {
	for _, reference := range twigcomponent.LiveActionArgumentReferencesInTwig(
		r.path,
		r.document.SyntaxTree.Root,
	) {
		if r.ctx.Err() != nil {
			return
		}
		var parameters []twigcomponent.LiveActionParameter
		for _, action := range actions {
			if strings.EqualFold(action.Name, reference.Action) {
				parameters = append(parameters, action.Parameters...)
			}
		}
		if len(parameters) == 0 {
			continue
		}
		names := make([]string, 0, len(parameters))
		found := false
		for _, parameter := range parameters {
			names = append(names, parameter.Name)
			if strings.EqualFold(parameter.Name, reference.Name) {
				found = true
			}
		}
		if found {
			continue
		}
		suggestions := suggestion.Similar(reference.Name, names)
		for index := range suggestions {
			suggestions[index] = liveArgumentAttributeSegment(
				suggestions[index],
			)
		}
		r.problems = append(r.problems, lsp.Problem{
			Range: reference.Range,
			Message: fmt.Sprintf(
				"Live Action '%s' has no argument named '%s'",
				reference.Action,
				reference.Name,
			),
			Severity: protocol.DiagnosticSeverityWarning,
			Source:   "twig",
			ID:       missingLiveArgumentCode,
			Payload: map[string]any{
				"suggestions": suggestions,
			},
		})
	}
}
