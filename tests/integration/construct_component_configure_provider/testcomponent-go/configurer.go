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

package main

import (
	"fmt"

	"github.com/pulumi/pulumi-tls/sdk/v4/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Configurer struct {
	pulumi.ResourceState

	TlsProxy pulumi.StringOutput `pulumi:"tlsProxy"`
}

type ConfigurerArgs struct {
	TlsProxy pulumi.StringInput `pulumi:"tlsProxy"`
}

func NewConfigurer(
	ctx *pulumi.Context,
	name string,
	args *ConfigurerArgs,
	opts ...pulumi.ResourceOption,
) (*Configurer, error) {
	if args == nil {
		return nil, fmt.Errorf("args is required")
	}
	component := &Configurer{}
	err := ctx.RegisterComponentResource(configurerResourceToken, name, component, opts...)
	if err != nil {
		return nil, err
	}

	component.TlsProxy = args.TlsProxy.ToStringOutput()

	if err := ctx.RegisterResourceOutputs(component, pulumi.Map{
		"tlsProxy": component.TlsProxy,
	}); err != nil {
		return nil, err
	}
	return component, nil
}

type TlsProviderArgs struct{}

type TlsProviderResult struct {
	Resource tls.ProviderOutput `pulumi:"resource"`
}

func (c *Configurer) TlsProvider(ctx *pulumi.Context, args *TlsProviderArgs) (*TlsProviderResult, error) {
	prov, err := tls.NewProvider(ctx, "tls-p", &tls.ProviderArgs{
		// Due to pulumi/pulumi-tls#160 cannot yet set URL here, but can test setting FromEnv.
		Proxy: &tls.ProviderProxyArgs{
			FromEnv: c.TlsProxy.ApplyT(func(proxy string) bool {
				if proxy == "FromEnv" {
					return true
				}
				return false
			}).(pulumi.BoolOutput),
		},
	})
	if err != nil {
		return nil, err
	}

	return &TlsProviderResult{
		Resource: prov.ToProviderOutput(),
	}, nil
}
