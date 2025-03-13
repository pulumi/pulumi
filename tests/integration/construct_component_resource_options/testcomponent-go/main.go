// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ComponentArgs struct {
	Echo pulumi.StringInput `pulumi:"echo"`
}

type Component struct {
	pulumi.ResourceState

	Echo pulumi.StringOutput `pulumi:"echo"`
	Foo  pulumi.StringOutput `pulumi:"foo"`
	Bar  pulumi.StringOutput `pulumi:"bar"`
}

func NewComponent(
	ctx *pulumi.Context, name string, args *ComponentArgs, opts ...pulumi.ResourceOption,
) (*Component, error) {
	var comp Component
	if err := ctx.RegisterComponentResource("testcomponent:index:Component", name, &comp, opts...); err != nil {
		return nil, err
	}

	_, err := NewResource(ctx, fmt.Sprintf("%s-child", name), pulumi.Parent(&comp))
	if err != nil {
		return nil, err
	}

	comp.Echo = args.Echo.ToStringOutput()
	comp.Foo = pulumi.String("foo").ToStringOutput()
	comp.Bar = pulumi.String("bar").ToStringOutput()

	return &comp, ctx.RegisterResourceOutputs(&comp, pulumi.Map{
		"echo": comp.Echo,
		"foo":  comp.Foo,
		"bar":  comp.Bar,
	})
}

type Resource struct {
	pulumi.CustomResourceState
}

func NewResource(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Resource, error) {
	var res Resource
	err := ctx.RegisterResource("testcomponent:index:Resource", name, nil, &res, opts...)
	return &res, err
}

func main() {
	err := provider.Main("testcomponent", func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return NewProvider(host, "testcomponent", "0.0.1"), nil
	})
	if err != nil {
		cmdutil.Exit(err)
	}
}

type Provider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host    *provider.HostClient
	name    string
	version string

	// The next ID to use for a resource.
	currentID atomic.Int64
}

func NewProvider(host *provider.HostClient, name, version string) pulumirpc.ResourceProviderServer {
	return &Provider{
		host:    host,
		name:    name,
		version: version,
	}
}

func (p *Provider) Create(ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.GetUrn())
	typ := urn.Type()
	if typ != "testcomponent:index:Resource" {
		return nil, fmt.Errorf("unknown resource type %q", typ)
	}

	id := p.currentID.Add(1)
	return &pulumirpc.CreateResponse{
		Id: strconv.FormatInt(id, 10),
	}, nil
}

func (p *Provider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(
		ctx *pulumi.Context,
		typ, name string,
		inputs pulumiprovider.ConstructInputs,
		options pulumi.ResourceOption,
	) (*pulumiprovider.ConstructResult, error) {
		if typ != "testcomponent:index:Component" {
			return nil, fmt.Errorf("unknown resource type %q", typ)
		}

		var args ComponentArgs
		if err := inputs.CopyTo(&args); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}

		comp, err := NewComponent(ctx, name, &args, options)
		if err != nil {
			return nil, err
		}

		return pulumiprovider.NewConstructResult(comp)
	})
}

func (p *Provider) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *Provider) DiffConfig(ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *Provider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
		AcceptOutputs:   true,
	}, nil
}

func (p *Provider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *Provider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *Provider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return &pulumirpc.ReadResponse{
		Id:         req.GetId(),
		Properties: req.GetProperties(),
	}, nil
}

func (p *Provider) Update(ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	return &pulumirpc.UpdateResponse{
		Properties: req.GetNews(),
	}, nil
}

func (p *Provider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *Provider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *Provider) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *Provider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{}, nil
}

func (p *Provider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *Provider) GetMapping(context.Context, *pulumirpc.GetMappingRequest) (*pulumirpc.GetMappingResponse, error) {
	return &pulumirpc.GetMappingResponse{}, nil
}

func (p *Provider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("unknown Invoke %q", req.GetTok())
}

func (p *Provider) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	return nil, fmt.Errorf("unknown Call %q", req.GetTok())
}
