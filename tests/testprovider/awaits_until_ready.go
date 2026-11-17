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
	"os"

	pschema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/emptypb"
)

func init() {
	providerSchema.Resources["testprovider:index:AwaitsUntilReady"] = pschema.ResourceSpec{
		ObjectTypeSpec: pschema.ObjectTypeSpec{
			Description: "A test resource whose Create returns an await failure until its ready file exists.",
			Properties: map[string]pschema.PropertySpec{
				"value": {TypeSpec: pschema.TypeSpec{Type: "string"}},
			},
			Type: "object",
		},
		InputProperties: map[string]pschema.PropertySpec{
			"readyFile": {TypeSpec: pschema.TypeSpec{Type: "string"}},
		},
	}
}

type awaitsUntilReadyProvider struct{ awaitsOnCreateProvider }

func (p *awaitsUntilReadyProvider) Create(
	ctx context.Context, req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	inputs, err := plugin.UnmarshalProperties(req.GetProperties(), plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(inputs["readyFile"].StringValue()); err != nil {
		detail := &pulumirpc.ErrorResourceAwaitFailed{}
		return nil, rpcerror.WithDetails(rpcerror.New(codes.Aborted, "not ready"), detail)
	}
	props, err := plugin.MarshalProperties(
		resource.PropertyMap{"value": resource.NewProperty("ready")}, plugin.MarshalOptions{})
	if err != nil {
		return nil, err
	}
	return &pulumirpc.CreateResponse{Id: "id", Properties: props}, nil
}

func (p *awaitsUntilReadyProvider) Delete(
	ctx context.Context, req *pulumirpc.DeleteRequest,
) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
