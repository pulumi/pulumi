// Copyright 2026, Pulumi Corporation.

package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

var version = semver.MustParse("0.0.1")

type Provider struct {
	plugin.UnimplementedProvider
}

func (p *Provider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{Version: &version}, nil
}

func (p *Provider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: []byte(`{
		"name": "test-provider",
		"version": "0.0.1",
		"resources": {
			"test-provider:index:Component": {
				"isComponent": true
			}
		}
	}`)}, nil
}

func (p *Provider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *Provider) Construct(ctx context.Context, req plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	sentinelDir := os.Getenv("SENTINEL_DIR")
	if sentinelDir == "" {
		sentinelDir = "."
	}

	// Write "started" sentinel to indicate Construct has been entered.
	_ = os.WriteFile(filepath.Join(sentinelDir, "started"), []byte("started"), 0o600)

	// Block forever
	select {}
	return plugin.ConstructResponse{}, ctx.Err()
}

func main() {
	// Ignore SIGINT
	signal.Ignore(os.Interrupt)

	err := provider.Main(
		"test-provider", func(host *provider.HostClient) (pulumirpc.ResourceProviderServer, error) {
			return plugin.NewProviderServer(&Provider{}), nil
		})
	if err != nil {
		cmdutil.ExitError(err.Error())
	}
}
