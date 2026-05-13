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
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/model"
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

	invoke      func(context.Context, *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)
	call        func(context.Context, *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)
	getResource func(context.Context, resource.ResourceReference) (resource.PropertyMap, error)

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
