// Copyright 2016-2023, Pulumi Corporation.
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
	"errors"

	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pbempty "github.com/golang/protobuf/ptypes/empty"
)

type failsOnCreateResourceProvider struct{}

func (p *failsOnCreateResourceProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *failsOnCreateResourceProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	return &rpc.DiffResponse{
		Changes: rpc.DiffResponse_DIFF_NONE,
	}, nil
}

func (p *failsOnCreateResourceProvider) Create(
	ctx context.Context, req *rpc.CreateRequest,
) (*rpc.CreateResponse, error) {
	return nil, errors.New("Create always fails for the FailsOnCreate resource")
}

func (p *failsOnCreateResourceProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *failsOnCreateResourceProvider) Update(
	ctx context.Context, req *rpc.UpdateRequest,
) (*rpc.UpdateResponse, error) {
	panic("Update not implemented")
}

func (p *failsOnCreateResourceProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}
