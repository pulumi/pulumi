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
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

type awaiting struct {
	pulumi.CustomResourceState

	Value pulumi.StringOutput `pulumi:"value"`
}

type awaitingArgs struct {
	ReadyFile pulumi.StringInput `pulumi:"readyFile"`
}

func (awaitingArgs) ElementType() reflect.Type { return reflect.TypeFor[awaitingArgs]() }

type Component struct {
	pulumi.ResourceState

	Out pulumi.StringOutput `pulumi:"out"`
}

func main() {
	err := pulumiprovider.Main("awaitcomp", func(
		host *pulumiprovider.HostClient,
	) (pulumirpc.ResourceProviderServer, error) {
		return &provider{host: host}, nil
	})
	if err != nil {
		cmdutil.Exit(err)
	}
}

type provider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host *pulumiprovider.HostClient
}

func (p *provider) Construct(
	ctx context.Context, req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption,
	) (*pulumiprovider.ConstructResult, error) {
		if typ != "awaitcomp:index:Component" {
			return nil, fmt.Errorf("unknown resource type %s", typ)
		}
		args := &awaitingArgs{}
		if err := inputs.CopyTo(args); err != nil {
			return nil, err
		}
		comp := &Component{}
		if err := ctx.RegisterComponentResource(typ, name, comp, options); err != nil {
			return nil, err
		}
		child := &awaiting{}
		if err := ctx.RegisterResource("testprovider:index:AwaitsUntilReady", name+"-child", args, child,
			pulumi.Parent(comp)); err != nil {
			return nil, err
		}
		comp.Out = child.Value
		if err := ctx.RegisterResourceOutputs(comp, pulumi.Map{"out": child.Value}); err != nil {
			return nil, err
		}
		return pulumiprovider.NewConstructResult(comp)
	})
}

func (p *provider) CheckConfig(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *provider) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *provider) Configure(
	ctx context.Context, req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{AcceptSecrets: true, AcceptResources: true, AcceptOutputs: true}, nil
}

func (p *provider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *provider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: "0.0.1"}, nil
}

func (p *provider) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}
