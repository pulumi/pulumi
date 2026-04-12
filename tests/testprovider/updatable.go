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

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Updatable"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that supports updates and reports changed property keys.",
			Properties: map[string]pschema.PropertySpec{
				"foo": {TypeSpec: pschema.TypeSpec{Type: "string"}},
				"bar": {TypeSpec: pschema.TypeSpec{Type: "string"}},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"foo": {TypeSpec: pschema.TypeSpec{Type: "string"}},
			"bar": {TypeSpec: pschema.TypeSpec{Type: "string"}},
		},
	}
}

type updatableProvider struct {
	id int
}

func (p *updatableProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

// Diff compares old and new inputs and reports changed top-level keys via the proto `diffs` field.
// This is what `pulumi preview --json --changes-only` relies on to know which inputs to surface.
func (p *updatableProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}
	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	d := olds.Diff(news)
	changes := rpc.DiffResponse_DIFF_NONE
	var diffs []string
	if d != nil {
		for _, k := range d.ChangedKeys() {
			diffs = append(diffs, string(k))
		}
		if len(diffs) > 0 {
			changes = rpc.DiffResponse_DIFF_SOME
		}
	}

	return &rpc.DiffResponse{
		Changes: changes,
		Diffs:   diffs,
	}, nil
}

func (p *updatableProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	if err != nil {
		return nil, err
	}
	outputProperties, err := plugin.MarshalProperties(
		inputs,
		plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true},
	)
	if err != nil {
		return nil, err
	}
	p.id++
	return &rpc.CreateResponse{
		Id:         strconv.Itoa(p.id),
		Properties: outputProperties,
	}, nil
}

func (p *updatableProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *updatableProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	if err != nil {
		return nil, err
	}
	outputProperties, err := plugin.MarshalProperties(
		news,
		plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true},
	)
	if err != nil {
		return nil, err
	}
	return &rpc.UpdateResponse{Properties: outputProperties}, nil
}

func (p *updatableProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *updatableProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	panic("Invoke not implemented")
}

func (p *updatableProvider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	panic("Call not implemented")
}
