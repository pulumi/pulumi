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

//go:build !all
// +build !all

package main

import (
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/resource/provider"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumiprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
)

const (
	providerName             = "metaprovider"
	version                  = "0.0.1"
	mainModule               = "index"
	configurerResourceToken  = "metaprovider:index:Configurer"
	tlsProviderMethodToken   = "metaprovider:index:Configurer/tlsProvider"
	meaningOfLifeMethodToken = "metaprovider:index:Configurer/meaningOfLife"
)

type module struct {
	version semver.Version
}

func (m *module) Version() semver.Version {
	return m.version
}

func (m *module) Construct(ctx *pulumi.Context, name, typ, urn string) (r pulumi.Resource, err error) {
	switch typ {
	case configurerResourceToken:
		r = &Configurer{}
	default:
		return nil, fmt.Errorf("unknown resource type: %s", typ)
	}

	err = ctx.RegisterResource(typ, name, nil, r, pulumi.URN_(urn))
	return
}

func call(ctx *pulumi.Context, tok string, args pulumiprovider.CallArgs) (*pulumiprovider.CallResult, error) {
	switch tok {
	case tlsProviderMethodToken:
		methodArgs := &TlsProviderArgs{}
		res, err := args.CopyTo(methodArgs)
		if err != nil {
			return nil, fmt.Errorf("setting args: %w", err)
		}
		component := res.(*Configurer)
		result, err := component.TlsProvider(ctx, methodArgs)
		if err != nil {
			return nil, fmt.Errorf("calling method: %w", err)
		}
		return pulumiprovider.NewCallResult(result)
	case meaningOfLifeMethodToken:
		methodArgs := &MeaningOfLifeArgs{}
		res, err := args.CopyTo(methodArgs)
		if err != nil {
			return nil, fmt.Errorf("setting args: %w", err)
		}
		component := res.(*Configurer)
		result, err := component.MeaningOfLife(ctx, methodArgs)
		if err != nil {
			return nil, fmt.Errorf("calling method: %w", err)
		}
		return pulumiprovider.NewCallResult(result)
	default:
		return nil, fmt.Errorf("unknown method %s", tok)
	}
}

func construct(ctx *pulumi.Context, typ, name string, inputs pulumiprovider.ConstructInputs,
	options pulumi.ResourceOption,
) (*pulumiprovider.ConstructResult, error) {
	if typ != configurerResourceToken {
		return nil, fmt.Errorf("unknown resource type %s", typ)
	}

	args := &ConfigurerArgs{}
	if err := inputs.CopyTo(args); err != nil {
		return nil, fmt.Errorf("setting args: %w", err)
	}

	component, err := NewConfigurer(ctx, name, args, options)
	if err != nil {
		return nil, fmt.Errorf("creating configurer: %w", err)
	}

	return pulumiprovider.NewConstructResult(component)
}

func main() {
	// Register any resources that can come back as resource references that need to be rehydrated.
	pulumi.RegisterResourceModule(providerName, mainModule, &module{semver.MustParse(version)})

	if err := provider.MainWithOptions(provider.Options{
		Name:      providerName,
		Version:   version,
		Construct: construct,
		Call:      call,
	}); err != nil {
		cmdutil.ExitError(err.Error())
	}
}
