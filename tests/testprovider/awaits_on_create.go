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

//go:build !all

package main

import (
	"context"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:AwaitsOnCreate"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource whose Create always returns an await failure.",
			Properties:  map[string]pschema.PropertySpec{},
			Type:        "object",
		},
		InputProperties: map[string]pschema.PropertySpec{},
	}
}

type awaitsOnCreateProvider struct{}

func (p *awaitsOnCreateProvider) Check(
	ctx context.Context, req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *awaitsOnCreateProvider) Diff(
	ctx context.Context, req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{Changes: pulumirpc.DiffResponse_DIFF_NONE}, nil
}

func (p *awaitsOnCreateProvider) Create(
	ctx context.Context, req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	detail := &pulumirpc.ErrorResourceAwaitFailed{}
	return nil, rpcerror.WithDetails(rpcerror.New(codes.Unknown, "await failed"), detail)
}

func (p *awaitsOnCreateProvider) Read(
	ctx context.Context, req *pulumirpc.ReadRequest,
) (*pulumirpc.ReadResponse, error) {
	return &pulumirpc.ReadResponse{Id: req.Id, Properties: req.Properties}, nil
}

func (p *awaitsOnCreateProvider) Update(
	ctx context.Context, req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *awaitsOnCreateProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *awaitsOnCreateProvider) Invoke(
	ctx context.Context, req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}

func (p *awaitsOnCreateProvider) Call(
	ctx context.Context, req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	panic("Call not implemented")
}
