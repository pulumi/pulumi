// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

	pbempty "github.com/golang/protobuf/ptypes/empty"
)

type Bucket struct {
	pulumi.CustomResourceState
}

func NewBucket(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*Bucket, error) {
	resource := &Bucket{}
	err := ctx.RegisterResource(typeToken("Bucket"), name, nil, resource, opts...)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

type BucketComponent struct {
	pulumi.ResourceState
}

func NewBucketComponent(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketComponent, error) {
	component := &BucketComponent{}
	err := ctx.RegisterComponentResource(typeToken("BucketComponent"), name, component, opts...)
	if err != nil {
		return nil, err
	}

	_, err = NewBucket(ctx, name+"child", pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	return component, nil
}

type BucketV2 struct {
	pulumi.CustomResourceState
}

func NewBucketV2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketV2, error) {
	resource := &BucketV2{}
	aliases := pulumi.Aliases([]pulumi.Alias{
		{
			Type: pulumi.String(typeToken("Bucket")),
		},
	})
	opts = append(opts, aliases)
	err := ctx.RegisterResource(typeToken("BucketV2"), name, nil, resource, opts...)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

type BucketComponentV2 struct {
	pulumi.ResourceState
}

func NewBucketComponentV2(ctx *pulumi.Context, name string, opts ...pulumi.ResourceOption) (*BucketComponentV2, error) {
	component := &BucketComponentV2{}
	aliases := pulumi.Aliases([]pulumi.Alias{
		{
			Type: pulumi.String(typeToken("BucketComponent")),
		},
	})
	opts = append(opts, aliases)
	err := ctx.RegisterComponentResource(typeToken("BucketComponentV2"), name, component, opts...)
	if err != nil {
		return nil, err
	}

	_, err = NewBucketV2(ctx, name+"child", pulumi.Parent(component))
	if err != nil {
		return nil, err
	}

	return component, nil
}

const providerName = "wibble"
const version = "0.0.1"

func typeToken(t string) string {
	return fmt.Sprintf("%s:index:%s", providerName, t)
}

var currentID int

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
}

func makeProvider(host *provider.HostClient, name, version string) (pulumirpc.ResourceProviderServer, error) {
	return &Provider{
		host:    host,
		name:    name,
		version: version,
	}, nil
}

func (p *Provider) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	id := currentID
	currentID++

	return &pulumirpc.CreateResponse{
		Id: fmt.Sprintf("%v", id),
	}, nil
}

func (p *Provider) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {
	return pulumiprovider.Construct(ctx, req, p.host.EngineConn(), func(ctx *pulumi.Context, typ, name string,
		inputs pulumiprovider.ConstructInputs, options pulumi.ResourceOption) (*pulumiprovider.ConstructResult, error) {

		var component pulumi.ComponentResource
		var err error
		switch typ {
		case typeToken("BucketComponent"):
			component, err = NewBucketComponent(ctx, name, options)
		case typeToken("BucketComponentV2"):
			component, err = NewBucketComponentV2(ctx, name, options)
		default:
			err = fmt.Errorf("unknown resource type %s", req.GetType())
		}
		if err != nil {
			return nil, fmt.Errorf("creating component: %w", err)
		}

		return pulumiprovider.NewConstructResult(component)
	})
}

func (p *Provider) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	return &pulumirpc.CheckResponse{Inputs: req.GetNews()}, nil
}

func (p *Provider) DiffConfig(ctx context.Context,
	req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	return &pulumirpc.DiffResponse{}, nil
}

func (p *Provider) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {
	return &pulumirpc.ConfigureResponse{
		AcceptSecrets:   true,
		SupportsPreview: true,
		AcceptResources: true,
	}, nil
}

func (p *Provider) Invoke(ctx context.Context,
	req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	return nil, fmt.Errorf("Unknown Invoke token '%s'", req.GetTok())
}

func (p *Provider) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer) error {
	return fmt.Errorf("Unknown StreamInvoke token '%s'", req.GetTok())
}

func (p *Provider) Call(ctx context.Context,
	req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	return nil, fmt.Errorf("Unknown Call token '%s'", req.GetTok())
}

func (p *Provider) Check(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
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
	req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	return &pulumirpc.UpdateResponse{
		Properties: req.GetNews(),
	}, nil
}

func (p *Provider) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}

func (p *Provider) GetPluginInfo(context.Context, *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{
		Version: p.version,
	}, nil
}

func (p *Provider) Attach(ctx context.Context, req *pulumirpc.PluginAttach) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}

func (p *Provider) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {
	return &pulumirpc.GetSchemaResponse{}, nil
}

func (p *Provider) Cancel(context.Context, *pbempty.Empty) (*pbempty.Empty, error) {
	return &pbempty.Empty{}, nil
}

func (p *Provider) GetMapping(context.Context, *pulumirpc.GetMappingRequest) (*pulumirpc.GetMappingResponse, error) {
	return &pulumirpc.GetMappingResponse{}, nil
}
