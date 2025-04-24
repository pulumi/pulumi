// Copyright 2024, Pulumi Corporation.

package main

import (
	"context"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type Provider struct {
	plugin.UnimplementedProvider
}

func (p *Provider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *Provider) Construct(ctx context.Context, req plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	propMap := resource.NewPropertyMap(nil)
	propMap["ITS_ALIVE"] = resource.NewPropertyValue("IT'S ALIVE!")
	return plugin.ConstructResponse{
		Outputs: propMap,
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
