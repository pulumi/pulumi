// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

// policyExceptionsKey is the reserved top-level key of a policy pack config file that holds its exceptions.
const policyExceptionsKey = "exceptions"

// policyExceptionFields are the fields of an exception entry that this version understands.
var policyExceptionFields = []string{"policies", "stacks", "urns", "reason"}

// PolicyExceptions is the set of active policy exceptions for one policy pack.
type PolicyExceptions struct {
	ids  []string // sorted, so that matching is deterministic
	byID map[string]apitype.PolicyException
}

// Len returns the number of exceptions in the set.
func (s *PolicyExceptions) Len() int {
	if s == nil {
		return 0
	}
	return len(s.ids)
}

// Match returns the exception that excepts a violation of the given policy, attributed to the given URN.
// A match on a URN is more specific than a match on a stack and wins; otherwise the lowest ID wins.
func (s *PolicyExceptions) Match(policy string, urn resource.URN) (string, apitype.PolicyException, bool) {
	if s.Len() == 0 || !urn.IsValid() {
		return "", apitype.PolicyException{}, false
	}
	stack := string(urn.Project()) + "/" + string(urn.Stack())

	stackMatch := ""
	for _, id := range s.ids {
		e := s.byID[id]
		if !slices.Contains(e.Policies, policy) {
			continue
		}
		if slices.Contains(e.URNs, string(urn)) {
			return id, e, true
		}
		if stackMatch == "" && slices.Contains(e.Stacks, stack) {
			stackMatch = id
		}
	}
	if stackMatch != "" {
		return stackMatch, s.byID[stackMatch], true
	}
	return "", apitype.PolicyException{}, false
}

// UnknownPolicyWarnings returns a warning for each policy named by an exception that isn't in the pack.
// That isn't an error, because a newer version of the pack may have removed the policy.
func (s *PolicyExceptions) UnknownPolicyWarnings(packName string, policies []plugin.AnalyzerPolicyInfo) []string {
	if s.Len() == 0 {
		return nil
	}
	known := make(map[string]bool, len(policies))
	for _, p := range policies {
		known[p.Name] = true
	}
	var warnings []string
	for _, id := range s.ids {
		for _, p := range s.byID[id].Policies {
			if !known[p] {
				warnings = append(warnings, fmt.Sprintf(
					"policy exception %q for policy pack %q names policy %q, which isn't in the pack", id, packName, p))
			}
		}
	}
	return warnings
}

// ParsePolicyExceptionsBlock parses the raw value of the "exceptions" key of a policy pack config file.
// See ParsePolicyExceptions.
func ParsePolicyExceptionsBlock(raw json.RawMessage, strict bool) (*PolicyExceptions, []string, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		err = errors.New(`"exceptions" must be an object that maps exception IDs to exceptions`)
		if strict {
			return nil, nil, err
		}
		return nil, []string{err.Error() + "; ignoring all exceptions"}, nil
	}
	return ParsePolicyExceptions(entries, strict)
}

// ParsePolicyExceptions parses and validates policy exceptions keyed by ID.
//
// An entry with a field this version doesn't recognize is always skipped with a warning: a newer field may
// narrow the exception's scope, and ignoring just that field would apply the exception more broadly than
// intended. An otherwise malformed entry is an error when strict is set (a local config file), and is skipped
// with a warning when it isn't (exceptions sent by the service).
func ParsePolicyExceptions(
	entries map[string]json.RawMessage, strict bool,
) (*PolicyExceptions, []string, error) {
	var warnings []string
	set := &PolicyExceptions{byID: map[string]apitype.PolicyException{}}

	for _, id := range slices.Sorted(maps.Keys(entries)) {
		e, unknown, err := parsePolicyException(id, entries[id])
		if err != nil {
			err = fmt.Errorf("policy exception %q: %w", id, err)
			if strict {
				return nil, nil, err
			}
			warnings = append(warnings, err.Error()+"; ignoring it")
			continue
		}
		if len(unknown) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"policy exception %q has unrecognized fields (%s); ignoring it. "+
					"Try upgrading the Pulumi CLI, or check the configuration", id, strings.Join(unknown, ", ")))
			continue
		}
		set.ids = append(set.ids, id)
		set.byID[id] = e
	}

	if set.Len() == 0 {
		return nil, warnings, nil
	}
	return set, warnings, nil
}

// parsePolicyException decodes and validates one entry. It returns the names of any fields it doesn't
// recognize; the entry must not be applied in that case.
func parsePolicyException(id string, raw json.RawMessage) (apitype.PolicyException, []string, error) {
	var e apitype.PolicyException
	if !tokens.IsName(id) {
		return e, nil, errors.New("the ID must be non-empty and contain only letters, digits, '-', '.' and '_'")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return e, nil, errors.New("must be an object")
	}
	var unknown []string
	for _, f := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(policyExceptionFields, f) {
			unknown = append(unknown, f)
		}
	}
	if len(unknown) > 0 {
		return e, unknown, nil
	}

	if err := json.Unmarshal(raw, &e); err != nil {
		return e, nil, err
	}

	if len(e.Policies) == 0 {
		return e, nil, errors.New(`"policies" must name at least one policy`)
	}
	for _, p := range e.Policies {
		if p == "" {
			return e, nil, errors.New(`"policies" can't contain an empty name`)
		}
		if p == "all" {
			return e, nil, errors.New(`"policies" can't contain "all"; name each policy`)
		}
	}
	if strings.TrimSpace(e.Reason) == "" {
		return e, nil, errors.New(`"reason" is required`)
	}
	if len(e.Stacks) == 0 && len(e.URNs) == 0 {
		return e, nil, errors.New(`at least one target is required in "stacks" or "urns"`)
	}
	for _, s := range e.Stacks {
		project, stack, ok := strings.Cut(s, "/")
		if !ok || project == "" || stack == "" || strings.Contains(stack, "/") {
			return e, nil, fmt.Errorf(`"stacks" item %q must have the form "<project>/<stack>"`, s)
		}
	}
	for _, u := range e.URNs {
		if _, err := resource.ParseURN(u); err != nil {
			return e, nil, fmt.Errorf(`"urns" item %q is not a valid URN: %w`, u, err)
		}
	}
	return e, nil, nil
}

// exceptionAnalyzer runs its policy pack's analyzer unchanged, and marks the violations that one of the pack's
// exceptions matches by setting their Exception.
type exceptionAnalyzer struct {
	plugin.Analyzer
	exceptions *PolicyExceptions
}

// WithPolicyExceptions applies a policy pack's exceptions to the violations its analyzer reports.
func WithPolicyExceptions(a plugin.Analyzer, exceptions *PolicyExceptions) plugin.Analyzer {
	if exceptions.Len() == 0 {
		return a
	}
	return &exceptionAnalyzer{Analyzer: a, exceptions: exceptions}
}

func (a *exceptionAnalyzer) Analyze(ctx context.Context, r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
	resp, err := a.Analyzer.Analyze(ctx, r)
	if err != nil {
		return resp, err
	}
	// The engine reports these violations against the analyzed resource.
	for i := range resp.Diagnostics {
		a.apply(&resp.Diagnostics[i], r.URN)
	}
	return resp, nil
}

func (a *exceptionAnalyzer) AnalyzeStack(
	ctx context.Context, resources []plugin.AnalyzerStackResource,
) (plugin.AnalyzeResponse, error) {
	resp, err := a.Analyzer.AnalyzeStack(ctx, resources)
	if err != nil {
		return resp, err
	}
	// AnalyzeStack has already attributed each violation to its URN; see plugin.AttributeStackDiagnostics.
	for i := range resp.Diagnostics {
		a.apply(&resp.Diagnostics[i], resp.Diagnostics[i].URN)
	}
	return resp, nil
}

func (a *exceptionAnalyzer) apply(d *plugin.AnalyzeDiagnostic, urn resource.URN) {
	if id, e, ok := a.exceptions.Match(d.PolicyName, urn); ok {
		d.Exception = &apitype.PolicyEventException{ID: id, Reason: e.Reason}
	}
}
