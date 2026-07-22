// Copyright 2026, Pulumi Corporation.
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

//nolint:govet // This file is deprecated and will be removed in a future release.
package provider

import (
	"context"

	sdkprovider "github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.HostClient].
//
//go:fix inline
type HostClient = sdkprovider.HostClient

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.Options].
//
//go:fix inline
type Options = sdkprovider.Options

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.NewHostClient].
//
//go:fix inline
func NewHostClient(addr string) (*sdkprovider.HostClient, error) {
	return sdkprovider.NewHostClient(addr)
}

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.Main].
//
//go:fix inline
func Main(name string, provMaker func(*sdkprovider.HostClient) (pulumirpc.ResourceProviderServer, error)) error {
	return sdkprovider.Main(name, provMaker)
}

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.MainContext].
//
//go:fix inline
func MainContext(
	ctx context.Context,
	name string,
	provMaker func(*sdkprovider.HostClient) (pulumirpc.ResourceProviderServer, error),
) error {
	return sdkprovider.MainContext(ctx, name, provMaker)
}

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.MainWithOptions].
//
//go:fix inline
func MainWithOptions(opts sdkprovider.Options) error {
	return sdkprovider.MainWithOptions(opts)
}

// Deprecated: use [github.com/pulumi/pulumi/sdk/v3/go/pulumi/provider.ComponentMain].
//
//go:fix inline
func ComponentMain(
	name string,
	version string,
	schema []byte,
	construct sdkprovider.ConstructFunc,
) error {
	return sdkprovider.ComponentMain(name, version, schema, construct)
}
