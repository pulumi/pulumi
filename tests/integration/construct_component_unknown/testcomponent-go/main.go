// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	"google.golang.org/protobuf/types/known/emptypb"
)

type Component struct {
	pulumi.ResourceState
}

type ComponentNested struct {
	Value string `pulumi:"value"`
}

type ComponentNestedInput interface {
	pulumi.Input

	ToComponentNestedOutput() ComponentNestedOutput
	ToComponentNestedOutputWithContext(context.Context) ComponentNestedOutput
}

type ComponentNestedArgs struct {
	Value pulumi.StringInput `pulumi:"value"`
}

func (ComponentNestedArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentNested)(nil)).Elem()
}

func (i ComponentNestedArgs) ToComponentNestedOutput() ComponentNestedOutput {
	return i.ToComponentNestedOutputWithContext(context.Background())
}

func (i ComponentNestedArgs) ToComponentNestedOutputWithContext(ctx context.Context) ComponentNestedOutput {
	return pulumi.ToOutputWithContext(ctx, i).(ComponentNestedOutput)
}

type ComponentNestedOutput struct{ *pulumi.OutputState }

func (ComponentNestedOutput) ElementType() reflect.Type {
	return reflect.TypeOf((*ComponentNested)(nil)).Elem()
}

func (o ComponentNestedOutput) ToComponentNestedOutput() ComponentNestedOutput {
	return o
}

func (o ComponentNestedOutput) ToComponentNestedOutputWithContext(ctx context.Context) ComponentNestedOutput {
	return o
}

type ComponentArgs struct {
	Message pulumi.StringInput   `pulumi:"message"`
	Nested  ComponentNestedInput `pulumi:"nested"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption,
) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	args.Message.ToStringOutput().ApplyT(func(val string) string {
		panic("should not run (message)")
	})
	args.Nested.ToComponentNestedOutput().ApplyT(func(val ComponentNested) string {
		panic("should not run (nested)")
	})

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{}); err != nil {
		return nil, err
	}

	return component, nil
}

const (
	providerName = "testcomponent"
	version      = "0.0.1"
)

var currentID int

func main() {
	err := provider.Main(providerName, func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
		return makeProvider(host, providerName, version)
	})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}

type testcomponentProvider struct {
	pulumirpc.UnimplementedResourceProviderServer

	host    *provider.HostClient
	name    string
	version string
}

func makeProvider(host *provider.HostClient, name, version string) (pulumirpc.ResourceProviderServer, error) {
	return &testcomponentProvider{
		host:    host,
		name:    name,
		version: version,
	}, nil
}

func (p *testcomponentProvider) Create(ctx context.Context,
	req *pulumirpc.CreateRequest,
) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.GetUrn())
	typ := urn.Type()
	if typ != "testcomponent:index:Resource" {
		return nil, fmt.Errorf("Unknown resource type '%s'", typ)
	}

	id := currentID
	currentID++

	return &pulumirpc.CreateResponse{
		Id: fmt.Sprintf("%v", id),
	}, nil
}

func (p *testcomponentProvider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest,
) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption,
	) (*pulumiprovider.ConstructResult, error) {
		if typ != "testcomponent:index:Component" {
			return nil, fmt.Errorf("unknown resource type %s", typ)
		}

		args := &ComponentArgs{}
		if err := inputs.CopyTo(args); err != nil {
			return nil, fmt.Errorf("setting args: %w", err)
		}

		component, err := NewComponent(ctx, name, args, options)
		if err != nil {
			return nil, fmt.Errorf("creating component: %w", err)
		}

		return pulumiprovider.NewConstructResult(component)
	})
}

func (p *testcomponentProvider) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testcomponentProvider) DiffConfig(ctx context.Context,
	req *pulumirpc.DiffRequest,
) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *testcomponentProvider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest,
) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
	}, nil
}

func (p *testcomponentProvider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("Unknown Invoke token '%s'", req.GetTok())
}

func (p *testcomponentProvider) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer,
) error {
	return fmt.Errorf("Unknown StreamInvoke token '%s'", req.GetTok())
}

func (p *testcomponentProvider) Call(ctx context.Context,
	req *pulumirpc.CallRequest,
) (*pulumirpc.CallResponse, error) {
	return nil, fmt.Errorf("Unknown Call token '%s'", req.GetTok())
}

func (p *testcomponentProvider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest,
) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.News, Failures: nil}, nil
}

func (p *testcomponentProvider) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *testcomponentProvider) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	return &pulumirpc.ReadResponse{
		Id:         req.GetId(),
		Properties: req.GetProperties(),
	}, nil
}

func (p *testcomponentProvider) Update(ctx context.Context,
	req *pulumirpc.UpdateRequest,
) (*pulumirpc.UpdateResponse, error) {
	return &pulumirpc.UpdateResponse{
		Properties: req.GetNews(),
	}, nil
}

func (p *testcomponentProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testcomponentProvider) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *testcomponentProvider) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testcomponentProvider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest,
) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{}, nil
}

func (p *testcomponentProvider) Cancel(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

func (p *testcomponentProvider) GetMapping(
	context.Context, *pulumirpc.GetMappingRequest,
) (*pulumirpc.GetMappingResponse, error) {
	return &pulumirpc.GetMappingResponse{}, nil
}

func init() {
	pulumi.RegisterOutputType(ComponentNestedOutput{})
}
