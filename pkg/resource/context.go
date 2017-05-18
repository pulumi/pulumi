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
	"context"

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/eval/rt"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/pkg/util/contract"
)

// Context is used to group related operations together so that associated OS resources can be cached, shared, and
// reclaimed as appropriate.
type Context struct {
	Diag      diag.Sink                   // the diagnostics sink to use for messages.
	Analyzers map[tokens.QName]Analyzer   // a cache of analyzer plugins and their processes.
	Providers map[tokens.Package]Provider // a cache of provider plugins and their processes.
	ObjRes    objectResourceMap           // the resources held inside of this snapshot.
	ObjURN    objectURNMap                // a convenient lookup map for object to urn.
	URNRes    urnResourceMap              // a convenient lookup map for urn to resource.
	URNOldIDs urnIDMap                    // a convenient lookup map for urns to old IDs.
}

type objectURNMap map[*rt.Object]URN
type objectResourceMap map[*rt.Object]Resource
type urnResourceMap map[URN]Resource
type urnIDMap map[URN]ID

func NewContext(d diag.Sink) *Context {
	return &Context{
		Diag:      d,
		Analyzers: make(map[tokens.QName]Analyzer),
		Providers: make(map[tokens.Package]Provider),
		ObjRes:    make(objectResourceMap),
		ObjURN:    make(objectURNMap),
		URNRes:    make(urnResourceMap),
		URNOldIDs: make(urnIDMap),
	}
}

// Analyzer fetches the analyzer with a given name, possibly lazily allocating the plugins for it.  If an analyzer
// could not be found, or an error occurred while creating it, a non-nil error is returned.
func (ctx *Context) Analyzer(name tokens.QName) (Analyzer, error) {
	// First see if we already loaded this plugin.
	if plug, has := ctx.Analyzers[name]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewAnalyzer(ctx, name)
	if err == nil {
		ctx.Analyzers[name] = plug // memoize the result.
	}
	return plug, err
}

// Provider fetches the provider for a given resource, possibly lazily allocating the plugins for it.  If a provider
// could not be found, or an error occurred while creating it, a non-nil error is returned.
func (ctx *Context) Provider(pkg tokens.Package) (Provider, error) {
	// First see if we already loaded this plugin.
	if plug, has := ctx.Providers[pkg]; has {
		contract.Assert(plug != nil)
		return plug, nil
	}

	// If not, try to load and bind to a plugin.
	plug, err := NewProvider(ctx, pkg)
	if err == nil {
		ctx.Providers[pkg] = plug // memoize the result.
	}
	return plug, err
}

// Request allocates a request sub-context.
func (ctx *Context) Request() context.Context {
	// TODO: support cancellation.
	return context.TODO()
}

// Close reclaims all resources associated with this context.
func (ctx *Context) Close() error {
	for _, plugin := range ctx.Providers {
		if err := plugin.Close(); err != nil {
			glog.Infof("Error closing '%v' plugin during shutdown; ignoring: %v", plugin.Pkg(), err)
		}
	}
	ctx.Providers = make(map[tokens.Package]Provider) // empty out the plugin map
	return nil
}
