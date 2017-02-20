// Copyright 2016 Marapongo, Inc. All rights reserved.

package main

import (
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/sdk/go/pkg/murpc"
	"golang.org/x/net/context"

	"github.com/marapongo/mu/lib/aws/provider/awsctx"
	"github.com/marapongo/mu/lib/aws/provider/ec2"
)

// provider implements the AWS resource provider's operations for all known AWS types.
type Provider struct {
	impls map[tokens.Type]murpc.ResourceProviderServer
}

// NewProvider creates a new provider instance with server objects registered for every resource type.
func NewProvider() (*Provider, error) {
	ctx, err := awsctx.New()
	if err != nil {
		return nil, err
	}
	return &Provider{
		impls: map[tokens.Type]murpc.ResourceProviderServer{
			ec2.Instance:      ec2.NewInstanceProvider(ctx),
			ec2.SecurityGroup: ec2.NewSecurityGroupProvider(ctx),
		},
	}, nil
}

var _ murpc.ResourceProviderServer = (*Provider)(nil)

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *Provider) Create(ctx context.Context, req *murpc.CreateRequest) (*murpc.CreateResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Create(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Create): %v", t)
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *Provider) Read(ctx context.Context, req *murpc.ReadRequest) (*murpc.ReadResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Read(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Read): %v", t)
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Provider) Update(ctx context.Context, req *murpc.UpdateRequest) (*murpc.UpdateResponse, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Update(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Update): %v", t)
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *Provider) Delete(ctx context.Context, req *murpc.DeleteRequest) (*pbempty.Empty, error) {
	t := tokens.Type(req.GetType())
	if prov, has := p.impls[t]; has {
		return prov.Delete(ctx, req)
	}
	return nil, fmt.Errorf("Unrecognized resource type (Delete): %v", t)
}
