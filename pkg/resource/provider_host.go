// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// A ProviderHost hosts provider plugins and makes them easily accessible by package name.
type ProviderHost interface {
	// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for it.  If an analyzer
	// could not be found, or an error occurred while creating it, a non-nil error is returned.
	Analyzer(nm tokens.QName) (Analyzer, error)

	// Provider fetches the provider for a given package, lazily allocating it if necessary.  If a provider for this
	// package could not be found, or an error occurs while creating it, a non-nil error is returned.
	Provider(pkg tokens.Package) (Provider, error)

	// Close reclaims any resources associated with the host.
	Close() error
}

// NewDefaultProviderHost implements the standard plugin logic, using the standard installation root to find them.
func NewDefaultProviderHost(ctx *Context) ProviderHost {
	return &defaultProviderHost{
		ctx:       ctx,
		analyzers: make(map[tokens.QName]Analyzer),
		providers: make(map[tokens.Package]Provider),
	}
}

type defaultProviderHost struct {
	ctx       *Context                    // the shared context for this host.
	analyzers map[tokens.QName]Analyzer   // a cache of analyzer plugins and their processes.
	providers map[tokens.Package]Provider // a cache of provider plugins and their processes.
}

func (host *defaultProviderHost) Analyzer(name tokens.QName) (Analyzer, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.analyzers[name]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewAnalyzer(host.ctx, name)
	if err == nil {
		host.analyzers[name] = plug // memoize the result.
	}
	return plug, err
}

func (host *defaultProviderHost) Provider(pkg tokens.Package) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := host.providers[pkg]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewProvider(host.ctx, pkg)
	if err == nil {
		host.providers[pkg] = plug // memoize the result.
	}
	return plug, err
}

func (host *defaultProviderHost) Close() error {
	// Close all plugins.
	for _, plugin := range host.analyzers {
		if err := plugin.Close(); err != nil {
			glog.Infof("Error closing '%v' analyzer plugin during shutdown; ignoring: %v", plugin.Name(), err)
		}
	}
	for _, plugin := range host.providers {
		if err := plugin.Close(); err != nil {
			glog.Infof("Error closing '%v' provider plugin during shutdown; ignoring: %v", plugin.Pkg(), err)
		}
	}
	// Empty out all maps.
	host.analyzers = make(map[tokens.QName]Analyzer)
	host.providers = make(map[tokens.Package]Provider)
	return nil
}
