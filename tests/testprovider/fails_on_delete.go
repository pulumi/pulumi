// Copyright 2016, Pulumi Corporation.
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

//go:build !all

package main

import (
	"context"
	"errors"
	"strconv"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:FailsOnDelete"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource fails on delete.",
			Properties:  map[string]pschema.PropertySpec{},
			Type:        "object",
		},
		InputProperties: map[string]pschema.PropertySpec{},
	}
}

type failsOnDeleteProvider struct {
	id int
}

func (p *failsOnDeleteProvider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *failsOnDeleteProvider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{
		Changes: pulumirpc.DiffResponse_DIFF_NONE,
	}, nil
}

func (p *failsOnDeleteProvider) Create(
	ctx context.Context, req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	p.id++
	return &pulumirpc.CreateResponse{
		Id: strconv.Itoa(p.id),
	}, nil
}

func (p *failsOnDeleteProvider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return &pulumirpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *failsOnDeleteProvider) Update(
	ctx context.Context, req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *failsOnDeleteProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return nil, errors.New("Delete always fails for the FailsOnDelete resource")
}

func (p *failsOnDeleteProvider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	// The fails-on-delete provider doesn't support any invokes currently.
	panic("Invoke not implemented")
}

func (p *failsOnDeleteProvider) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	// The random provider doesn't support any call currently.
	panic("Call not implemented")
}
