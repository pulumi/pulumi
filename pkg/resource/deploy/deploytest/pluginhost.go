// Copyright 2016-2018, Pulumi Corporation.
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

package deploytest

import (
	"github.com/blang/semver"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

type LoadProviderFunc func() (plugin.Provider, error)

type ProviderLoader struct {
	pkg     tokens.Package
	version semver.Version
	load    LoadProviderFunc
}

func NewProviderLoader(pkg tokens.Package, version semver.Version, load LoadProviderFunc) *ProviderLoader {
	return &ProviderLoader{
		pkg:     pkg,
		version: version,
		load:    load,
	}
}

type pluginHost struct {
	providerLoaders []*ProviderLoader
	languageRuntime plugin.LanguageRuntime
	sink            diag.Sink
}

func NewPluginHost(sink diag.Sink, languageRuntime plugin.LanguageRuntime,
	providerLoaders ...*ProviderLoader) plugin.Host {

	return &pluginHost{
		providerLoaders: providerLoaders,
		languageRuntime: languageRuntime,
		sink:            sink,
	}
}

func (host *pluginHost) Provider(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
	var best *ProviderLoader
	for _, l := range host.providerLoaders {
		if l.pkg != pkg {
			continue
		}

		if version != nil && l.version.LT(*version) {
			continue
		}
		if best == nil || l.version.GT(best.version) {
			best = l
		}
	}
	if best == nil {
		return nil, nil
	}
	return best.load()
}

func (host *pluginHost) LanguageRuntime(runtime string) (plugin.LanguageRuntime, error) {
	return host.languageRuntime, nil
}

func (host *pluginHost) SignalCancellation() error {
	return nil
}
func (host *pluginHost) Close() error {
	return nil
}
func (host *pluginHost) ServerAddr() string {
	panic("Host RPC address not available")
}
func (host *pluginHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32,
	isStatus bool) {
	host.sink.Logf(sev, diag.StreamMessage(urn, msg, streamID), isStatus)
}
func (host *pluginHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	return nil, errors.New("unsupported")
}
func (host *pluginHost) CloseProvider(provider plugin.Provider) error {
	return nil
}
func (host *pluginHost) ListPlugins() []workspace.PluginInfo {
	return nil
}
func (host *pluginHost) EnsurePlugins(plugins []workspace.PluginInfo, kinds plugin.Flags) error {
	return nil
}
func (host *pluginHost) GetRequiredPlugins(info plugin.ProgInfo,
	kinds plugin.Flags) ([]workspace.PluginInfo, error) {
	return nil, nil
}
