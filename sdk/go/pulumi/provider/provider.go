// Copyright 2016-2021, Pulumi Corporation.
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

package provider

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/grpc"
)

// This file relies on implementations in ../provider_linked.go that are made available in this package via
// go:linkname.

type ConstructFunc func(ctx *pulumi.Context, typ, name string, inputs ConstructInputs,
	options pulumi.ResourceOption) (*ConstructResult, error)

// Construct adapts the gRPC ConstructRequest/ConstructResponse to/from the Pulumi Go SDK programming model.
func Construct(ctx context.Context, req *pulumirpc.ConstructRequest, engineConn *grpc.ClientConn,
	construct ConstructFunc) (*pulumirpc.ConstructResponse, error) {
	return linkedConstruct(ctx, req, engineConn, func(pulumiCtx *pulumi.Context, typ, name string,
		inputs map[string]interface{}, options pulumi.ResourceOption) (pulumi.URNInput, pulumi.Input, error) {
		ci := ConstructInputs{ctx: pulumiCtx, inputs: inputs}
		result, err := construct(pulumiCtx, typ, name, ci, options)
		if err != nil {
			return nil, nil, err
		}
		return result.URN, result.State, nil
	})
}

// ConstructInputs represents the inputs associated with a call to Construct.
type ConstructInputs struct {
	ctx    *pulumi.Context
	inputs map[string]interface{}
}

// Map returns the inputs as a Map.
func (inputs ConstructInputs) Map() (pulumi.Map, error) {
	return linkedConstructInputsMap(inputs.ctx, inputs.inputs)
}

// CopyTo sets the inputs on the given args struct.
func (inputs ConstructInputs) CopyTo(args interface{}) error {
	return linkedConstructInputsCopyTo(inputs.ctx, inputs.inputs, args)
}

// ConstructResult is the result of a call to Construct.
type ConstructResult struct {
	URN   pulumi.URNInput
	State pulumi.Input
}

// NewConstructResult creates a ConstructResult from the resource.
func NewConstructResult(resource pulumi.ComponentResource) (*ConstructResult, error) {
	urn, state, err := linkedNewConstructResult(resource)
	if err != nil {
		return nil, err
	}
	return &ConstructResult{
		URN:   urn,
		State: state,
	}, nil
}

type CallFunc func(ctx *pulumi.Context, tok string, args CallArgs) (*CallResult, error)

// Call adapts the gRPC CallRequest/CallResponse to/from the Pulumi Go SDK programming model.
func Call(ctx context.Context, req *pulumirpc.CallRequest, engineConn *grpc.ClientConn,
	call CallFunc) (*pulumirpc.CallResponse, error) {
	return linkedCall(ctx, req, engineConn, func(pulumiCtx *pulumi.Context, tok string,
		args map[string]interface{}) (pulumi.Input, error) {
		ca := CallArgs{ctx: pulumiCtx, args: args}
		result, err := call(pulumiCtx, tok, ca)
		if err != nil {
			return nil, err
		}
		return result.Return, nil
	})
}

// CallArgs represents the Call's arguments.
type CallArgs struct {
	ctx  *pulumi.Context
	args map[string]interface{}
}

// Map returns the args as a Map.
func (a CallArgs) Map() (pulumi.Map, error) {
	// Use the same implementation as construct.
	return linkedConstructInputsMap(a.ctx, a.args)
}

// CopyTo sets the args on the given args struct. If there is a `__self__` argument, it will be
// returned, otherwise it will return nil.
func (a CallArgs) CopyTo(args interface{}) (pulumi.Resource, error) {
	return linkedCallArgsCopyTo(a.ctx, a.args, args)
}

// Self retrieves the `__self__` argument. If `__self__` is present the value is returned,
// otherwise the returned value will be nil.
func (a CallArgs) Self() (pulumi.Resource, error) {
	return linkedCallArgsSelf(a.ctx, a.args)
}

// CallResult is the result of the Call.
type CallResult struct {
	Return pulumi.Input
}

// NewCallResult creates a CallResult from the given result.
func NewCallResult(result interface{}) (*CallResult, error) {
	ret, err := linkedNewCallResult(result)
	if err != nil {
		return nil, err
	}
	return &CallResult{
		Return: ret,
	}, nil
}

type constructFunc func(ctx *pulumi.Context, typ, name string, inputs map[string]interface{},
	options pulumi.ResourceOption) (pulumi.URNInput, pulumi.Input, error)

// linkedConstruct is made available here from ../provider_linked.go via go:linkname.
func linkedConstruct(ctx context.Context, req *pulumirpc.ConstructRequest, engineConn *grpc.ClientConn,
	constructF constructFunc) (*pulumirpc.ConstructResponse, error)

// linkedConstructInputsMap is made available here from ../provider_linked.go via go:linkname.
func linkedConstructInputsMap(ctx *pulumi.Context, inputs map[string]interface{}) (pulumi.Map, error)

// linkedConstructInputsCopyTo is made available here from ../provider_linked.go via go:linkname.
func linkedConstructInputsCopyTo(ctx *pulumi.Context, inputs map[string]interface{}, args interface{}) error

// linkedNewConstructResult is made available here from ../provider_linked.go via go:linkname.
func linkedNewConstructResult(resource pulumi.ComponentResource) (pulumi.URNInput, pulumi.Input, error)

type callFunc func(ctx *pulumi.Context, tok string, args map[string]interface{}) (pulumi.Input, error)

// linkedCall is made available here from ../provider_linked.go via go:linkname.
func linkedCall(ctx context.Context, req *pulumirpc.CallRequest, engineConn *grpc.ClientConn,
	callF callFunc) (*pulumirpc.CallResponse, error)

// linkedCallArgsCopyTo is made available here from ../provider_linked.go via go:linkname.
func linkedCallArgsCopyTo(ctx *pulumi.Context, source map[string]interface{},
	args interface{}) (pulumi.Resource, error)

// linkedCallArgsSelf is made available here from ../provider_linked.go via go:linkname.
func linkedCallArgsSelf(ctx *pulumi.Context, source map[string]interface{}) (pulumi.Resource, error)

// linkedNewCallResult is made available here from ../provider_linked.go via go:linkname.
func linkedNewCallResult(result interface{}) (pulumi.Input, error)
