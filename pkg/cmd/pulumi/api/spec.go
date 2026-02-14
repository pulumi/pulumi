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
	_ "embed"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
)

//go:embed openapi_public.json
var openAPISpecJSON []byte

// ParamSpec describes a parameter for an API operation.
type ParamSpec struct {
	Name        string
	In          string // "path", "query", "header"
	Type        string // "string", "integer", "boolean", "number"
	Required    bool
	Description string
	Values      []string // Allowed values for enum params (empty for non-enum)
}

// PathVariant describes an alternative path for a merged operation.
// When multiple OpenAPI operations share the same summary within a tag
// but have different paths (e.g., with/without a version segment),
// they are merged into a single command with PathVariants.
type PathVariant struct {
	Path                string
	BodyContentType     string
	ResponseContentType string
	BodySchemaText      string
	ResponseSchemaText  string
}

// OperationSpec describes an API operation.
type OperationSpec struct {
	OperationID         string
	CommandName         string // Kebab-cased name used as the cobra command name
	Method              string
	Path                string
	Summary             string
	Description         string
	Tag                 string
	Params              []ParamSpec
	HasBody             bool
	BodyContentType     string // e.g. "application/json", "application/x-yaml"
	ResponseContentType string // e.g. "application/json", "text/plain"
	BodySchemaText      string // Pre-rendered request body schema for help text
	ResponseSchemaText  string // Pre-rendered response schema for help text
	PathVariants        []PathVariant // Non-empty when merged from multiple paths
}

// tagGroup groups operations by their tag.
type tagGroup struct {
	Name       string // Original tag name e.g. "AI Agents"
	Slug       string // Kebab slug e.g. "ai-agents"
	Operations []OperationSpec
}

// httpMethodOps returns (method, *Operation) pairs for a PathItem in a fixed order.
func httpMethodOps(item *v3high.PathItem) []struct {
	Method string
	Op     *v3high.Operation
} {
	return []struct {
		Method string
		Op     *v3high.Operation
	}{
		{"GET", item.Get},
		{"POST", item.Post},
		{"PUT", item.Put},
		{"DELETE", item.Delete},
		{"PATCH", item.Patch},
		{"HEAD", item.Head},
	}
}

// parseEmbeddedSpec parses the embedded OpenAPI JSON into tag groups.
func parseEmbeddedSpec() ([]tagGroup, error) {
	doc, err := libopenapi.NewDocument(openAPISpecJSON)
	if err != nil {
		return nil, fmt.Errorf("parsing OpenAPI document: %w", err)
	}

	v3Model, errs := doc.BuildV3Model()
	if errs != nil {
		// BuildV3Model returns a list of errors; only fail on fatal ones.
		// Circular reference warnings are expected and non-fatal.
		if v3Model == nil {
			return nil, fmt.Errorf("building OpenAPI v3 model: %v", errs)
		}
	}

	model := v3Model.Model

	componentSchemas := model.Components.Schemas
	renderer := newSchemaRenderer(componentSchemas)

	tagOps := make(map[string][]OperationSpec)

	if model.Paths != nil && model.Paths.PathItems != nil {
		for path, pathItem := range model.Paths.PathItems.FromOldest() {
			for _, mo := range httpMethodOps(pathItem) {
				op := mo.Op
				if op == nil {
					continue
				}
				if op.OperationId == "" {
					continue
				}
				if op.Deprecated != nil && *op.Deprecated {
					continue
				}

				tag := "Miscellaneous"
				if len(op.Tags) > 0 {
					tag = op.Tags[0]
				}

				parsed := OperationSpec{
					OperationID: toKebab(op.OperationId),
					Method:      mo.Method,
					Path:        path,
					Summary:     op.Summary,
					Description: op.Description,
					Tag:         tag,
				}

				// Parse parameters.
				for _, p := range op.Parameters {
					if p == nil {
						continue
					}
					pp := ParamSpec{
						Name:        p.Name,
						In:          p.In,
						Required:    p.Required != nil && *p.Required,
						Description: p.Description,
					}
					if p.Schema != nil {
						resolved := p.Schema.Schema()
						if resolved != nil {
							pp.Type = schemaType(resolved)
						}
					}
					if pp.Type == "" {
						pp.Type = "string"
					}
					parsed.Params = append(parsed.Params, pp)
				}

				// Parse and render request body schema.
				if op.RequestBody != nil && op.RequestBody.Content != nil {
					parsed.HasBody = true
					parsed.BodyContentType = preferContentType(op.RequestBody.Content)
					if parsed.BodyContentType != "" {
						ct, ok := op.RequestBody.Content.Get(parsed.BodyContentType)
						if ok && ct != nil && ct.Schema != nil {
							resolved := ct.Schema.Schema()
							if resolved != nil {
								parsed.BodySchemaText = renderer.renderBodySchema(resolved)
							}
						}
					}
				}

				// Parse and render success response schema.
				if op.Responses != nil && op.Responses.Codes != nil {
					for _, code := range []string{"200", "201", "202"} {
						resp, ok := op.Responses.Codes.Get(code)
						if !ok || resp == nil || resp.Content == nil {
							continue
						}
						respCT := preferContentType(resp.Content)
						if parsed.ResponseContentType == "" {
							parsed.ResponseContentType = respCT
						}
						ct, ok := resp.Content.Get(respCT)
						if !ok || ct == nil || ct.Schema == nil {
							break
						}
						resolved := ct.Schema.Schema()
						if resolved != nil {
							parsed.ResponseSchemaText = renderer.renderResponseSchema(resolved)
						}
						break
					}
					// Fall back to default response if no success code found.
					if op.Responses.Default != nil && op.Responses.Default.Content != nil {
						if parsed.ResponseContentType == "" {
							parsed.ResponseContentType = preferContentType(op.Responses.Default.Content)
						}
						if parsed.ResponseSchemaText == "" {
							ct, ok := op.Responses.Default.Content.Get(parsed.ResponseContentType)
							if ok && ct != nil && ct.Schema != nil {
								resolved := ct.Schema.Schema()
								if resolved != nil {
									parsed.ResponseSchemaText = renderer.renderResponseSchema(resolved)
								}
							}
						}
					}
				}

				tagOps[tag] = append(tagOps[tag], parsed)
			}
		}
	}

	var tagNames []string
	for t := range tagOps {
		tagNames = append(tagNames, t)
	}
	sort.Strings(tagNames)

	var groups []tagGroup
	for _, name := range tagNames {
		ops := tagOps[name]

		// Merge colliding operations (same summary) into single commands.
		ops = mergeCollisions(ops)

		// Assign command names: use summary when unique, operationId when not.
		summaryCounts := make(map[string]int)
		for i := range ops {
			key := toKebab(ops[i].Summary)
			if key == "" {
				key = ops[i].OperationID
			}
			summaryCounts[key]++
		}
		for i := range ops {
			key := toKebab(ops[i].Summary)
			if key != "" && summaryCounts[key] == 1 {
				ops[i].CommandName = key
			} else {
				ops[i].CommandName = ops[i].OperationID
			}
		}

		sort.Slice(ops, func(i, j int) bool {
			return ops[i].CommandName < ops[j].CommandName
		})
		groups = append(groups, tagGroup{
			Name:       name,
			Slug:       tagToSlug(name),
			Operations: ops,
		})
	}

	return groups, nil
}

// --- Naming functions ---

// knownAcronyms lists acronyms treated as single words in kebab conversion.
var knownAcronyms = []string{
	"OIDC", "SAML", "SCIM", "YAML",
	"AWS", "ESC", "SSO", "IDP", "TTL", "MFA", "IAM", "ARN",
	"AI",
}

func toKebab(s string) string {
	parts := strings.Split(s, "_")
	var allWords []string
	for _, part := range parts {
		allWords = append(allWords, splitCamelCase(part)...)
	}
	for i := range allWords {
		allWords[i] = strings.ToLower(allWords[i])
	}
	return strings.Join(allWords, "-")
}

func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}

	acronymGroup := make([]int, len(s))
	remaining := s
	offset := 0
	for offset < len(s) {
		matched := false
		for ai, acr := range knownAcronyms {
			if strings.HasPrefix(remaining, acr) {
				for j := 0; j < len(acr); j++ {
					acronymGroup[offset+j] = ai + 1
				}
				offset += len(acr)
				remaining = s[offset:]
				matched = true
				break
			}
		}
		if !matched {
			offset++
			remaining = s[offset:]
		}
	}

	var words []string
	var current strings.Builder
	currentAcronym := 0

	for i, r := range s {
		ag := acronymGroup[i]
		if ag != 0 {
			if ag != currentAcronym {
				if current.Len() > 0 {
					words = append(words, current.String())
					current.Reset()
				}
				currentAcronym = ag
			}
			current.WriteRune(r)
		} else {
			if currentAcronym != 0 {
				if current.Len() > 0 {
					words = append(words, current.String())
					current.Reset()
				}
				currentAcronym = 0
			}
			if unicode.IsUpper(r) && current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func tagToSlug(tag string) string {
	if strings.Contains(tag, " ") {
		parts := strings.Fields(tag)
		for i := range parts {
			parts[i] = strings.ToLower(parts[i])
		}
		return strings.Join(parts, "-")
	}

	runes := []rune(tag)
	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			words = append(words, string(runes[start:i]))
			start = i
		}
		if i >= 2 && unicode.IsUpper(runes[i-2]) && unicode.IsUpper(runes[i-1]) && unicode.IsLower(runes[i]) {
			words = append(words, string(runes[start:i-1]))
			start = i - 1
		}
	}
	if start < len(runes) {
		words = append(words, string(runes[start:]))
	}
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "-")
}

// contentTypePreference is the order in which we prefer content types.
var contentTypePreference = []string{
	"application/json",
	"application/x-yaml",
}

// preferContentType picks the best content type key from an ordered map of media types.
// It prefers application/json, then application/x-yaml, then the first available key.
func preferContentType(content *orderedmap.Map[string, *v3high.MediaType]) string {
	if content == nil {
		return ""
	}
	for _, preferred := range contentTypePreference {
		if _, ok := content.Get(preferred); ok {
			return preferred
		}
	}
	// Fall back to the first available content type.
	for ct := range content.FromOldest() {
		return ct
	}
	return ""
}

// --- Collision merging ---

// mergeCollisions processes operations within a tag group to:
// 1. Remove preview-path duplicates (/api/preview/... when /api/... exists)
// 2. Merge operations that share the same summary into a single command
func mergeCollisions(ops []OperationSpec) []OperationSpec {
	ops = deduplicatePreview(ops)

	// Group by summary-based key.
	type summaryGroup struct {
		key string
		ops []OperationSpec
	}
	groups := make(map[string]*summaryGroup)
	var order []string
	for i := range ops {
		key := toKebab(ops[i].Summary)
		if key == "" {
			key = ops[i].OperationID
		}
		if _, ok := groups[key]; !ok {
			groups[key] = &summaryGroup{key: key}
			order = append(order, key)
		}
		groups[key].ops = append(groups[key].ops, ops[i])
	}

	var result []OperationSpec
	for _, key := range order {
		g := groups[key]
		if len(g.ops) == 1 {
			result = append(result, g.ops[0])
			continue
		}
		merged, ok := tryMerge(g.ops)
		if ok {
			result = append(result, merged)
		} else {
			// Could not merge; keep all with operationID-based names.
			result = append(result, g.ops...)
		}
	}
	return result
}

// deduplicatePreview removes operations at /api/preview/... paths when a
// non-preview equivalent exists at /api/... with the same method.
func deduplicatePreview(ops []OperationSpec) []OperationSpec {
	nonPreview := make(map[string]bool)
	for _, op := range ops {
		if !strings.HasPrefix(op.Path, "/api/preview/") {
			nonPreview[op.Method+"|"+op.Path] = true
		}
	}
	var result []OperationSpec
	for _, op := range ops {
		if strings.HasPrefix(op.Path, "/api/preview/") {
			equiv := "/api/" + strings.TrimPrefix(op.Path, "/api/preview/")
			if nonPreview[op.Method+"|"+equiv] {
				continue
			}
		}
		result = append(result, op)
	}
	return result
}

// tryMerge attempts to merge a group of operations with the same summary.
func tryMerge(ops []OperationSpec) (OperationSpec, bool) {
	if len(ops) < 2 {
		return ops[0], true
	}
	for _, op := range ops[1:] {
		if op.Method != ops[0].Method {
			return OperationSpec{}, false
		}
	}

	segs := make([][]string, len(ops))
	for i, op := range ops {
		segs[i] = strings.Split(op.Path, "/")
	}

	if merged, ok := tryLiteralMerge(ops, segs); ok {
		return merged, true
	}
	return tryParamMerge(ops, segs)
}

// tryLiteralMerge handles the case where all paths have the same length and
// differ in exactly one literal segment (e.g., destroy/preview/refresh/update).
// It creates a synthetic enum parameter for the varying segment.
func tryLiteralMerge(ops []OperationSpec, segs [][]string) (OperationSpec, bool) {
	n := len(segs[0])
	for _, s := range segs[1:] {
		if len(s) != n {
			return OperationSpec{}, false
		}
	}

	var diffPos []int
	for i := 0; i < n; i++ {
		first := segs[0][i]
		for _, s := range segs[1:] {
			if s[i] != first {
				diffPos = append(diffPos, i)
				break
			}
		}
	}

	if len(diffPos) != 1 {
		return OperationSpec{}, false
	}
	pos := diffPos[0]

	for _, s := range segs {
		if strings.HasPrefix(s[pos], "{") {
			return OperationSpec{}, false
		}
	}

	values := make([]string, len(segs))
	for i, s := range segs {
		values[i] = s[pos]
	}
	sort.Strings(values)

	flagName := deriveFlagName(segs[0], pos)

	mergedSegs := make([]string, n)
	copy(mergedSegs, segs[0])
	mergedSegs[pos] = "{" + flagName + "}"
	mergedPath := strings.Join(mergedSegs, "/")

	merged := ops[0]
	merged.Path = mergedPath
	merged.Params = make([]ParamSpec, len(ops[0].Params))
	copy(merged.Params, ops[0].Params)
	merged.Params = append(merged.Params, ParamSpec{
		Name:        flagName,
		In:          "path",
		Type:        "string",
		Required:    true,
		Description: fmt.Sprintf("Kind of operation (%s)", strings.Join(values, ", ")),
		Values:      values,
	})

	return merged, true
}

// tryParamMerge handles the case where operations have different-length paths
// (e.g., with/without a version segment, or with different param structures).
// It stores PathVariants and makes variant-specific params optional.
func tryParamMerge(ops []OperationSpec, segs [][]string) (OperationSpec, bool) {
	// Identify shared vs unique params across all operations.
	sharedParams := make(map[string]bool)
	for _, p := range ops[0].Params {
		sharedParams[p.Name] = true
	}
	for _, op := range ops[1:] {
		thisParams := make(map[string]bool)
		for _, p := range op.Params {
			thisParams[p.Name] = true
		}
		for name := range sharedParams {
			if !thisParams[name] {
				delete(sharedParams, name)
			}
		}
	}

	// Build the merged operation.
	merged := OperationSpec{
		OperationID:         ops[0].OperationID,
		Method:              ops[0].Method,
		Summary:             ops[0].Summary,
		Description:         ops[0].Description,
		Tag:                 ops[0].Tag,
		HasBody:             ops[0].HasBody,
		BodyContentType:     ops[0].BodyContentType,
		ResponseContentType: ops[0].ResponseContentType,
		BodySchemaText:      ops[0].BodySchemaText,
		ResponseSchemaText:  ops[0].ResponseSchemaText,
	}

	// Add shared params first (preserving order from first op).
	for _, p := range ops[0].Params {
		if sharedParams[p.Name] {
			merged.Params = append(merged.Params, p)
		}
	}

	// Add unique params as optional, collecting from all ops.
	seen := make(map[string]bool)
	for _, op := range ops {
		for _, p := range op.Params {
			if !sharedParams[p.Name] && !seen[p.Name] {
				seen[p.Name] = true
				p.Required = false
				merged.Params = append(merged.Params, p)
			}
		}
	}

	// Build PathVariants sorted by path length descending (longest first).
	merged.PathVariants = make([]PathVariant, len(ops))
	for i, op := range ops {
		merged.PathVariants[i] = PathVariant{
			Path:                op.Path,
			BodyContentType:     op.BodyContentType,
			ResponseContentType: op.ResponseContentType,
			BodySchemaText:      op.BodySchemaText,
			ResponseSchemaText:  op.ResponseSchemaText,
		}
	}
	sort.Slice(merged.PathVariants, func(i, j int) bool {
		return len(merged.PathVariants[i].Path) > len(merged.PathVariants[j].Path)
	})

	// Use the most detailed description/schemas.
	for _, op := range ops {
		if len(op.Description) > len(merged.Description) {
			merged.Description = op.Description
		}
		if op.BodySchemaText != "" && merged.BodySchemaText == "" {
			merged.BodySchemaText = op.BodySchemaText
		}
		if op.ResponseSchemaText != "" && merged.ResponseSchemaText == "" {
			merged.ResponseSchemaText = op.ResponseSchemaText
		}
	}

	return merged, true
}

// deriveFlagName creates a flag name for a synthetic enum parameter by looking
// at adjacent path segments. For example, if the next segment is {updateID},
// the flag name is "update-kind".
func deriveFlagName(segs []string, pos int) string {
	// Look at the next segment.
	if pos+1 < len(segs) {
		next := segs[pos+1]
		if strings.HasPrefix(next, "{") && strings.HasSuffix(next, "}") {
			paramName := next[1 : len(next)-1]
			base := strings.TrimSuffix(strings.TrimSuffix(paramName, "ID"), "Id")
			return toKebab(base) + "-kind"
		}
	}
	// Look at the previous segment.
	if pos-1 >= 0 {
		prev := segs[pos-1]
		if strings.HasPrefix(prev, "{") && strings.HasSuffix(prev, "}") {
			paramName := prev[1 : len(prev)-1]
			base := strings.TrimSuffix(strings.TrimSuffix(paramName, "ID"), "Id")
			return toKebab(base) + "-kind"
		}
	}
	return "kind"
}
