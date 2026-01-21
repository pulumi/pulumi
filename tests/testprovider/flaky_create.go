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
// +build !all

package main

import (
	"context"
	"strconv"
	"sync"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc/codes"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:FlakyCreate"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource fails once on create and then succeeds.",
			Properties:  map[string]pschema.PropertySpec{},
			Type:        "object",
		},
		InputProperties: map[string]pschema.PropertySpec{},
	}
}

type flakyCreateProvider struct {
	mu             sync.Mutex
	id             int
	createAttempts int
	lastID         string
}

func (p *flakyCreateProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *flakyCreateProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{
		Changes: rpc.DiffResponse_DIFF_NONE,
	}, nil
}

func (p *flakyCreateProvider) Create(
	ctx context.Context, req *rpc.CreateRequest,
) (*rpc.CreateResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.id++
	p.lastID = strconv.Itoa(p.id)
	p.createAttempts++
	if p.createAttempts == 1 {
		reasons := []string{"Create failed once for the FlakyCreate resource"}
		detail := rpc.ErrorResourceInitFailed{
			Id:      p.lastID,
			Reasons: reasons,
		}
		return nil, rpcerror.WithDetails(rpcerror.New(codes.Unknown, reasons[0]), &detail)
	}

	return &rpc.CreateResponse{
		Id: p.lastID,
	}, nil
}

func (p *flakyCreateProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *flakyCreateProvider) Update(
	ctx context.Context, req *rpc.UpdateRequest,
) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *flakyCreateProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *flakyCreateProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}

func (p *flakyCreateProvider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	panic("Call not implemented")
}
