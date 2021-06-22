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
