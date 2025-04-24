// Copyright 2024-2024, Pulumi Corporation.
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

package pulumi

import "golang.org/x/net/context"

// ResourceTransformArgs is the argument bag passed to a resource transform.
type ResourceTransformArgs struct {
	// If the resource is a custom or component resource
	Custom bool
	// The type of the resource.
	Type string
	// The name of the resource.
	Name string
	// The original properties passed to the resource constructor.
	Props Map
	// The original resource options passed to the resource constructor.
	Opts ResourceOptions
}

// ResourceTransformResult is the result that must be returned by a resource transform
// callback. It includes new values to use for the `props` and `opts` of the `Resource` in place of
// the originally provided values.
type ResourceTransformResult struct {
	// The new properties to use in place of the original `props`.
	Props Map
	// The new resource options to use in place of the original `opts`.
	Opts ResourceOptions
}

// ResourceTransform is the callback signature for the `transforms` resource option.  A
// transform is passed the same set of inputs provided to the `Resource` constructor, and can
// optionally return back alternate values for the `props` and/or `opts` prior to the resource
// actually being created.  The effect will be as though those props and opts were passed in place
// of the original call to the `Resource` constructor.  If the transform returns nil,
// this indicates that the resource will not be transformed.
type ResourceTransform func(context.Context, *ResourceTransformArgs) *ResourceTransformResult

// InvokeTransformArgs is the argument bag passed to a invoke transform.
type InvokeTransformArgs struct {
	// The token of the invoke.
	Token string
	// The original args passed to the resource constructor.
	Args Map
	// The original invoke options passed to the resource constructor.
	Opts InvokeOptions
}

// InvokeTransformResult is the result that must be returned by an invoke transform
// callback. It includes new values to use for the `args` and `opts` of the `Invoke` in place of
// the originally provided values.
type InvokeTransformResult struct {
	// The new args to use in place of the original `args`.
	Args Map
	// The new invoke options to use in place of the original `opts`.
	Opts InvokeOptions
}

// InvokeTransform is the callback signature for the `transforms` resource option for invokes.  A
// transform is passed the same set of inputs provided to the `Invoke` constructor, and can
// optionally return back alternate values for the `args` and/or `opts` prior to the invoke
// actually being executed.  The effect will be as though those args and opts were passed in place
// of the original call to the `Invoke`.  If the transform returns nil, this indicates that the Invoke
// will not be transformed.
type InvokeTransform func(context.Context, *InvokeTransformArgs) *InvokeTransformResult
