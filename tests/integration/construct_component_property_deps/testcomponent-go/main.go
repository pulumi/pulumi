// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type Component struct {
	pulumi.ResourceState

	PropertyDependencies pulumi.StringArrayMapOutput `pulumi:"propertyDependencies"`
}

const (
	providerName = "testcomponent"
	version      = "0.0.1"
)

func main() {
	err := provider.Main(providerName, func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return makeProvider(host, providerName, version)
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type Provider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host    *provider.HostClient
	name    string
	version string

	expectResourceArg bool
}

func makeProvider(host *provider.HostClient, name, version string) (pulumirpc.ResourceProviderServer, error) {
	return &Provider{
		host:    host,
		name:    name,
		version: version,
	}, nil
}

func (p *Provider) Create(ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.GetUrn())
	typ := urn.Type()
	if typ != providerName+":index:Custom" {
		return nil, fmt.Errorf("Unknown resource type '%s'", typ)
	}

	id := "id-" + req.GetProperties().Fields["value"].AsInterface().(string)
	if req.Preview {
		id = ""
	}

	return &pulumirpc.CreateResponse{
		Id:         id,
		Properties: req.GetProperties(),
	}, nil
}

func (p *Provider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption,
	) (*pulumiprovider.ConstructResult, error) {
		if typ != providerName+":index:Component" {
			return nil, fmt.Errorf("unknown resource type %s", typ)
		}

		component := &Component{}
		if err := ctx.RegisterComponentResource(providerName+":index:Component", name, component); err != nil {
			return nil, err
		}

		propertyDeps := pulumi.StringArrayMap{}
		for k, v := range req.GetInputDependencies() {
			deps := pulumi.StringArray{}
			for _, dep := range v.GetUrns() {
				deps = append(deps, pulumi.String(dep))
			}
			propertyDeps[k] = deps
		}

		if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
			"propertyDeps": propertyDeps,
		}); err != nil {
			return nil, err
		}

		return pulumiprovider.NewConstructResult(component)
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
	}, nil
}

func (p *Provider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("Unknown Invoke token '%s'", req.GetTok())
}

func (p *Provider) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer,
) error {
	return fmt.Errorf("Unknown StreamInvoke token '%s'", req.GetTok())
}

func (p *Provider) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	if req.Tok != providerName+":index:Component/refs" {
		return nil, fmt.Errorf("Unknown Call token '%s'", req.GetTok())
	}

	propertyDeps := map[string]any{}
	for k, v := range req.GetArgDependencies() {
		deps := []any{}
		for _, dep := range v.GetUrns() {
			deps = append(deps, dep)
		}
		propertyDeps[k] = deps
	}

	ret, err := structpb.NewStruct(map[string]any{
		"result": propertyDeps,
	})
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CallResponse{
		Return: ret,
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
