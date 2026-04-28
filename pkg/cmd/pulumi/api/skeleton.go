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

package api

import (
	"encoding/json"
)

// skeletonMaxDepth caps recursion when generating a body skeleton from an
// OpenAPI schema. Real request bodies rarely nest this deep; the limit
// protects against runaway expansion on pathological schemas.
const skeletonMaxDepth = 6

// GenerateBodySkeleton produces a JSON body template from an inline schema
// (the shape emitted by Operation.BodySchemaJSON). The template includes
// every required field at every nesting level, populated with a type- and
// format-appropriate placeholder. Optional fields are omitted — users add
// them explicitly, and the full list is visible via `describe`.
//
// Returns nil if the schema is empty or unparseable; callers should fall
// back to an empty object skeleton.
func GenerateBodySkeleton(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}
	val := skeletonValue(schema, 0)
	out, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return nil
	}
	return out
}

// skeletonValue walks an OpenAPI-style schema object and returns a suitable
// placeholder value for its type.
func skeletonValue(s map[string]any, depth int) any {
	if s == nil || depth > skeletonMaxDepth {
		return nil
	}

	// Honor examples / default when present — the spec is the authority on
	// what a good placeholder looks like.
	if ex, ok := s["example"]; ok {
		return ex
	}
	if exs, ok := s["examples"].([]any); ok && len(exs) > 0 {
		return exs[0]
	}
	if def, ok := s["default"]; ok {
		return def
	}

	// Enum: first allowed value is a safe, valid starting point.
	if enum, ok := s["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}

	// allOf: merge properties + required from all branches, then recurse as
	// a synthetic object schema.
	if ao, ok := s["allOf"].([]any); ok && len(ao) > 0 {
		return skeletonValue(mergeAllOf(ao), depth)
	}

	// oneOf / anyOf: pick the first branch as a representative body.
	// Discriminated unions usually list the required discriminator on every
	// branch so the first one is a valid, complete body.
	if branch := firstBranch(s, "oneOf"); branch != nil {
		return skeletonValue(branch, depth+1)
	}
	if branch := firstBranch(s, "anyOf"); branch != nil {
		return skeletonValue(branch, depth+1)
	}

	switch schemaTypeString(s) {
	case "string":
		return stringPlaceholder(s)
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return false
	case "array":
		if items, ok := s["items"].(map[string]any); ok {
			return []any{skeletonValue(items, depth+1)}
		}
		return []any{}
	case "object", "":
		return objectSkeleton(s, depth)
	}
	return nil
}

// objectSkeleton fills in required fields of an object schema, recursing
// into nested schemas. Optional fields are omitted to keep the template
// compact; the full field list lives in `describe`.
func objectSkeleton(s map[string]any, depth int) map[string]any {
	out := make(map[string]any)
	required := stringSlice(s["required"])
	props, _ := s["properties"].(map[string]any)
	for _, name := range required {
		child, ok := props[name].(map[string]any)
		if !ok {
			out[name] = nil
			continue
		}
		out[name] = skeletonValue(child, depth+1)
	}
	return out
}

// schemaTypeString returns the first entry from a schema's `type` field,
// which may be a string (OpenAPI 3.0) or array of strings (3.1).
func schemaTypeString(s map[string]any) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	return ""
}

// stringPlaceholder returns a format-appropriate placeholder for a string
// field so the skeleton passes basic client-side validation at a glance.
func stringPlaceholder(s map[string]any) string {
	format, _ := s["format"].(string)
	switch format {
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "email":
		return "user@example.com"
	case "date-time", "datetime":
		return "2026-01-01T00:00:00Z"
	case "date":
		return "2026-01-01"
	case "uri", "url":
		return "https://example.com"
	case "password":
		return "<password>"
	}
	return ""
}

// stringSlice coerces a []any of strings into []string, dropping
// non-string entries. Returns nil when the input is not a slice.
func stringSlice(v any) []string {
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, e := range slice {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// mergeAllOf collapses an allOf list into a single synthetic object schema
// by unioning properties and required-field lists.
func mergeAllOf(ao []any) map[string]any {
	props := make(map[string]any)
	reqSet := make(map[string]bool)
	for _, a := range ao {
		branch, ok := a.(map[string]any)
		if !ok {
			continue
		}
		if bp, ok := branch["properties"].(map[string]any); ok {
			for k, v := range bp {
				props[k] = v
			}
		}
		for _, r := range stringSlice(branch["required"]) {
			reqSet[r] = true
		}
	}
	required := make([]any, 0, len(reqSet))
	for r := range reqSet {
		required = append(required, r)
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// firstBranch returns the first map in a polymorphic list (oneOf/anyOf) or
// nil when the key is absent or empty.
func firstBranch(s map[string]any, key string) map[string]any {
	list, ok := s[key].([]any)
	if !ok || len(list) == 0 {
		return nil
	}
	m, _ := list[0].(map[string]any)
	return m
}
