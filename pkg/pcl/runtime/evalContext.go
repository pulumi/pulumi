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

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
	"github.com/pulumi/pulumi/pkg/v3/codegen/pcl"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/zclconf/go-cty/cty"
)

type EvalContext struct {
	workingDirectory string
	rootDirectory    string
	organization     string
	project          string
	stack            string

	lookupResource func(context.Context, string) (*schema.Resource, error)
	lookupFunction func(context.Context, string) (*schema.Function, error)

	invoke         func(context.Context, *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)
	call           func(context.Context, *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)
	getResource    func(context.Context, resource.ResourceReference) (resource.PropertyMap, error)
	existsResource func(context.Context, *pulumirpc.ExistsResourceRequest) (*pulumirpc.ExistsResourceResponse, error)

	// we write variables to the hcl.EvalContext in parallel during execution, so we need to synchronize access to it
	evalLock    sync.Mutex
	evalContext *hcl.EvalContext
}

func NewEvalContext(
	workingDirectory, rootDirectory, organization, project, stack string,
	lookupResource func(context.Context, string) (*schema.Resource, error),
	lookupFunction func(context.Context, string) (*schema.Function, error),
	getResource func(context.Context, resource.ResourceReference) (resource.PropertyMap, error),
	invoke func(context.Context, *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error),
	call func(context.Context, *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error),
	existsResource func(context.Context, *pulumirpc.ExistsResourceRequest) (*pulumirpc.ExistsResourceResponse, error),
) *EvalContext {
	ctx := &EvalContext{
		workingDirectory: workingDirectory,
		rootDirectory:    rootDirectory,
		organization:     organization,
		project:          project,
		stack:            stack,
		lookupResource:   lookupResource,
		lookupFunction:   lookupFunction,
		getResource:      getResource,
		invoke:           invoke,
		call:             call,
		existsResource:   existsResource,
	}

	ctx.evalContext = &hcl.EvalContext{
		Functions: ctx.builtinFunctions(),
	}

	return ctx
}

func (ectx *EvalContext) NewChild() *EvalContext {
	ectx.evalLock.Lock()
	defer ectx.evalLock.Unlock()
	child := ectx.evalContext.NewChild()
	return &EvalContext{
		workingDirectory: ectx.workingDirectory,
		rootDirectory:    ectx.rootDirectory,
		organization:     ectx.organization,
		project:          ectx.project,
		stack:            ectx.stack,
		lookupResource:   ectx.lookupResource,
		lookupFunction:   ectx.lookupFunction,
		getResource:      ectx.getResource,
		invoke:           ectx.invoke,
		call:             ectx.call,
		existsResource:   ectx.existsResource,
		evalContext:      child,
	}
}

func (ectx *EvalContext) SetVariable(name string, value cty.Value) {
	ectx.evalLock.Lock()
	defer ectx.evalLock.Unlock()
	if ectx.evalContext.Variables == nil {
		ectx.evalContext.Variables = make(map[string]cty.Value)
	}
	ectx.evalContext.Variables[name] = value
}

// Evaluate evaluates an expression in the context of the interpreter's evalContext and returns a PropertyValue. If the
// expression evaluates to a poisoned value, the culprit resource's name will be returned in the second return value. If
// there are any errors during evaluation, they will be returned in the diagnostics.
func (ectx *EvalContext) Evaluate(expr model.Expression) (resource.PropertyValue, *string, hcl.Diagnostics) {
	ectx.evalLock.Lock()
	defer ectx.evalLock.Unlock()
	value, diags := expr.Evaluate(ectx.evalContext)

	if diags.HasErrors() {
		return resource.PropertyValue{}, nil, diags
	}
	pv, err := ctyToPropertyValue(value)
	if err != nil {
		var poison *poisonError
		if errors.As(err, &poison) {
			return resource.PropertyValue{}, &poison.name, nil
		}
		diags = append(diags, &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		})
		return resource.PropertyValue{}, nil, diags
	}
	return pv, nil, diags
}

// EvaluateObject evaluates a list of bound attributes against a target input type and schema. If any attribute
// evaluates to a poisoned value (e.g. an upstream resource that failed to create) the poison name is returned in the
// second result and evaluation stops early. Callers decide whether to propagate the poison (the interpreter, so that
// dependents are skipped) or surface it as an error (one-shot callers like `pulumi do`).
func (ectx *EvalContext) EvaluateObject(
	attrs []*model.Attribute, inputType model.Type, properties []*schema.Property,
) (resource.PropertyMap, *string, hcl.Diagnostics) {
	values := resource.PropertyMap{}
	var diagnostics hcl.Diagnostics

	// Look up the per-attribute target type from inputType once, unwrapping any optional wrapper. We want the same
	// model.Type instances the binder built so that RewriteConversions can use pointer identity where it matters.
	var inputProperties map[string]model.Type
	inputType = pcl.UnwrapOption(inputType)
	if obj, ok := inputType.(*model.ObjectType); ok {
		inputProperties = obj.Properties
	}

	for _, attr := range attrs {
		targetType := attr.Value.Type()
		if t, ok := inputProperties[attr.Name]; ok {
			targetType = t
		}

		expr, diags := pcl.RewriteConversions(attr.Value, targetType)
		diagnostics = append(diagnostics, diags...)
		if diags.HasErrors() {
			continue
		}

		value, poison, diags := ectx.Evaluate(expr)
		diagnostics = append(diagnostics, diags...)
		if diags.HasErrors() {
			continue
		}
		if poison != nil {
			// Stop evaluating further attributes — the caller will decide what to do with the poison.
			return nil, poison, diagnostics
		}
		values[resource.PropertyKey(attr.Name)] = collapseResourceReferences(value)
	}

	values, err := applySchemaInputs(values, properties)
	if err != nil {
		// Subject is the start and end of the attributes
		var rng hcl.Range
		for _, attr := range attrs {
			attrRng := attr.Syntax.Range()
			if attrRng.Start.Byte < rng.Start.Byte {
				rng.Start = attrRng.Start
			}
			if attrRng.End.Byte > rng.End.Byte {
				rng.End = attrRng.End
			}
		}
		diag := &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Subject:  &rng,
			Summary:  fmt.Sprintf("apply schema inputs: %v", err),
		}
		diagnostics = append(diagnostics, diag)
		return nil, nil, diagnostics
	}

	return values, nil, diagnostics
}
