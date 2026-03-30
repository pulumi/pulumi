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
	Protect         *bool    `json:"protect,omitempty"`
	IgnoreChanges   []string `json:"ignoreChanges,omitempty"`
	ReplaceOnChanges []string `json:"replaceOnChanges,omitempty"`
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
		opts := &DeclarativeOptions{}
		hasOpts := false

		if res.Options.Protect != nil {
			val := exprToValue(res.Options.Protect)
			if b, ok := val.(bool); ok {
				opts.Protect = &b
				hasOpts = true
			}
		}
		if res.Options.IgnoreChanges != nil {
			if arr, ok := exprToValue(res.Options.IgnoreChanges).([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						opts.IgnoreChanges = append(opts.IgnoreChanges, s)
					}
				}
				hasOpts = true
			}
		}

		if hasOpts {
			dr.Options = opts
		}
	}

	return dr, nil
}

// exprToValue converts a PCL expression to a JSON-compatible value.
// For simple literals it returns the value directly; for references it returns "${name.field}".
func exprToValue(expr model.Expression) interface{} {
	switch e := expr.(type) {
	case *model.LiteralValueExpression:
		ty := e.Value.Type()
		switch {
		case ty.Equals(cty.Bool):
			return e.Value.True()
		case ty.Equals(cty.Number):
			bf := e.Value.AsBigFloat()
			if bf.IsInt() {
				i, _ := bf.Int64()
				return i
			}
			f, _ := bf.Float64()
			return f
		case ty.Equals(cty.String):
			return e.Value.AsString()
		default:
			return e.Value.GoString()
		}

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
		if e.Name == pcl.Invoke {
			// Invoke calls are handled separately.
			return exprToRef(e)
		}
		return exprToRef(e)

	case *model.RelativeTraversalExpression:
		return exprToRef(e)

	default:
		return exprToRef(expr)
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
