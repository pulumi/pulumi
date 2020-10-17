// Copyright 2016-2020, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"github.com/blang/semver"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
)

type pluginHost struct {
	plugin.Host

	languageRuntime plugin.LanguageRuntime
	providers       []workspace.PluginInfo
}

func (host *pluginHost) connectToLanguageRuntime(ctx *plugin.Context, address string) error {
	// Dial the language runtime.
	conn, err := grpc.Dial(address, grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(rpcutil.OpenTracingClientInterceptor()), rpcutil.GrpcChannelOptions())
	if err != nil {
		return errors.Wrap(err, "could not connect to language host")
	}

	client := pulumirpc.NewLanguageRuntimeClient(conn)
	host.languageRuntime = plugin.NewLanguageRuntimeClient(ctx, clientRuntimeName, client)
	return nil
}

func (host *pluginHost) loadProvider(name tokens.Package, version *semver.Version) (plugin.Provider, error) {
	if host.languageRuntime == nil {
		return nil, nil
	}

	bestInHost, err := workspace.GetPluginInfoFromList(host.providers, workspace.ResourcePlugin, string(name), version)
	if err != nil || bestInHost == nil {
		return nil, nil
	}

	workspacePlugins, err := workspace.GetPlugins()
	if err != nil {
		return nil, err
	}

	allPlugins := append(workspacePlugins, host.providers...)
	bestOverall, err := workspace.GetPluginInfoFromList(allPlugins, workspace.ResourcePlugin, string(name), version)
	if err != nil || *bestInHost != *bestOverall {
		return nil, err
	}

	var bestVersion semver.Version
	if bestOverall.Version != nil {
		bestVersion = *bestOverall.Version
	}

	return host.languageRuntime.StartProvider(bestOverall.Name, bestVersion)
}

func (host *pluginHost) Provider(name tokens.Package, version *semver.Version) (plugin.Provider, error) {
	provider, err := host.loadProvider(name, version)
	if err != nil || provider == nil {
		return host.Host.Provider(name, version)
	}
	return provider, nil
}

func (host *pluginHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	if host.languageRuntime != nil {
		return host.languageRuntime, nil
	}
	lang, err := host.Host.LanguageRuntime(runtime)
	if err != nil {
		return nil, err
	}
	host.languageRuntime = lang
	return lang, nil
}
