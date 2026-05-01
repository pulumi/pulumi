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

package cloud

import (
	"path"
	"slices"
	"strings"

	"github.com/pgavlin/fx/v2"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// Binding is one resolved or deferred path-parameter value derived from
// matching a concrete path against an OpenAPI template.
type Binding struct {
	// Literal is the value the user supplied verbatim in the path
	// (e.g. "acme" for `/api/stacks/acme`). Empty when the user supplied
	// a template placeholder instead.
	Literal string
	// Placeholder is the alias the user wrote (e.g. "org" from `{org}`)
	// when they want the CLI to resolve the value from --org / context.
	// Empty when Literal is set.
	Placeholder string
}

// isTemplateSegment reports whether a path segment is wrapped in {braces}.
func isTemplateSegment(s string) bool {
	return len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}'
}

// trimBraces strips the surrounding {} from a template segment.
func trimBraces(s string) string {
	if !isTemplateSegment(s) {
		return s
	}
	return s[1 : len(s)-1]
}

// splitSegments normalises p with path.Clean and splits on "/".
func splitSegments(p string) []string {
	return strings.Split(path.Clean(p), "/")
}

// countTemplateParams returns the number of {param} segments in a spec path.
// More-specific paths (fewer params) win ties when multiple operations match.
func countTemplateParams(p string) int {
	n := 0
	for _, seg := range splitSegments(p) {
		if isTemplateSegment(seg) {
			n++
		}
	}
	return n
}

// MatchResult is the outcome of matching a user-provided path against the
// parsed spec.
type MatchResult struct {
	Op       *Operation
	Bindings map[string]Binding // keyed by spec param name (e.g. "orgName")
}

// MatchByOperationID finds the Operation in idx whose OperationID matches id
// case-insensitively. Returns a structured APIError on no match or on an
// ambiguous match.
func MatchByOperationID(idx *Index, id string) (*MatchResult, error) {
	matches := slices.Collect(fx.Filter(
		slices.Values(idx.Operations),
		func(op *Operation) bool { return strings.EqualFold(op.OperationID, id) },
	))
	if len(matches) == 0 {
		return nil, NewAPIError(cmdutil.ExitCodeError, ErrNoMatch,
			"no operation with ID "+id).
			WithSuggestions(
				"run 'pulumi cloud api list' to see available endpoints",
				"operation IDs are case-insensitive but must match exactly otherwise",
			)
	}
	if len(matches) > 1 {
		paths := make([]string, 0, len(matches))
		for _, op := range matches {
			paths = append(paths, op.Method+" "+op.Path)
		}
		return nil, NewAPIError(cmdutil.ExitCodeError, ErrNoMatch,
			"operation ID "+id+" matches multiple operations").
			WithSuggestions(paths...)
	}
	return &MatchResult{Op: matches[0], Bindings: map[string]Binding{}}, nil
}

// looksLikeOperationID reports whether s looks like an operation identifier
// (e.g. "ListAccounts") rather than a URL path.
func looksLikeOperationID(s string) bool {
	return s != "" && !strings.Contains(s, "/")
}

// splitLeadingHTTPMethod peels a leading HTTP verb off of s. Returns the
// verb upper-cased, the remainder trimmed, and ok=true when it found one.
// Verbs are the set in methodPrecedence (spec.go), so adding a new method
// to the spec auto-enables it here.
func splitLeadingHTTPMethod(s string) (verb, rest string, ok bool) {
	sp := strings.IndexByte(s, ' ')
	if sp <= 0 {
		return "", "", false
	}
	head := strings.ToUpper(s[:sp])
	if _, ok := methodPrecedence[head]; !ok {
		return "", "", false
	}
	return head, strings.TrimSpace(s[sp+1:]), true
}

// MatchPath finds the Operation in idx whose template best matches userPath
// for the given HTTP method. Returns a structured APIError on no match.
// Ties between equally-specific templates produce an "ambiguous" error.
//
// Matching rules, segment by segment:
//   - spec literal must equal user literal (case-sensitive)
//   - spec {param} captures either a user literal value, or a user {alias}
//     placeholder that the caller will resolve later
//   - a user {alias} cannot align with a spec literal — that means the user
//     typed the wrong path
//
// Specificity preference: among matches, pick the one with the fewest
// template parameters. If two are equally specific, return an error so the
// agent has to disambiguate explicitly.
func MatchPath(idx *Index, method, userPath string) (*MatchResult, error) {
	userSegs := splitSegments(userPath)

	var matches []*MatchResult
	for _, op := range idx.Operations {
		if op.Method != method {
			continue
		}
		specSegs := splitSegments(op.Path)
		if len(specSegs) != len(userSegs) {
			continue
		}

		bindings := make(map[string]Binding)
		ok := true
		for i, spec := range specSegs {
			user := userSegs[i]
			specIsParam := isTemplateSegment(spec)
			userIsParam := isTemplateSegment(user)

			switch {
			case specIsParam && userIsParam:
				bindings[trimBraces(spec)] = Binding{Placeholder: trimBraces(user)}
			case specIsParam && !userIsParam:
				bindings[trimBraces(spec)] = Binding{Literal: user}
			case !specIsParam && userIsParam:
				ok = false // can't parametrize a literal segment
			default:
				if spec != user {
					ok = false
				}
			}
			if !ok {
				break
			}
		}
		if ok {
			matches = append(matches, &MatchResult{Op: op, Bindings: bindings})
		}
	}

	if len(matches) == 0 {
		suggestions := []string{
			"run 'pulumi cloud api list' to see available endpoints",
			"check the method with -X / --method",
		}
		if looksLikeOperationID(userPath) {
			suggestions = append([]string{
				"if you meant an operation ID, try 'pulumi cloud api describe " + userPath + "'",
			}, suggestions...)
		}
		return nil, NewAPIError(cmdutil.ExitCodeError, ErrNoMatch,
			"no operation matches "+method+" "+userPath).
			WithSuggestions(suggestions...)
	}

	// Prefer the most specific (fewest param segments) match.
	best := matches[0]
	bestCount := countTemplateParams(best.Op.Path)
	tied := false
	for _, m := range matches[1:] {
		c := countTemplateParams(m.Op.Path)
		switch {
		case c < bestCount:
			best = m
			bestCount = c
			tied = false
		case c == bestCount:
			tied = true
		}
	}
	if tied {
		paths := make([]string, 0, len(matches))
		for _, m := range matches {
			if countTemplateParams(m.Op.Path) == bestCount {
				paths = append(paths, m.Op.Method+" "+m.Op.Path)
			}
		}
		return nil, NewAPIError(cmdutil.ExitCodeError, ErrNoMatch,
			"path matches multiple operations with equal specificity").
			WithSuggestions(paths...)
	}
	return best, nil
}

// splitPathQuery separates `?...` from a user-supplied path. Used by both
// describe (which discards the query) and the dispatcher (which forwards it).
func splitPathQuery(userPath string) (string, string) {
	path, query, _ := strings.Cut(userPath, "?")
	return path, query
}
