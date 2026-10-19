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
	"cmp"
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

var (
	// The fields of an exception, and of one of its match entries.
	policyExceptionFields = []string{"reason", "match"}
	matchEntryFields      = []string{"policies", "targets"}
)

// PolicyExceptions is the set of active policy exceptions for one policy pack.
type PolicyExceptions struct {
	exceptions []policyException // in ID order
}

type policyException struct {
	id     string
	reason string
	match  []policyExceptionMatcher
}

// policyExceptionMatcher is a parsed match entry: it matches the violations of any of its policies on any of its
// target resources.
type policyExceptionMatcher struct {
	policies []string
	urns     []resource.URN
}

// matches reports whether the entry matches a violation of the given policy, reported against the given URN.
func (m policyExceptionMatcher) matches(policy string, urn resource.URN) bool {
	return slices.Contains(m.policies, policy) && slices.Contains(m.urns, urn)
}

// len returns the number of exceptions in the set.
func (s *PolicyExceptions) len() int {
	if s == nil {
		return 0
	}
	return len(s.exceptions)
}

// match returns the exceptions that except a violation of the given policy, reported against the given URN, lowest
// ID first.
func (s *PolicyExceptions) match(policy string, urn resource.URN) []apitype.PolicyEventException {
	if s == nil {
		return nil
	}
	var result []apitype.PolicyEventException
	for _, e := range s.exceptions {
		if slices.ContainsFunc(e.match, func(m policyExceptionMatcher) bool { return m.matches(policy, urn) }) {
			result = append(result, apitype.PolicyEventException{ID: e.id, Reason: e.reason})
		}
	}
	return result
}

// UnknownPolicyWarnings returns a warning for each policy named by an exception that isn't in the pack. That isn't
// an error, because a newer version of the pack may have removed the policy.
func (s *PolicyExceptions) UnknownPolicyWarnings(packName string, policies []plugin.AnalyzerPolicyInfo) []string {
	if s == nil {
		return nil
	}
	var warnings []string
	for _, e := range s.exceptions {
		var names []string
		for _, m := range e.match {
			for _, p := range m.policies {
				if !slices.Contains(names, p) {
					names = append(names, p)
				}
			}
		}
		for _, name := range names {
			if !slices.ContainsFunc(policies, func(p plugin.AnalyzerPolicyInfo) bool { return p.Name == name }) {
				warnings = append(warnings, fmt.Sprintf(
					"policy exception %q for policy pack %q names policy %q, which isn't in the pack",
					e.id, packName, name))
			}
		}
	}
	return warnings
}

// parsePolicyExceptionsBlock parses and validates the raw value of the "exceptions" key of a policy pack config file,
// which maps exception IDs to exceptions.
func parsePolicyExceptionsBlock(raw json.RawMessage) (*PolicyExceptions, error) {
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, errors.New(`"exceptions" must be an object that maps exception IDs to exceptions`)
	}

	set := &PolicyExceptions{}
	for _, id := range slices.SortedFunc(maps.Keys(entries), compareExceptionIDs) {
		e, err := parsePolicyException(id, entries[id])
		if err != nil {
			return nil, fmt.Errorf("policy exception %q: %w", id, err)
		}
		set.exceptions = append(set.exceptions, e)
	}
	if set.len() == 0 {
		return nil, nil
	}
	return set, nil
}

// parsePolicyException decodes and validates one exception.
func parsePolicyException(id string, raw json.RawMessage) (policyException, error) {
	result := policyException{id: id}
	if !tokens.IsName(id) {
		return result, errors.New("the ID must be non-empty and contain only letters, digits, '-', '.' and '_'")
	}

	fields, err := objectFields(raw)
	if err != nil {
		return result, err
	}
	if err := checkFields(fields, policyExceptionFields); err != nil {
		return result, err
	}
	var rawMatch []json.RawMessage
	if m, ok := fields["match"]; ok {
		if err := json.Unmarshal(m, &rawMatch); err != nil {
			return result, errors.New(`"match" must be a list of {"policies", "targets"} entries`)
		}
	}
	for i, m := range rawMatch {
		entryFields, err := objectFields(m)
		if err == nil {
			err = checkFields(entryFields, matchEntryFields)
		}
		if err != nil {
			return result, fmt.Errorf("match[%d]: %w", i, err)
		}
	}

	var e struct {
		Reason string                 `json:"reason"`
		Match  []policyExceptionMatch `json:"match"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return result, err
	}
	if strings.TrimSpace(e.Reason) == "" {
		return result, errors.New(`"reason" is required`)
	}
	result.reason = e.Reason
	if len(e.Match) == 0 {
		return result, errors.New(`"match" must list at least one {"policies", "targets"} entry`)
	}
	for i, m := range e.Match {
		matcher, err := parsePolicyExceptionMatch(m)
		if err != nil {
			return result, fmt.Errorf("match[%d]: %w", i, err)
		}
		result.match = append(result.match, matcher)
	}
	return result, nil
}

// policyExceptionMatch is a match entry as written in a config file.
type policyExceptionMatch struct {
	Policies []string `json:"policies"`
	Targets  []string `json:"targets"`
}

// parsePolicyExceptionMatch validates a match entry. Its targets are URNs, compared exactly.
func parsePolicyExceptionMatch(m policyExceptionMatch) (policyExceptionMatcher, error) {
	result := policyExceptionMatcher{policies: m.Policies}
	if len(m.Policies) == 0 {
		return result, errors.New(`"policies" must list at least one policy`)
	}
	for _, p := range m.Policies {
		switch p {
		case "":
			return result, errors.New(`"policies" can't contain an empty name`)
		case "all":
			return result, errors.New(`"policies" must name policies; "all" isn't allowed`)
		}
	}
	if len(m.Targets) == 0 {
		return result, errors.New(`"targets" must list at least one URN`)
	}
	for _, t := range m.Targets {
		urn, err := resource.ParseURN(t)
		if err != nil {
			return result, fmt.Errorf("target %q is not a valid URN: %w", t, err)
		}
		result.urns = append(result.urns, urn)
	}
	return result, nil
}

// objectFields decodes a JSON object into its fields.
func objectFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("must be an object")
	}
	return fields, nil
}

// checkFields returns an error for the first field that isn't in known.
func checkFields(fields map[string]json.RawMessage, known []string) error {
	for _, f := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(known, f) {
			return fmt.Errorf("unknown field %q", f)
		}
	}
	return nil
}

// compareExceptionIDs orders IDs naturally, so that runs of digits compare as numbers: EXC-2 comes before EXC-10.
// IDs that only differ in leading zeros, such as EXC-7 and EXC-007, are ordered byte by byte.
func compareExceptionIDs(a, b string) int {
	if c := compareNatural(a, b); c != 0 {
		return c
	}
	return strings.Compare(a, b)
}

func compareNatural(a, b string) int {
	for a != "" && b != "" {
		ra, rb := leadingRun(a), leadingRun(b)
		if isDigits(ra) && isDigits(rb) {
			na, nb := strings.TrimLeft(ra, "0"), strings.TrimLeft(rb, "0")
			if c := cmp.Compare(len(na), len(nb)); c != 0 {
				return c
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
		} else if c := strings.Compare(ra, rb); c != 0 {
			return c
		}
		a, b = a[len(ra):], b[len(rb):]
	}
	return strings.Compare(a, b)
}

// leadingRun returns the longest prefix of s made only of digits, or only of non-digits.
func leadingRun(s string) string {
	digit := isDigit(s[0])
	for i := range len(s) {
		if isDigit(s[i]) != digit {
			return s[:i]
		}
	}
	return s
}

// isDigits reports whether s starts with a digit. IDs are ASCII (see tokens.IsName).
func isDigits(s string) bool { return s != "" && isDigit(s[0]) }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// exceptionAnalyzer runs its policy pack's analyzer unchanged, and marks the resource violations that one of the
// pack's exceptions match by setting their Exceptions. The engine attributes stack violations to a URN after
// AnalyzeStack returns, so it matches them with ApplyStackPolicyExceptions.
type exceptionAnalyzer struct {
	plugin.Analyzer
	exceptions *PolicyExceptions
}

// WithPolicyExceptions applies a policy pack's exceptions to the violations its analyzer reports.
func WithPolicyExceptions(a plugin.Analyzer, exceptions *PolicyExceptions) plugin.Analyzer {
	if exceptions.len() == 0 {
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

// ApplyStackPolicyExceptions matches a stack policy violation, which the engine attributes to urn, against the
// exceptions of the analyzer that reported it. The analyzer must be the one WithPolicyExceptions returned, not
// another wrapper around it.
func ApplyStackPolicyExceptions(a plugin.Analyzer, d *plugin.AnalyzeDiagnostic, urn resource.URN) {
	if a, ok := a.(*exceptionAnalyzer); ok {
		a.apply(d, urn)
	}
}

func (a *exceptionAnalyzer) apply(d *plugin.AnalyzeDiagnostic, urn resource.URN) {
	d.Exceptions = a.exceptions.match(d.PolicyName, urn)
}
