// Copyright 2016-2026, Pulumi Corporation.
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

package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/zclconf/go-cty/cty"
)

// DeclarativeProgram is the JSON format generated from PCL for the REST gateway.
type DeclarativeProgram struct {
	Resources []DeclarativeResource  `json:"resources,omitempty"`
	Invokes   []DeclarativeInvoke    `json:"invokes,omitempty"`
	Outputs   map[string]interface{} `json:"outputs,omitempty"`
}

// DeclarativeResource represents a single resource to register.
type DeclarativeResource struct {
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Custom       bool                   `json:"custom"`
	Parent       string                 `json:"parent,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Options      *DeclarativeOptions    `json:"options,omitempty"`
}

// DeclarativeInvoke represents a function invocation.
type DeclarativeInvoke struct {
	Name  string                 `json:"name"`
	Token string                 `json:"token"`
	Args  map[string]interface{} `json:"args,omitempty"`
}

// DeclarativeOptions holds resource options.
type DeclarativeOptions struct {
	Protect                 *bool    `json:"protect,omitempty"`
	IgnoreChanges           []string `json:"ignoreChanges,omitempty"`
	ReplaceOnChanges        []string `json:"replaceOnChanges,omitempty"`
	DeleteBeforeReplace     *bool    `json:"deleteBeforeReplace,omitempty"`
	AdditionalSecretOutputs []string `json:"additionalSecretOutputs,omitempty"`
	RetainOnDelete          *bool    `json:"retainOnDelete,omitempty"`
	Version                 string   `json:"version,omitempty"`
	PluginDownloadURL       string   `json:"pluginDownloadURL,omitempty"`
	ImportID                string   `json:"import,omitempty"`
	HideDiffs               []string `json:"hideDiffs,omitempty"`
	ReplaceWith             []string `json:"replaceWith,omitempty"`
}

// secretMagicKey is the Pulumi secret sentinel key.
const secretMagicKey = "4dabf18193072939515e22adb298388d"

// secretMagicValue is the corresponding sentinel value.
const secretMagicValue = "1b47061264138c4ac30d75fd1eb44270"

// wrapSecret wraps a value in the Pulumi secret sentinel object.
func wrapSecret(v interface{}) map[string]interface{} {
	return map[string]interface{}{
		secretMagicKey: secretMagicValue,
		"value":        v,
	}
}

// generateProgram converts a bound PCL program into our JSON declarative format.
func generateProgram(program *pcl.Program) (map[string][]byte, hcl.Diagnostics, error) {
	decl := &DeclarativeProgram{
		Outputs: map[string]interface{}{},
	}

	nodes := pcl.Linearize(program)

	for _, node := range nodes {
		switch n := node.(type) {
		case *pcl.Resource:
			res, diags := convertResource(n)
			if diags.HasErrors() {
				return nil, diags, nil
			}
			decl.Resources = append(decl.Resources, res)

		case *pcl.OutputVariable:
			decl.Outputs[n.LogicalName()] = exprToValue(n.Value)

		case *pcl.ConfigVariable, *pcl.LocalVariable, *pcl.PulumiBlock:
			// Config and locals are resolved at runtime via expression references.
			// Nothing to emit in the declarative format for now.
		}
	}

	data, err := json.MarshalIndent(decl, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal program: %w", err)
	}

	return map[string][]byte{
		"program.json": data,
	}, nil, nil
}

// convertResource converts a PCL Resource node to our declarative format.
func convertResource(res *pcl.Resource) (DeclarativeResource, hcl.Diagnostics) {
	// Use the original PCL source token (e.g. "simple:index:Resource")
	// rather than the canonical schema token (e.g. "simple::Resource")
	// because the engine needs the full three-part token.
	resourceType := res.Token
	if res.Definition != nil && len(res.Definition.Labels) >= 2 {
		resourceType = res.Definition.Labels[1]
	}

	isCustom := true
	if res.Schema != nil {
		isCustom = !res.Schema.IsComponent
	}

	dr := DeclarativeResource{
		Name:   res.LogicalName(),
		Type:   resourceType,
		Custom: isCustom,
	}

	// Convert properties from PCL inputs.
	if len(res.Inputs) > 0 {
		dr.Properties = make(map[string]interface{})
		for _, attr := range res.Inputs {
			dr.Properties[attr.Name] = exprToValue(attr.Value)
		}
	}

	// Track dependencies.
	for _, dep := range res.GetDependencies() {
		dr.Dependencies = append(dr.Dependencies, dep.Name())
	}

	// Handle resource options.
	if res.Options != nil {
		opts := convertResourceOptions(res.Options)
		if opts != nil {
			dr.Options = opts
		}
	}

	return dr, nil
}

// convertResourceOptions converts PCL resource options to our declarative format.
func convertResourceOptions(opts *pcl.ResourceOptions) *DeclarativeOptions {
	d := &DeclarativeOptions{}
	hasOpts := false

	if opts.Protect != nil {
		if b, ok := exprToValue(opts.Protect).(bool); ok {
			d.Protect = &b
			hasOpts = true
		}
	}

	if opts.RetainOnDelete != nil {
		if b, ok := exprToValue(opts.RetainOnDelete).(bool); ok {
			d.RetainOnDelete = &b
			hasOpts = true
		}
	}

	if opts.DeleteBeforeReplace != nil {
		if b, ok := exprToValue(opts.DeleteBeforeReplace).(bool); ok {
			d.DeleteBeforeReplace = &b
			hasOpts = true
		}
	}

	if opts.IgnoreChanges != nil {
		d.IgnoreChanges = exprToStringList(opts.IgnoreChanges)
		hasOpts = true
	}

	if opts.ReplaceOnChanges != nil {
		d.ReplaceOnChanges = exprToStringList(opts.ReplaceOnChanges)
		hasOpts = true
	}

	if opts.AdditionalSecretOutputs != nil {
		d.AdditionalSecretOutputs = exprToStringList(opts.AdditionalSecretOutputs)
		hasOpts = true
	}

	if opts.HideDiffs != nil {
		d.HideDiffs = exprToStringList(opts.HideDiffs)
		hasOpts = true
	}

	if opts.Version != nil {
		if s, ok := exprToValue(opts.Version).(string); ok {
			d.Version = s
			hasOpts = true
		}
	}

	if opts.PluginDownloadURL != nil {
		if s, ok := exprToValue(opts.PluginDownloadURL).(string); ok {
			d.PluginDownloadURL = s
			hasOpts = true
		}
	}

	if opts.ImportID != nil {
		if s, ok := exprToValue(opts.ImportID).(string); ok {
			d.ImportID = s
			hasOpts = true
		}
	}

	if opts.ReplaceWith != nil {
		// ReplaceWith is a list of resource references — extract their names as URN references.
		d.ReplaceWith = exprToResourceRefList(opts.ReplaceWith)
		hasOpts = true
	}

	if !hasOpts {
		return nil
	}
	return d
}

// exprToStringList converts a PCL expression to a list of strings.
// Used for ignoreChanges, replaceOnChanges, additionalSecretOutputs, etc.
// These lists contain property names (bare identifiers), not resource references.
func exprToStringList(expr model.Expression) []string {
	tuple, ok := expr.(*model.TupleConsExpression)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range tuple.Expressions {
		switch e := item.(type) {
		case *model.ScopeTraversalExpression:
			// Bare identifier like "value" — use the root name directly as a property path.
			var parts []string
			parts = append(parts, e.RootName)
			for _, t := range e.Traversal[1:] {
				if attr, ok := t.(hcl.TraverseAttr); ok {
					parts = append(parts, attr.Name)
				}
			}
			result = append(result, strings.Join(parts, "."))
		case *model.LiteralValueExpression:
			if e.Value.Type().Equals(cty.String) {
				result = append(result, e.Value.AsString())
			}
		default:
			if s, ok := exprToValue(item).(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}

// exprToResourceRefList converts a PCL expression to a list of "${name}" references.
// Used for replaceWith which references other resources.
func exprToResourceRefList(expr model.Expression) []string {
	val := exprToValue(expr)
	arr, ok := val.([]interface{})
	if !ok {
		// Single reference.
		if s, ok := val.(string); ok {
			return []string{s}
		}
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// exprToValue converts a PCL expression to a JSON-compatible value.
// For simple literals it returns the value directly; for references it returns "${name.field}".
func exprToValue(expr model.Expression) interface{} {
	switch e := expr.(type) {
	case *model.LiteralValueExpression:
		return ctyToValue(e.Value)

	case *model.TemplateExpression:
		if len(e.Parts) == 1 {
			// Single part: might be a literal or reference.
			return exprToValue(e.Parts[0])
		}
		// Multi-part template: build interpolated string.
		var sb strings.Builder
		for _, part := range e.Parts {
			switch p := part.(type) {
			case *model.LiteralValueExpression:
				if p.Value.Type().Equals(cty.String) {
					sb.WriteString(p.Value.AsString())
				}
			default:
				sb.WriteString(exprToRef(p))
			}
		}
		return sb.String()

	case *model.ScopeTraversalExpression:
		return exprToRef(e)

	case *model.ObjectConsExpression:
		obj := make(map[string]interface{})
		for _, item := range e.Items {
			key := exprToValue(item.Key)
			keyStr, ok := key.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", key)
			}
			obj[keyStr] = exprToValue(item.Value)
		}
		return obj

	case *model.TupleConsExpression:
		arr := make([]interface{}, len(e.Expressions))
		for i, item := range e.Expressions {
			arr[i] = exprToValue(item)
		}
		return arr

	case *model.FunctionCallExpression:
		return convertFunctionCall(e)

	case *model.RelativeTraversalExpression:
		return exprToRef(e)

	default:
		return exprToRef(expr)
	}
}

// ctyToValue converts a cty.Value to a Go value suitable for JSON serialization.
func ctyToValue(v cty.Value) interface{} {
	if !v.IsKnown() {
		return nil
	}

	ty := v.Type()
	switch {
	case ty.Equals(cty.Bool):
		return v.True()
	case ty.Equals(cty.Number):
		bf := v.AsBigFloat()
		return bigFloatToValue(bf)
	case ty.Equals(cty.String):
		return v.AsString()
	case ty.IsListType() || ty.IsTupleType() || ty.IsSetType():
		var result []interface{}
		for it := v.ElementIterator(); it.Next(); {
			_, val := it.Element()
			result = append(result, ctyToValue(val))
		}
		return result
	case ty.IsMapType() || ty.IsObjectType():
		result := make(map[string]interface{})
		for it := v.ElementIterator(); it.Next(); {
			key, val := it.Element()
			result[key.AsString()] = ctyToValue(val)
		}
		return result
	case ty.Equals(cty.DynamicPseudoType):
		return nil
	default:
		return v.GoString()
	}
}

// bigFloatToValue converts a *big.Float to an appropriate Go numeric value.
// It preserves integer representation when possible and uses json.Number
// for extreme float values to avoid precision loss.
func bigFloatToValue(bf *big.Float) interface{} {
	if bf.IsInt() {
		i, _ := bf.Int64()
		return i
	}
	// For extreme float values, use the text representation to avoid precision loss.
	f, accuracy := bf.Float64()
	if accuracy == big.Exact {
		return f
	}
	// Use the full-precision text form via json.Number.
	return json.Number(bf.Text('e', -1))
}

// convertFunctionCall handles PCL function calls like secret(), invoke(), etc.
func convertFunctionCall(e *model.FunctionCallExpression) interface{} {
	switch e.Name {
	case "secret":
		// secret(value) → wrap in Pulumi's secret sentinel.
		if len(e.Args) > 0 {
			return wrapSecret(exprToValue(e.Args[0]))
		}
		return exprToRef(e)

	case pcl.Invoke:
		// invoke("token", {args}) → reference for resolution at runtime.
		return exprToRef(e)

	default:
		return exprToRef(e)
	}
}

// exprToRef produces a "${...}" reference string for expressions that can't be
// fully resolved at generation time (resource outputs, invokes, etc.).
func exprToRef(expr model.Expression) string {
	switch e := expr.(type) {
	case *model.ScopeTraversalExpression:
		var parts []string
		parts = append(parts, e.RootName)
		for _, t := range e.Traversal[1:] {
			switch tt := t.(type) {
			case hcl.TraverseAttr:
				parts = append(parts, tt.Name)
			case hcl.TraverseIndex:
				parts = append(parts, fmt.Sprintf("[%s]", tt.Key.GoString()))
			}
		}
		return "${" + strings.Join(parts, ".") + "}"

	case *model.RelativeTraversalExpression:
		src := exprToRef(e.Source)
		// Strip the outer ${} if present to chain traversals.
		src = strings.TrimPrefix(src, "${")
		src = strings.TrimSuffix(src, "}")
		for _, t := range e.Traversal {
			switch tt := t.(type) {
			case hcl.TraverseAttr:
				src += "." + tt.Name
			}
		}
		return "${" + src + "}"

	default:
		// Fallback: use the expression's string representation.
		return fmt.Sprintf("${%s}", expr.SyntaxNode().Range())
	}
}
