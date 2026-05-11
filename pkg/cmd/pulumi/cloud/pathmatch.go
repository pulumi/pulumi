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
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/gorilla/mux"
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
	// when they want the CLI to resolve the value from -F or the current
	// Pulumi context. Empty when Literal is set.
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
	// Populate placeholder-style bindings by feeding the op's own templated
	// path back through MatchPath.
	op := matches[0]
	mr, err := MatchPath(idx, op.Method, op.Path)
	if err != nil {
		return &MatchResult{Op: op, Bindings: map[string]Binding{}}, nil
	}
	return mr, nil
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
//
// Routing is delegated to gorilla/mux. The captured vars are post-processed
// so that a captured value of the form `{alias}` is treated as a
// placeholder the caller will resolve later (org/project/stack auto-
// resolve, or -F field).
func MatchPath(idx *Index, method, userPath string) (*MatchResult, error) {
	// Synthesize a request for gorilla.Match. URL.Path is set directly so we
	// don't use url.Parse — `{}` characters in user input are valid here.
	req := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path.Clean(userPath)},
	}

	if idx.router != nil {
		var rm mux.RouteMatch
		if idx.router.Match(req, &rm) {
			op := idx.ByKey[rm.Route.GetName()]
			if op != nil {
				return &MatchResult{Op: op, Bindings: varsToBindings(rm.Vars)}, nil
			}
		}
	}

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

// compareOps orders two operations for router registration: per-segment
// static-first — at each path position a literal sorts before a {template},
// otherwise lexical. Mirrors the pulumi-service codegen ordering
// Returns -1, 0, or +1.
func compareOps(a, b *Operation) int {
	asegs := splitSegments(a.Path)
	bsegs := splitSegments(b.Path)
	n := len(asegs)
	if len(bsegs) < n {
		n = len(bsegs)
	}
	for i := 0; i < n; i++ {
		ah, bh := isTemplateSegment(asegs[i]), isTemplateSegment(bsegs[i])
		if ah != bh {
			if !ah {
				return -1
			}
			return 1
		}
		if c := strings.Compare(asegs[i], bsegs[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(asegs) < len(bsegs):
		return -1
	case len(asegs) > len(bsegs):
		return 1
	}
	return 0
}

// varsToBindings converts gorilla mux's captured path variables into our
// Binding form. A captured value wrapped in {braces} is treated as the
// user's placeholder alias (resolved later from -F or context); a bare
// value is taken as a literal.
func varsToBindings(vars map[string]string) map[string]Binding {
	bindings := make(map[string]Binding, len(vars))
	for name, val := range vars {
		if isTemplateSegment(val) {
			bindings[name] = Binding{Placeholder: trimBraces(val)}
		} else {
			bindings[name] = Binding{Literal: val}
		}
	}
	return bindings
}

// splitPathQuery separates `?...` from a user-supplied path. Used by both
// describe (which discards the query) and the dispatcher (which forwards it).
func splitPathQuery(userPath string) (string, string) {
	path, query, _ := strings.Cut(userPath, "?")
	return path, query
}
