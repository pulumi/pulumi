// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pbempty "github.com/golang/protobuf/ptypes/empty"
)

type Resource struct {
	pulumi.CustomResourceState
}

type resourceArgs struct {
	Echo interface{} `pulumi:"echo"`
}

type ResourceArgs struct {
	Echo pulumi.Input
}

func (ResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*resourceArgs)(nil)).Elem()
}

func NewResource(ctx *pulumi.Context, name string, echo pulumi.Input,
	opts ...pulumi.ResourceOption) (*Resource, error) {
	args := &ResourceArgs{Echo: echo}
	var resource Resource
	err := ctx.RegisterResource("testcomponent:index:Resource", name, args, &resource, opts...)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

type Component struct {
	pulumi.ResourceState

	Echo    pulumi.Input        `pulumi:"echo"`
	ChildID pulumi.IDOutput     `pulumi:"childId"`
	Secret  pulumi.StringOutput `pulumi:"secret"`
}

type ComponentArgs struct {
	Echo pulumi.Input `pulumi:"echo"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	secretKey := "secret"
	fullSecretKey := fmt.Sprintf("%s:%s", ctx.Project(), secretKey)
	if !ctx.IsConfigSecret(fullSecretKey) {
		return nil, errors.Errorf("expected configuration key to be secret: %s", fullSecretKey)
	}

	conf := config.New(ctx, "")
	secret := conf.RequireSecret(secretKey)

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	res, err := NewResource(ctx, fmt.Sprintf("child-%s", name), args.Echo, pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	component.Echo = args.Echo
	component.ChildID = res.ID()
	component.Secret = secret

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"secret":  component.Secret,
		"echo":    component.Echo,
		"childId": component.ChildID,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

const providerName = "testcomponent"
const version = "0.0.1"

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
	req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.GetUrn())
	typ := urn.Type()
	if typ != "testcomponent:index:Resource" {
		return nil, errors.Errorf("Unknown resource type '%s'", typ)
	}

	id := currentID
	currentID++

	return &pulumirpc.CreateResponse{
		Id: fmt.Sprintf("%v", id),
	}, nil
}

func (p *testcomponentProvider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption) (*pulumiprovider.ConstructResult, error) {

		if typ != "testcomponent:index:Component" {
			return nil, errors.Errorf("unknown resource type %s", typ)
		}

		args := &ComponentArgs{}
		if err := inputs.CopyTo(args); err != nil {
			return nil, errors.Wrap(err, "setting args")
		}

		component, err := NewComponent(ctx, name, args, options)
		if err != nil {
			return nil, errors.Wrap(err, "creating component")
		}

		return pulumiprovider.NewConstructResult(component)
	})
}

func (p *testcomponentProvider) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *testcomponentProvider) DiffConfig(ctx context.Context,
	req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *testcomponentProvider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
	}, nil
}

func (p *testcomponentProvider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return nil, errors.Errorf("Unknown Invoke token '%s'", req.GetTok())
}

func (p *testcomponentProvider) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer) error {
	return errors.Errorf("Unknown StreamInvoke token '%s'", req.GetTok())
}

func (p *testcomponentProvider) Call(ctx context.Context,
	req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return nil, errors.Errorf("Unknown Call token '%s'", req.GetTok())
}

func (p *testcomponentProvider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
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
	req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return &pulumirpc.UpdateResponse{
		Properties: req.GetNews(),
	}, nil
}

func (p *testcomponentProvider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}

func (p *testcomponentProvider) GetPluginInfo(context.Context, *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *testcomponentProvider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{}, nil
}

func (p *testcomponentProvider) Cancel(context.Context, *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}
