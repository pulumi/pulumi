// Copyright 2024, Pulumi Corporation.

package main

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type Provider struct {
	plugin.UnimplementedProvider
}

func (p *Provider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *Provider) Construct(ctx context.Context, req plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	return plugin.ConstructResponse{
		Outputs: property.NewMap(map[string]property.Value{
			"ITS_ALIVE": property.New("IT'S ALIVE!"),
		}),
	}, nil
}

func main() {
	err := provider.Main(
		"provider-go", func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
			return plugin.NewProviderServer(&Provider{}), nil
		})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}
