// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

package main

import (
	"context"
	"fmt"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pbempty "github.com/golang/protobuf/ptypes/empty"
)

type Component struct {
	pulumi.ResourceState
	First  pulumi.StringOutput `pulumi:"first"`
	Second pulumi.StringOutput `pulumi:"second"`
}

type ComponentArgs struct {
	First  pulumi.StringInput `pulumi:"first"`
	Second pulumi.StringInput `pulumi:"second"`
}

func NewComponent(ctx *pulumi.Context, name string, args *ComponentArgs,
	opts ...pulumi.ResourceOption) (*Component, error) {
	if args == nil {
		return nil, errors.New("args is required")
	}

	component := &Component{}
	err := ctx.RegisterComponentResource("testcomponent:index:Component", name, component, opts...)
	if err != nil {
		return nil, err
	}

	component.First = args.First.ToStringOutput()
	component.Second = args.Second.ToStringOutput()

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"first":  args.First,
		"second": args.Second,
	}); err != nil {
		return nil, err
	}

	return component, nil
}

type ComponentGetMessageArgs struct {
	Name pulumi.StringInput `pulumi:"name"`
}

type ComponentGetMessageResult struct {
	Message pulumi.StringOutput `pulumi:"message"`
}

func (c *Component) GetMessage(args *ComponentGetMessageArgs) (*ComponentGetMessageResult, error) {
	return &ComponentGetMessageResult{
		Message: pulumi.Sprintf("%s %s, %s!", c.First, c.Second, args.Name),
	}, nil
}

const providerName = "testcomponent"
const version = "0.0.1"

type module struct {
	version semver.Version
}

func (m *module) Version() semver.Version {
	return m.version
}

func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {
	switch typ {
	case "testcomponent:index:Component":
		r = &Component{}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", typ)
	}

	err = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return
}

func main() {
	// Register any resources that can come back as resource references that need to be rehydrated.
	pulumi.RegisterResourceModule("testcomponent", "index", &module{semver.MustParse(version)})

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
	return nil, errors.Errorf("Unknown resource type '%s'", typ)
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
	return pulumiprovider.Call(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, tok string,
		args pulumiprovider.CallArgs) (*pulumiprovider.CallResult, error) {

		if tok != "testcomponent:index:Component/getMessage" {
			return nil, errors.Errorf("unknown method %s", tok)
		}

		methodArgs := &ComponentGetMessageArgs{}
		res, err := args.CopyTo(methodArgs)
		if err != nil {
			return nil, errors.Wrap(err, "setting args")
		}
		component := res.(*Component)

		result, err := component.GetMessage(methodArgs)
		if err != nil {
			return nil, errors.Wrap(err, "calling method")
		}

		return pulumiprovider.NewCallResult(result)
	})
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
