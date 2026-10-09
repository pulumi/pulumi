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
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// policyExceptionsKey is the reserved top-level key of a policy pack config file that holds its exceptions.
const policyExceptionsKey = "exceptions"

// The fields of an exception entry, and of one of its resource selectors, that this version understands.
var (
	policyExceptionFields  = []string{"policies", "stacks", "resources", "reason"}
	resourceSelectorFields = []string{"urn", "type", "name", "properties"}
)

// PolicyExceptions is the set of active policy exceptions for one policy pack.
type PolicyExceptions struct {
	exceptions []policyException // in ID order, so that matching is deterministic
}

// policyException is a parsed exception. A violation matches when it matches every filter that's set; a filter
// with no items matches every violation.
type policyException struct {
	id        string
	reason    string
	policies  []string
	stacks    []string // "<project>/<stack>"
	resources []resourceSelector
}

// resourceSelector matches a resource when every field that's set matches. The URN is matched exactly; the type,
// the name and property values are patterns.
type resourceSelector struct {
	urn        resource.URN
	typ, name  *glob
	properties []propertyMatcher
}

// propertyMatcher matches a resource when the property at path has one of values.
type propertyMatcher struct {
	path   resource.PropertyPath
	values []valueMatcher
}

// valueMatcher matches a property value: a string pattern, or a number, boolean or null.
type valueMatcher struct {
	pattern *glob
	scalar  any // float64, bool or nil; only used when pattern is nil
}

// Len returns the number of exceptions in the set.
func (s *PolicyExceptions) Len() int {
	if s == nil {
		return 0
	}
	return len(s.exceptions)
}

// Match returns the ID and reason of the exception that excepts a violation of the given policy, reported against
// the resource with the given URN and properties. When several exceptions match, the one with the lowest ID wins.
func (s *PolicyExceptions) Match(policy string, urn resource.URN, props property.Map) (string, string, bool) {
	if s == nil {
		return "", "", false
	}
	for _, e := range s.exceptions {
		if e.matches(policy, urn, props) {
			return e.id, e.reason, true
		}
	}
	return "", "", false
}

func (e *policyException) matches(policy string, urn resource.URN, props property.Map) bool {
	if len(e.policies) > 0 && !slices.Contains(e.policies, policy) {
		return false
	}
	if len(e.stacks) > 0 && (!urn.IsValid() || !slices.Contains(e.stacks, qualifiedStack(urn))) {
		return false
	}
	if len(e.resources) > 0 &&
		!slices.ContainsFunc(e.resources, func(r resourceSelector) bool { return r.matches(urn, props) }) {
		return false
	}
	return true
}

func (r *resourceSelector) matches(urn resource.URN, props property.Map) bool {
	if r.urn != "" && r.urn != urn {
		return false
	}
	if r.typ != nil || r.name != nil {
		// URN.Type and URN.Name panic on an invalid URN.
		if !urn.IsValid() {
			return false
		}
		if r.typ != nil && !r.typ.match(string(urn.Type())) {
			return false
		}
		if r.name != nil && !r.name.match(urn.Name()) {
			return false
		}
	}
	for _, p := range r.properties {
		if !p.matches(props) {
			return false
		}
	}
	return true
}

func (p *propertyMatcher) matches(props property.Map) bool {
	found := lookupProperty(property.New(props), p.path)
	if len(found) == 0 {
		// A property that isn't set matches null.
		return slices.ContainsFunc(p.values, func(v valueMatcher) bool { return v.pattern == nil && v.scalar == nil })
	}
	for _, v := range found {
		if slices.ContainsFunc(p.values, func(m valueMatcher) bool { return m.matches(v) }) {
			return true
		}
	}
	return false
}

// lookupProperty returns the values at path, which may contain "*" to visit every element of an array or every
// value of an object. It doesn't descend into unknown or secret values.
func lookupProperty(v property.Value, path resource.PropertyPath) []property.Value {
	if len(path) == 0 {
		return []property.Value{v}
	}
	if v.IsComputed() || v.Secret() {
		return nil
	}
	var next []property.Value
	switch key := path[0].(type) {
	case int:
		if v.IsArray() && key >= 0 && key < v.AsArray().Len() {
			next = []property.Value{v.AsArray().Get(key)}
		}
	case string:
		switch {
		case key == "*" && v.IsArray():
			next = v.AsArray().AsSlice()
		case key == "*" && v.IsMap():
			for _, child := range v.AsMap().AllStable {
				next = append(next, child)
			}
		case v.IsMap():
			if child, ok := v.AsMap().GetOk(key); ok {
				next = []property.Value{child}
			}
		}
	}
	var result []property.Value
	for _, child := range next {
		result = append(result, lookupProperty(child, path[1:])...)
	}
	return result
}

func (m valueMatcher) matches(v property.Value) bool {
	// An unknown or secret value never matches: it can't be compared, and an exception mustn't reveal a secret.
	if v.IsComputed() || v.Secret() {
		return false
	}
	if m.pattern != nil {
		return v.IsString() && m.pattern.match(v.AsString())
	}
	switch s := m.scalar.(type) {
	case nil:
		return v.IsNull()
	case bool:
		return v.IsBool() && v.AsBool() == s
	case float64:
		return v.IsNumber() && v.AsNumber() == s
	}
	return false
}

// UnknownPolicyWarnings returns a warning for each policy named by an exception that isn't in the pack. That isn't
// an error, because a newer version of the pack may have removed the policy.
func (s *PolicyExceptions) UnknownPolicyWarnings(packName string, policies []plugin.AnalyzerPolicyInfo) []string {
	if s == nil {
		return nil
	}
	var warnings []string
	for _, e := range s.exceptions {
		for _, name := range e.policies {
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
// An entry with a field this version doesn't recognize is always skipped with a warning: a newer field may
// narrow the exception's scope, and ignoring just that field would apply the exception more broadly than
// intended. An otherwise malformed entry, or one that can never match, is an error when strict is set (a local
// config file), and is skipped with a warning when it isn't (exceptions sent by the service).
func ParsePolicyExceptions(
	entries map[string]json.RawMessage, strict bool,
) (*PolicyExceptions, []string, error) {
	var warnings []string
	set := &PolicyExceptions{}

	ids := slices.SortedFunc(maps.Keys(entries), compareExceptionIDs)
	for _, id := range ids {
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
		set.exceptions = append(set.exceptions, e)
	}

	if set.Len() == 0 {
		return nil, warnings, nil
	}
	return set, warnings, nil
}

// parsePolicyException decodes and validates one entry. It returns the names of any fields it doesn't
// recognize; the entry must not be applied in that case.
func parsePolicyException(id string, raw json.RawMessage) (policyException, []string, error) {
	result := policyException{id: id}
	if !tokens.IsName(id) {
		return result, nil, errors.New("the ID must be non-empty and contain only letters, digits, '-', '.' and '_'")
	}

	// Check for unknown fields in the entry and in each of its resource selectors before decoding it.
	fields, err := objectFields(raw)
	if err != nil {
		return result, nil, err
	}
	unknown := unknownFields(fields, policyExceptionFields, "")
	var rawResources []json.RawMessage
	if r, ok := fields["resources"]; ok {
		if err := json.Unmarshal(r, &rawResources); err != nil {
			return result, nil, errors.New(`"resources" must be a list of objects`)
		}
	}
	for i, r := range rawResources {
		selectorFields, err := objectFields(r)
		if err != nil {
			return result, nil, fmt.Errorf("resources[%d]: %w", i, err)
		}
		unknown = append(unknown, unknownFields(selectorFields, resourceSelectorFields, fmt.Sprintf("resources[%d].", i))...)
	}
	if len(unknown) > 0 {
		return result, unknown, nil
	}

	var e apitype.PolicyException
	if err := json.Unmarshal(raw, &e); err != nil {
		return result, nil, err
	}

	if strings.TrimSpace(e.Reason) == "" {
		return result, nil, errors.New(`"reason" is required`)
	}
	result.reason = e.Reason

	// Policies and stacks are matched exactly. Neither a policy name nor a stack name can contain "*", so one
	// there is a pattern that would never match.
	for _, p := range e.Policies {
		if p == "" {
			return result, nil, errors.New(`"policies" can't contain an empty name`)
		}
		if strings.Contains(p, "*") {
			return result, nil, fmt.Errorf(`"policies" item %q: policies are matched by exact name, without patterns`, p)
		}
		result.policies = append(result.policies, p)
	}
	for _, s := range e.Stacks {
		project, stack, ok := strings.Cut(s, "/")
		if !ok || project == "" || stack == "" || strings.Contains(stack, "/") {
			return result, nil, fmt.Errorf(`"stacks" item %q must have the form "<project>/<stack>"`, s)
		}
		if strings.Contains(s, "*") {
			return result, nil, fmt.Errorf(`"stacks" item %q: stacks are matched by exact name, without patterns`, s)
		}
		result.stacks = append(result.stacks, s)
	}
	for i, r := range e.Resources {
		selector, err := parseResourceSelector(r, result.stacks)
		if err != nil {
			return result, nil, fmt.Errorf("resources[%d]: %w", i, err)
		}
		result.resources = append(result.resources, selector)
	}
	return result, nil, nil
}

// objectFields decodes a JSON object into its fields.
func objectFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("must be an object")
	}
	return fields, nil
}

// unknownFields returns the fields that aren't in known, each prefixed with prefix.
func unknownFields(fields map[string]json.RawMessage, known []string, prefix string) []string {
	var unknown []string
	for _, f := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(known, f) {
			unknown = append(unknown, prefix+f)
		}
	}
	return unknown
}

func parseResourceSelector(r apitype.PolicyExceptionResource, stacks []string) (resourceSelector, error) {
	var result resourceSelector
	optionalGlob := func(value string) *glob {
		if value == "" {
			return nil
		}
		return new(compileGlob(value))
	}
	result.typ = optionalGlob(r.Type)
	result.name = optionalGlob(r.Name)

	// A URN names one resource: check that the rest of the exception can match it.
	if r.URN != "" {
		urn, err := resource.ParseURN(r.URN)
		if err != nil {
			return result, fmt.Errorf(`"urn" %q is not a valid URN: %w`, r.URN, err)
		}
		result.urn = urn
		if result.typ != nil && !result.typ.match(string(urn.Type())) {
			return result, fmt.Errorf(`"urn" %q never matches "type" %q`, r.URN, r.Type)
		}
		if result.name != nil && !result.name.match(urn.Name()) {
			return result, fmt.Errorf(`"urn" %q never matches "name" %q`, r.URN, r.Name)
		}
		if len(stacks) > 0 && !slices.Contains(stacks, qualifiedStack(urn)) {
			return result, fmt.Errorf(`"urn" %q is in stack %q, which "stacks" doesn't include`,
				r.URN, qualifiedStack(urn))
		}
	}

	for _, path := range slices.Sorted(maps.Keys(r.Properties)) {
		matcher, err := parsePropertyMatcher(path, r.Properties[path])
		if err != nil {
			return result, fmt.Errorf(`"properties" %q: %w`, path, err)
		}
		result.properties = append(result.properties, matcher)
	}
	return result, nil
}

func parsePropertyMatcher(path string, value any) (propertyMatcher, error) {
	result := propertyMatcher{}
	parsed, err := resource.ParsePropertyPath(path)
	if err != nil {
		return result, fmt.Errorf("invalid property path: %w", err)
	}
	if len(parsed) == 0 {
		return result, errors.New("the property path is empty")
	}
	result.path = parsed

	values, isList := value.([]any)
	if !isList {
		values = []any{value}
	}
	if len(values) == 0 {
		return result, errors.New("an empty list of values never matches")
	}
	for _, v := range values {
		switch v := v.(type) {
		case string:
			g := compileGlob(v)
			result.values = append(result.values, valueMatcher{pattern: &g})
		case float64, bool, nil:
			result.values = append(result.values, valueMatcher{scalar: v})
		default:
			return result, errors.New("a value must be a string, number, boolean or null, or a list of these; " +
				"to match inside an object or a list, use a longer path")
		}
	}
	return result, nil
}

// qualifiedStack returns the "<project>/<stack>" name of a valid URN's stack.
func qualifiedStack(urn resource.URN) string {
	return string(urn.Project()) + "/" + string(urn.Stack())
}

// glob is a pattern in which `*` matches any run of characters, and every other character matches itself. It must
// match the whole value. These are the semantics of the exemption patterns that policy packs accept in their config.
type glob struct {
	pattern string
	// The literal text between wildcards. A single segment means the pattern has no wildcard.
	segments []string
}

func compileGlob(pattern string) glob {
	collapsed := pattern
	for strings.Contains(collapsed, "**") {
		collapsed = strings.ReplaceAll(collapsed, "**", "*")
	}
	return glob{pattern: pattern, segments: strings.Split(collapsed, "*")}
}

func (g glob) match(value string) bool {
	if len(g.segments) == 1 {
		return value == g.segments[0]
	}
	first, last := g.segments[0], g.segments[len(g.segments)-1]
	if len(value) < len(first)+len(last) || !strings.HasPrefix(value, first) || !strings.HasSuffix(value, last) {
		return false
	}
	// Match the segments in between leftmost-first; for `*`-only patterns that finds a match whenever one exists.
	rest := value[len(first) : len(value)-len(last)]
	for _, segment := range g.segments[1 : len(g.segments)-1] {
		i := strings.Index(rest, segment)
		if i < 0 {
			return false
		}
		rest = rest[i+len(segment):]
	}
	return true
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
	// The engine reports these violations against the analyzed resource. Its properties are its inputs, which is
	// what the policy saw.
	for i := range resp.Diagnostics {
		a.apply(&resp.Diagnostics[i], r.URN, r.Properties)
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
	// AnalyzeStack has already attributed each violation to its URN; see plugin.AttributeStackDiagnostics. The
	// properties are that resource's outputs, which is what the policy saw.
	props := make(map[resource.URN]property.Map, len(resources))
	for _, r := range resources {
		props[r.URN] = r.Properties
	}
	for i := range resp.Diagnostics {
		d := &resp.Diagnostics[i]
		a.apply(d, d.URN, props[d.URN])
	}
	return resp, nil
}

func (a *exceptionAnalyzer) apply(d *plugin.AnalyzeDiagnostic, urn resource.URN, props property.Map) {
	if id, reason, ok := a.exceptions.Match(d.PolicyName, urn, props); ok {
		d.Exception = &apitype.PolicyEventException{ID: id, Reason: reason}
	}
}
