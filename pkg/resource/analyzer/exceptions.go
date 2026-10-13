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
	"unicode"

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
	matchEntryFields      = []string{"policy", "urn"}
	// Fields reserved for later versions of match entries.
	reservedMatchEntryFields = []string{"policies", "stacks", "resources"}
)

// PolicyExceptions is the set of active policy exceptions for one policy pack.
type PolicyExceptions struct {
	exceptions []policyException // in ID order
	// The exception that excepts each (policy, URN) pair: the one with the lowest ID that lists it.
	byMatch map[exceptionKey]*policyException
}

type policyException struct {
	id     string
	reason string
	match  []exceptionKey
}

type exceptionKey struct {
	policy string
	urn    resource.URN
}

// Len returns the number of exceptions in the set.
func (s *PolicyExceptions) Len() int {
	if s == nil {
		return 0
	}
	return len(s.exceptions)
}

// Match returns the ID and reason of the exception that excepts a violation of the given policy, reported against
// the given URN. When several exceptions match, the one with the lowest ID wins.
func (s *PolicyExceptions) Match(policy string, urn resource.URN) (string, string, bool) {
	if s == nil {
		return "", "", false
	}
	e, ok := s.byMatch[exceptionKey{policy: policy, urn: urn}]
	if !ok {
		return "", "", false
	}
	return e.id, e.reason, true
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
			if !slices.Contains(names, m.policy) {
				names = append(names, m.policy)
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
// An invalid exception, including one with a field this version doesn't recognize, is an error when strict is set
// (a local config file). When it isn't (exceptions sent by the service), the exception is skipped with a warning, so
// the violations it lists still fail the operation.
func ParsePolicyExceptions(
	entries map[string]json.RawMessage, strict bool,
) (*PolicyExceptions, []string, error) {
	var warnings []string
	set := &PolicyExceptions{byMatch: map[exceptionKey]*policyException{}}

	ids := slices.SortedFunc(maps.Keys(entries), compareExceptionIDs)
	for _, id := range ids {
		e, err := parsePolicyException(id, entries[id])
		if err != nil {
			err = fmt.Errorf("policy exception %q: %w", id, err)
			if strict {
				return nil, nil, err
			}
			warnings = append(warnings, err.Error()+"; ignoring it")
			continue
		}
		set.exceptions = append(set.exceptions, e)
	}

	if set.Len() == 0 {
		return nil, warnings, nil
	}
	// Exceptions are in ID order, so the first one to list a pair is the one that excepts it.
	for i := range set.exceptions {
		e := &set.exceptions[i]
		for _, key := range e.match {
			if _, ok := set.byMatch[key]; !ok {
				set.byMatch[key] = e
			}
		}
	}
	return set, warnings, nil
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
	if err := checkFields(fields, policyExceptionFields, nil); err != nil {
		return result, err
	}
	var rawMatch []json.RawMessage
	if m, ok := fields["match"]; ok {
		if err := json.Unmarshal(m, &rawMatch); err != nil {
			return result, errors.New(`"match" must be a list of {"policy", "urn"} entries`)
		}
	}
	for i, m := range rawMatch {
		entryFields, err := objectFields(m)
		if err == nil {
			err = checkFields(entryFields, matchEntryFields, reservedMatchEntryFields)
		}
		if err != nil {
			return result, fmt.Errorf("match[%d]: %w", i, err)
		}
	}

	var e apitype.PolicyException
	if err := json.Unmarshal(raw, &e); err != nil {
		return result, err
	}
	if strings.TrimSpace(e.Reason) == "" {
		return result, errors.New(`"reason" is required`)
	}
	result.reason = e.Reason
	if len(e.Match) == 0 {
		return result, errors.New(`"match" must list at least one {"policy", "urn"} entry`)
	}
	for i, m := range e.Match {
		key, err := parseMatchEntry(m)
		if err != nil {
			return result, fmt.Errorf("match[%d]: %w", i, err)
		}
		result.match = append(result.match, key)
	}
	return result, nil
}

func parseMatchEntry(m apitype.PolicyExceptionMatch) (exceptionKey, error) {
	switch m.Policy {
	case "":
		return exceptionKey{}, errors.New(`"policy" is required`)
	case "all":
		return exceptionKey{}, errors.New(`"policy" must name one policy; "all" isn't allowed`)
	}
	if m.URN == "" {
		return exceptionKey{}, errors.New(`"urn" is required`)
	}
	urn, err := resource.ParseURN(m.URN)
	if err != nil {
		return exceptionKey{}, fmt.Errorf(`"urn" %q is not a valid URN: %w`, m.URN, err)
	}
	return exceptionKey{policy: m.Policy, urn: urn}, nil
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
func checkFields(fields map[string]json.RawMessage, known, reserved []string) error {
	for _, f := range slices.Sorted(maps.Keys(fields)) {
		switch {
		case slices.Contains(known, f):
		case slices.Contains(reserved, f):
			return fmt.Errorf(`%q isn't supported yet; list one "policy" and "urn" per entry`, f)
		default:
			return fmt.Errorf("unknown field %q", f)
		}
	}
	return nil
}

// compareExceptionIDs orders IDs naturally, so that runs of digits compare as numbers: EXC-2 comes before EXC-10.
func compareExceptionIDs(a, b string) int {
	for a != "" && b != "" {
		ra, rb := leadingRun(a), leadingRun(b)
		if isDigits(ra) && isDigits(rb) {
			na, nb := strings.TrimLeft(ra, "0"), strings.TrimLeft(rb, "0")
			if c := compareInts(len(na), len(nb)); c != 0 {
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
	digit := unicode.IsDigit(rune(s[0]))
	for i, r := range s {
		if unicode.IsDigit(r) != digit {
			return s[:i]
		}
	}
	return s
}

func isDigits(s string) bool { return s != "" && unicode.IsDigit(rune(s[0])) }

func compareInts(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
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
		d := &resp.Diagnostics[i]
		a.apply(d, d.URN)
	}
	return resp, nil
}

func (a *exceptionAnalyzer) apply(d *plugin.AnalyzeDiagnostic, urn resource.URN) {
	if id, reason, ok := a.exceptions.Match(d.PolicyName, urn); ok {
		d.Exception = &apitype.PolicyEventException{ID: id, Reason: reason}
	}
}
