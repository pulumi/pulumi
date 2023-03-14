// Copyright 2016-2023, Pulumi Corporation.
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

package plugin

import (
	"context"
	"io"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// A version of Provider interface that is enhanced by giving access to the request Context.
type ProviderWithContext interface {
	io.Closer

	PkgWithContext(ctx context.Context) tokens.Package

	GetSchemaWithContext(ctx context.Context, version int) ([]byte, error)

	CheckConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool) (resource.PropertyMap, []CheckFailure, error)

	DiffConfigWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, ignoreChanges []string) (DiffResult, error)

	ConfigureWithContext(ctx context.Context, inputs resource.PropertyMap) error

	CheckWithContext(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
		allowUnknowns bool, randomSeed []byte) (resource.PropertyMap, []CheckFailure, error)

	DiffWithContext(ctx context.Context, urn resource.URN, id resource.ID, olds resource.PropertyMap,
		news resource.PropertyMap, allowUnknowns bool, ignoreChanges []string) (DiffResult, error)

	CreateWithContext(ctx context.Context, urn resource.URN, news resource.PropertyMap, timeout float64,
		preview bool) (resource.ID, resource.PropertyMap, resource.Status, error)

	ReadWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap) (ReadResult, resource.Status, error)

	UpdateWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		olds resource.PropertyMap, news resource.PropertyMap, timeout float64,
		ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error)

	DeleteWithContext(ctx context.Context, urn resource.URN, id resource.ID,
		props resource.PropertyMap, timeout float64) (resource.Status, error)

	ConstructWithContext(ctx context.Context, info ConstructInfo, typ tokens.Type, name tokens.QName,
		parent resource.URN, inputs resource.PropertyMap,
		options ConstructOptions) (ConstructResult, error)

	InvokeWithContext(ctx context.Context, tok tokens.ModuleMember,
		args resource.PropertyMap) (resource.PropertyMap, []CheckFailure, error)

	StreamInvokeWithContext(
		ctx context.Context,
		tok tokens.ModuleMember,
		args resource.PropertyMap,
		onNext func(resource.PropertyMap) error) ([]CheckFailure, error)

	CallWithContext(ctx context.Context, tok tokens.ModuleMember, args resource.PropertyMap, info CallInfo,
		options CallOptions) (CallResult, error)

	GetPluginInfoWithContext(ctx context.Context) (workspace.PluginInfo, error)

	SignalCancellationWithContext(ctx context.Context) error

	GetMappingWithContext(ctx context.Context, key string) ([]byte, string, error)
}

func toProviderWithContext(prov Provider) ProviderWithContext {
	if p, ok := prov.(ProviderWithContext); ok {
		return p
	}
	return &providerWithDiscardedContext{prov}
}

type providerWithDiscardedContext struct {
	p Provider
}

func (p *providerWithDiscardedContext) Close() error {
	return p.p.Close()
}

func (p *providerWithDiscardedContext) PkgWithContext(_ context.Context) tokens.Package {
	return p.p.Pkg()
}

func (p *providerWithDiscardedContext) GetSchemaWithContext(_ context.Context, version int) ([]byte, error) {
	return p.p.GetSchema(version)
}

func (p *providerWithDiscardedContext) CheckConfigWithContext(
	_x context.Context,
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
) (resource.PropertyMap, []CheckFailure, error) {
	return p.p.CheckConfig(urn, olds, news, allowUnknowns)
}

func (p *providerWithDiscardedContext) DiffConfigWithContext(
	_ context.Context,
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (DiffResult, error) {
	return p.p.DiffConfig(urn, olds, news, allowUnknowns, ignoreChanges)
}

func (p *providerWithDiscardedContext) ConfigureWithContext(_ context.Context, inputs resource.PropertyMap) error {
	return p.p.Configure(inputs)
}

func (p *providerWithDiscardedContext) CheckWithContext(
	_ context.Context,
	urn resource.URN,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	randomSeed []byte,
) (resource.PropertyMap, []CheckFailure, error) {
	return p.p.Check(urn, olds, news, allowUnknowns, randomSeed)
}

func (p *providerWithDiscardedContext) DiffWithContext(
	_ context.Context,
	urn resource.URN,
	id resource.ID,
	olds, news resource.PropertyMap,
	allowUnknowns bool,
	ignoreChanges []string,
) (DiffResult, error) {
	return p.p.Diff(urn, id, olds, news, allowUnknowns, ignoreChanges)
}

func (p *providerWithDiscardedContext) CreateWithContext(
	_ context.Context,
	urn resource.URN,
	news resource.PropertyMap,
	timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	return p.p.Create(urn, news, timeout, preview)
}

func (p *providerWithDiscardedContext) ReadWithContext(
	_ context.Context,
	urn resource.URN,
	id resource.ID,
	inputs, state resource.PropertyMap,
) (ReadResult, resource.Status, error) {
	return p.p.Read(urn, id, inputs, state)
}

func (p *providerWithDiscardedContext) UpdateWithContext(
	_ context.Context,
	urn resource.URN,
	id resource.ID,
	olds resource.PropertyMap,
	news resource.PropertyMap,
	timeout float64,
	ignoreChanges []string,
	preview bool,
) (resource.PropertyMap, resource.Status, error) {
	return p.p.Update(urn, id, olds, news, timeout, ignoreChanges, preview)
}

func (p *providerWithDiscardedContext) DeleteWithContext(
	_ context.Context,
	urn resource.URN,
	id resource.ID,
	props resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	return p.p.Delete(urn, id, props, timeout)
}

func (p *providerWithDiscardedContext) ConstructWithContext(
	_ context.Context,
	info ConstructInfo,
	typ tokens.Type,
	name tokens.QName,
	parent resource.URN,
	inputs resource.PropertyMap,
	options ConstructOptions,
) (ConstructResult, error) {
	return p.p.Construct(info, typ, name, parent, inputs, options)
}

func (p *providerWithDiscardedContext) InvokeWithContext(
	_ context.Context,
	tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []CheckFailure, error) {
	return p.p.Invoke(tok, args)
}

func (p *providerWithDiscardedContext) StreamInvokeWithContext(
	_ context.Context,
	tok tokens.ModuleMember,
	args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]CheckFailure, error) {
	return p.p.StreamInvoke(tok, args, onNext)
}

func (p *providerWithDiscardedContext) CallWithContext(
	_ context.Context,
	tok tokens.ModuleMember,
	args resource.PropertyMap,
	info CallInfo,
	options CallOptions,
) (CallResult, error) {
	return p.p.Call(tok, args, info, options)
}

func (p *providerWithDiscardedContext) GetPluginInfoWithContext(_ context.Context) (workspace.PluginInfo, error) {
	return p.p.GetPluginInfo()
}

func (p *providerWithDiscardedContext) SignalCancellationWithContext(_ context.Context) error {
	return p.p.SignalCancellation()
}

func (p *providerWithDiscardedContext) GetMappingWithContext(_ context.Context, key string) ([]byte, string, error) {
	return p.p.GetMapping(key)
}
