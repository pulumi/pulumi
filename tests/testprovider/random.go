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
//go:build !all
// +build !all

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	rpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:Random"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource that generates a random string of a given length and with an optional prefix.",
			Properties: map[string]pschema.PropertySpec{
				"length": {
					TypeSpec:    pschema.TypeSpec{Type: "integer"},
					Description: "The length of the random string (not including the prefix, if any).",
				},
				"prefix": {
					TypeSpec:    pschema.TypeSpec{Type: "string"},
					Description: "An optional prefix.",
				},
				"result": {
					TypeSpec:    pschema.TypeSpec{Type: "string"},
					Description: "A random string.",
				},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"length": {
				TypeSpec:    pschema.TypeSpec{Type: "integer"},
				Description: "The length of the random string (not including the prefix, if any).",
			},
			"prefix": {
				TypeSpec:    pschema.TypeSpec{Type: "string"},
				Description: "An optional prefix.",
			},
		},
	}
}

type randomProvider struct{}

func (p *randomProvider) Check(ctx context.Context, req *rpc.CheckRequest) (*rpc.CheckResponse, error) {
	return &rpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *randomProvider) Diff(ctx context.Context, req *rpc.DiffRequest) (*rpc.DiffResponse, error) {
	olds, err := plugin.UnmarshalProperties(req.GetOlds(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	news, err := plugin.UnmarshalProperties(req.GetNews(), plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true})
	if err != nil {
		return nil, err
	}

	d := olds.Diff(news)
	var replaces []string
	changes := rpc.DiffResponse_DIFF_NONE
	if d.Changed("length") {
		changes = rpc.DiffResponse_DIFF_SOME
		replaces = append(replaces, "length")
	}

	if d.Changed("prefix") {
		changes = rpc.DiffResponse_DIFF_SOME
		replaces = append(replaces, "prefix")
	}

	return &rpc.DiffResponse{
		Changes:  changes,
		Replaces: replaces,
	}, nil
}

func (p *randomProvider) Create(ctx context.Context, req *rpc.CreateRequest) (*rpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{
		KeepUnknowns: true,
		SkipNulls:    true,
	})
	if err != nil {
		return nil, err
	}

	if !inputs["length"].IsNumber() {
		return nil, fmt.Errorf("expected input property 'length' of type 'number' but got '%s", inputs["length"].TypeString())
	}

	n := int(inputs["length"].NumberValue())

	var prefix string
	if p, has := inputs["prefix"]; has {
		if !p.IsString() {
			return nil, fmt.Errorf("expected input property 'prefix' of type 'string' but got '%s", p.TypeString())
		}
		prefix = p.StringValue()
	}

	// Actually "create" the random number
	result, err := makeRandom(n)
	if err != nil {
		return nil, err
	}

	outputs := resource.NewPropertyMapFromMap(map[string]interface{}{
		"length": n,
		"result": prefix + result,
	})
	if prefix != "" {
		outputs["prefix"] = resource.NewStringProperty(prefix)
	}
	outputs["result"] = resource.MakeSecret(outputs["result"])

	outputProperties, err := plugin.MarshalProperties(
		outputs,
		plugin.MarshalOptions{KeepUnknowns: true, SkipNulls: true},
	)
	if err != nil {
		return nil, err
	}
	return &rpc.CreateResponse{
		Id:         result,
		Properties: outputProperties,
	}, nil
}

func (p *randomProvider) Read(ctx context.Context, req *rpc.ReadRequest) (*rpc.ReadResponse, error) {
	// Just return back the input state.
	return &rpc.ReadResponse{
		Id:         req.Id,
		Properties: req.Properties,
	}, nil
}

func (p *randomProvider) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	// Our Random resource will never be updated - if there is a diff, it will be a replacement.
	panic("Update not implemented")
}

func (p *randomProvider) Delete(ctx context.Context, req *rpc.DeleteRequest) (*emptypb.Empty, error) {
	// Note that for our Random resource, we don't have to do anything on Delete.
	return &emptypb.Empty{}, nil
}

func (p *randomProvider) Invoke(ctx context.Context, req *rpc.InvokeRequest) (*rpc.InvokeResponse, error) {
	// The random provider doesn't support any invokes currently.
	panic("Invoke not implemented")
}

func (p *randomProvider) Call(ctx context.Context, req *rpc.CallRequest) (*rpc.CallResponse, error) {
	// The random provider doesn't support any call currently.
	panic("Call not implemented")
}

func makeRandom(length int) (string, error) {
	charset := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	result := make([]rune, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[num.Int64()]
	}
	return string(result), nil
}
