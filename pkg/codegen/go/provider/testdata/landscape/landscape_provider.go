package landscape

import (
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/provider"
)

type ProviderArgs struct {
	// Name provides a friendly name for the provider.
	Name string `pulumi:"name,optional"`
}

//pulumi:provider
type Provider struct {
	name string
}

func (p *Provider) Configure(ctx *provider.Context, args *ProviderArgs, options provider.ConfigureOptions) error {
	p.name = args.Name
	return nil
}
