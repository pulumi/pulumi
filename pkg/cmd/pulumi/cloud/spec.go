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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// ParamSpec describes a single parameter for an API operation.
// JSON tags are part of the `describe --output=json` envelope contract
// (SchemaVersion 1).
type ParamSpec struct {
	Name        string   `json:"name"`
	In          string   `json:"in"` // "path", "query", "header"
	Type        string   `json:"type"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Values      []string `json:"values,omitempty"`
}

// ResponseSpec describes one response code that carries a schema. Rendered in
// both text and markdown form at parse time so downstream renderers stay cheap.
type ResponseSpec struct {
	Status         string
	Description    string
	ContentType    string
	SchemaText     string
	SchemaMarkdown string
	SchemaJSON     json.RawMessage
}

// ErrorRef describes a response code that has no schema (a "stock error" in
// docs terminology). Rendered as a compact chip row rather than a full block.
type ErrorRef struct {
	Status      string
	Description string
}

// Operation describes an API operation parsed from the OpenAPI spec.
// It is the flat, per-endpoint shape consumed by the raw dispatcher, ls,
// and describe.
type Operation struct {
	OperationID         string
	Method              string
	Path                string
	Summary             string
	Description         string
	Tag                 string
	Params              []ParamSpec
	HasBody             bool
	BodyContentType     string
	ResponseContentType string
	// SuccessContentTypes lists every content type declared on the primary
	// 2xx response, in spec order. An endpoint that offers both
	// `application/json` and `text/markdown` will carry both, which lets the
	// dispatcher drive content negotiation via --output. The legacy
	// ResponseContentType field is still populated with the first preferred
	// entry so human/JSON describe output stays stable.
	SuccessContentTypes []string
	BodySchemaText      string
	ResponseSchemaText  string
	// Request/response schemas serialized with all $refs inlined, so `describe --output=json`
	// consumers can walk the structure programmatically.
	BodySchemaJSON     json.RawMessage
	ResponseSchemaJSON json.RawMessage
	// Markdown-rendered request body schema for --output=markdown and the TUI.
	BodySchemaMarkdown string
	// InlineResponses carries every response that has a schema (both success and
	// error). The primary success response is duplicated into the flat
	// ResponseContentType/ResponseSchemaText/ResponseSchemaJSON fields above so
	// the existing --output=human and --output=json contracts stay stable.
	InlineResponses []ResponseSpec
	// StockErrors lists response codes with no schema — docs renders these as a
	// compact chip row.
	StockErrors []ErrorRef
	// IsPreview is true when x-pulumi-route-property.Visibility is "Preview".
	IsPreview bool
	// IsDeprecated is true when x-pulumi-route-property.Deprecated is true
	// or when the operation uses OpenAPI's native `deprecated: true` field.
	IsDeprecated bool
	// SupersededBy is the operationId of the replacement route when
	// x-pulumi-route-property.SupersededBy is set on a deprecated op.
	SupersededBy string
}

// Index is the parsed OpenAPI spec with lookup aids.
type Index struct {
	Operations  []*Operation
	ByKey       map[string]*Operation // "GET /api/user" -> op
	SpecVersion string                // the spec's own info.version, surfaced in `ls --output=json`
}

// methodPrecedence defines the method order used for stable sorting of operations.
var methodPrecedence = map[string]int{
	"GET": 0, "POST": 1, "PUT": 2, "PATCH": 3, "DELETE": 4, "HEAD": 5, "OPTIONS": 6,
}

// LoadIndex returns the parsed OpenAPI spec, fetching it from Pulumi Cloud on
// cache miss and writing the result to the per-user cache. Pass refresh=true
// to force a re-fetch even when the cache has a copy. Warnings (stale-cache
// fallback, cache-write failure) are written to warnW.
func LoadIndex(ctx context.Context, warnW io.Writer, refresh bool) (*Index, error) {
	data, err := ensureSpec(ctx, warnW, refresh)
	if err != nil {
		return nil, err
	}
	return parseIndex(data)
}

// Key returns the canonical lookup key "METHOD path" for an operation.
func (o *Operation) Key() string {
	return o.Method + " " + o.Path
}

func parseIndex(data []byte) (*Index, error) {
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrSpecParse,
			fmt.Sprintf("parsing OpenAPI document: %v", err))
	}
	v3Model, errs := doc.BuildV3Model()
	if errs != nil && v3Model == nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrSpecParse,
			fmt.Sprintf("building OpenAPI v3 model: %v", errs))
	}
	model := v3Model.Model

	renderer := newSchemaRenderer(model.Components.Schemas)

	var ops []*Operation
	if model.Paths != nil && model.Paths.PathItems != nil {
		for path, pathItem := range model.Paths.PathItems.FromOldest() {
			for _, mo := range httpMethodOps(pathItem) {
				op := mo.Op
				if op == nil || op.OperationId == "" {
					continue
				}
				// Drop Visibility: Internal endpoints. The server only returns
				// them to site admins; filter defensively so those cases still
				// see the public view.
				if isInternalOp(op) {
					continue
				}
				ops = append(ops, parseOperation(path, mo.Method, op, renderer))
			}
		}
	}

	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Tag != ops[j].Tag {
			return ops[i].Tag < ops[j].Tag
		}
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return methodPrecedence[ops[i].Method] < methodPrecedence[ops[j].Method]
	})

	byKey := make(map[string]*Operation, len(ops))
	for _, op := range ops {
		byKey[op.Key()] = op
	}

	specVersion := ""
	if model.Info != nil {
		specVersion = model.Info.Version
	}

	return &Index{
		Operations:  ops,
		ByKey:       byKey,
		SpecVersion: specVersion,
	}, nil
}

func parseOperation(path, method string, op *v3high.Operation, renderer *schemaRenderer) *Operation {
	// Prefer the second tag when the spec provides one. The Pulumi Cloud
	// OpenAPI uses tags[1] specifically to reorganize ops whose top-level
	// bucket is too broad — it matches how pulumi.com/docs groups them.
	// Examples from the spec:
	//   ListOrgTokens     tags=["Organizations", "AccessTokens"]
	//   ListOrgAgentPool  tags=["Workflows",     "DeploymentRunners"]
	//   ListPolicyViolationsV2 tags=["Organizations", "PolicyResults"]
	// Surfacing these under "AccessTokens" / "DeploymentRunners" / etc.
	// matches the docs site rather than burying them under generic
	// "Organizations" / "Workflows" headings.
	tag := "Miscellaneous"
	switch {
	case len(op.Tags) >= 2:
		tag = op.Tags[1]
	case len(op.Tags) == 1:
		tag = op.Tags[0]
	}
	out := &Operation{
		OperationID: op.OperationId,
		Method:      method,
		Path:        path,
		Summary:     op.Summary,
		Description: op.Description,
		Tag:         tag,
	}
	applyRouteProperty(out, op)
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
			if resolved := p.Schema.Schema(); resolved != nil {
				pp.Type = schemaType(resolved)
			}
		}
		if pp.Type == "" {
			pp.Type = "string"
		}
		out.Params = append(out.Params, pp)
	}
	if op.RequestBody != nil && op.RequestBody.Content != nil {
		out.HasBody = true
		out.BodyContentType = preferContentType(op.RequestBody.Content)
		if out.BodyContentType != "" {
			if ct, ok := op.RequestBody.Content.Get(out.BodyContentType); ok && ct != nil && ct.Schema != nil {
				if resolved := ct.Schema.Schema(); resolved != nil {
					out.BodySchemaText = renderer.renderBodySchema(resolved)
					out.BodySchemaMarkdown = renderer.renderSchemaMarkdown(resolved)
					out.BodySchemaJSON = schemaToJSON(resolved)
				}
			}
		}
	}
	populateResponses(out, op, renderer)
	return out
}

// populateResponses walks every response code on op and buckets them into
// `InlineResponses` (has a schema) vs `StockErrors` (no schema). The first
// 2xx success response is also duplicated into the flat
// ResponseContentType/ResponseSchemaText/ResponseSchemaJSON fields to preserve
// the existing `describe --output=human|json` contracts.
func populateResponses(out *Operation, op *v3high.Operation, renderer *schemaRenderer) {
	if op.Responses == nil {
		return
	}

	type codeEntry struct {
		status string
		resp   *v3high.Response
	}
	var entries []codeEntry
	if op.Responses.Codes != nil {
		for code, resp := range op.Responses.Codes.FromOldest() {
			entries = append(entries, codeEntry{status: code, resp: resp})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].status < entries[j].status })
	if op.Responses.Default != nil {
		entries = append(entries, codeEntry{status: "default", resp: op.Responses.Default})
	}

	for _, e := range entries {
		if e.resp == nil {
			continue
		}
		ct := preferContentType(e.resp.Content)
		var resolved *base.Schema
		if ct != "" && e.resp.Content != nil {
			if media, ok := e.resp.Content.Get(ct); ok && media != nil && media.Schema != nil {
				resolved = media.Schema.Schema()
			}
		}

		if resolved == nil {
			// A schemaless 2xx is a valid "success without body" response (204,
			// sometimes 202). Keep it in InlineResponses so the Responses section
			// still shows it. A schemaless 4xx/5xx goes to StockErrors since there
			// are usually many, and docs renders them as a compact chip row.
			if isSuccessStatus(e.status) {
				out.InlineResponses = append(out.InlineResponses, ResponseSpec{
					Status:      e.status,
					Description: e.resp.Description,
				})
			} else {
				out.StockErrors = append(out.StockErrors, ErrorRef{
					Status:      e.status,
					Description: e.resp.Description,
				})
			}
			continue
		}

		spec := ResponseSpec{
			Status:         e.status,
			Description:    e.resp.Description,
			ContentType:    ct,
			SchemaText:     renderer.renderResponseSchema(resolved),
			SchemaMarkdown: renderer.renderSchemaMarkdown(resolved),
			SchemaJSON:     schemaToJSON(resolved),
		}
		out.InlineResponses = append(out.InlineResponses, spec)

		// Mirror the first success response into the flat fields so the existing
		// --output=human / --output=json contracts still see the "primary" body.
		// Also snapshot the full list of content types the spec declares for
		// that response — the dispatcher uses it to drive --output-based
		// content negotiation.
		if out.ResponseSchemaText == "" && isSuccessStatus(e.status) {
			out.ResponseContentType = spec.ContentType
			out.ResponseSchemaText = spec.SchemaText
			out.ResponseSchemaJSON = spec.SchemaJSON
			out.SuccessContentTypes = contentTypes(e.resp.Content)
		}
	}

	// Fallback: if no success response had a schema, prefer the `default` block
	// so --output=human/json still surface something — matches prior behavior.
	if out.ResponseSchemaText == "" {
		for _, spec := range out.InlineResponses {
			if spec.Status == "default" {
				out.ResponseContentType = spec.ContentType
				out.ResponseSchemaText = spec.SchemaText
				out.ResponseSchemaJSON = spec.SchemaJSON
				break
			}
		}
	}
}

func isSuccessStatus(s string) bool {
	return len(s) == 3 && s[0] == '2'
}

// isInternalOp reports whether op has `x-pulumi-route-property.Visibility`
// set to "Internal". The server only returns Internal ops to site admins;
// client-side filtering keeps those callers on the same public view.
func isInternalOp(op *v3high.Operation) bool {
	if op.Extensions == nil {
		return false
	}
	node := op.Extensions.GetOrZero("x-pulumi-route-property")
	if node == nil {
		return false
	}
	var rp struct {
		Visibility string `yaml:"Visibility" json:"Visibility"`
	}
	if err := node.Decode(&rp); err != nil {
		return false
	}
	return strings.EqualFold(rp.Visibility, "Internal")
}

// applyRouteProperty reads the Pulumi-specific x-pulumi-route-property
// extension and folds its Visibility/Deprecated/SupersededBy fields into
// the Operation. The spec also has a native `deprecated: true` bool which
// we honor as an override so pre-extension ops still surface correctly.
func applyRouteProperty(out *Operation, op *v3high.Operation) {
	if op.Extensions != nil {
		if node := op.Extensions.GetOrZero("x-pulumi-route-property"); node != nil {
			var rp struct {
				Visibility   string `yaml:"Visibility"   json:"Visibility"`
				Deprecated   bool   `yaml:"Deprecated"   json:"Deprecated"`
				SupersededBy string `yaml:"SupersededBy" json:"SupersededBy"`
			}
			if err := node.Decode(&rp); err == nil {
				if strings.EqualFold(rp.Visibility, "Preview") {
					out.IsPreview = true
				}
				if rp.Deprecated {
					out.IsDeprecated = true
				}
				out.SupersededBy = rp.SupersededBy
			}
		}
	}
	if op.Deprecated != nil && *op.Deprecated {
		out.IsDeprecated = true
	}
}

// schemaToJSON serializes a resolved libopenapi schema to JSON with all $refs
// inlined. Returns nil on failure — callers should treat the jsonSchema field
// as optional.
func schemaToJSON(s *base.Schema) json.RawMessage {
	if s == nil {
		return nil
	}
	raw, err := s.MarshalJSONInline()
	if err != nil {
		return nil
	}
	return json.RawMessage(raw)
}

// httpMethodOps enumerates (method, operation) pairs on a PathItem in a fixed order.
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

// contentTypePreference is the ordered preference for picking a content type from a media-type map.
var contentTypePreference = []string{
	"application/json",
	"application/x-yaml",
}

// contentTypes returns every content-type key declared on content, in spec order.
func contentTypes(content *orderedmap.Map[string, *v3high.MediaType]) []string {
	if content == nil {
		return nil
	}
	var out []string
	for ct := range content.FromOldest() {
		out = append(out, ct)
	}
	return out
}

// preferContentType picks the best content type key from an ordered map of media types.
func preferContentType(content *orderedmap.Map[string, *v3high.MediaType]) string {
	if content == nil {
		return ""
	}
	for _, preferred := range contentTypePreference {
		if _, ok := content.Get(preferred); ok {
			return preferred
		}
	}
	for ct := range content.FromOldest() {
		return ct
	}
	return ""
}
