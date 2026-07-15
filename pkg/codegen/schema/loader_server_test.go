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

package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	codegenrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/codegen"
)

// mockSchemaHost returns a plugin context whose host resolves pluginName@pluginVersion to the
// given provider.
func mockSchemaHost(
	t *testing.T, pluginName string, pluginVersion semver.Version, provider plugin.Provider,
) *plugin.Context {
	host := &plugin.MockHost{
		ProviderF: func(_ *plugin.Context, descriptor workspace.PluginDescriptor, e env.Env) (plugin.Provider, error) {
			assert.Equal(t, pluginName, descriptor.Name)
			return provider, nil
		},
		ResolvePluginF: func(_ *plugin.Context, spec workspace.PluginDescriptor) (*workspace.PluginInfo, error) {
			return &workspace.PluginInfo{
				Name:    pluginName,
				Kind:    apitype.ResourcePlugin,
				Version: &pluginVersion,
			}, nil
		},
	}
	pctx, err := plugin.NewContextWithHost(t.Context(), nil, nil, host, "", "", nil)
	require.NoError(t, err)
	return pctx
}

func schemaProvider(schemaBytes []byte) *plugin.MockProvider {
	return &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			return plugin.GetSchemaResponse{Schema: schemaBytes}, nil
		},
	}
}

// serveLoader serves loader over gRPC and returns a LoaderClient connected to it.
func serveLoader(t *testing.T, loader ReferenceLoader) *LoaderClient {
	cancel := make(chan bool)
	handle, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
		Cancel: cancel,
		Init: func(srv *grpc.Server) error {
			codegenrpc.RegisterLoaderServer(srv, NewLoaderServer(loader))
			return nil
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		close(cancel)
		require.NoError(t, <-handle.Done)
	})

	client, err := NewLoaderClient(fmt.Sprintf("127.0.0.1:%d", handle.Port))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return client
}

// Tests that when the schema declares its own version, GetSchema serves the raw provider bytes verbatim and the
// client binds them to a package equivalent to an in-process load.
func TestLoaderServerRawSchemaBytes(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	// Distinctive whitespace: re-marshaling would not preserve it, so byte equality proves the raw path was taken.
	rawSchema := []byte(`{ "name": "test",  "version": "1.2.3",` +
		` "resources": { "test:index:Resource": { "properties": { "foo": { "type": "string" } } } } }`)
	pctx := mockSchemaHost(t, "test", semver.MustParse("1.2.3"), schemaProvider(rawSchema))
	client := serveLoader(t, NewPluginLoader(pctx))

	resp, err := client.clientRaw.GetSchema(t.Context(), &codegenrpc.GetSchemaRequest{
		Package: "test",
		Version: "1.2.3",
	})
	require.NoError(t, err)
	assert.Equal(t, rawSchema, resp.Schema)

	version := semver.MustParse("1.2.3")
	descriptor := &PackageDescriptor{Name: "test", Version: &version}

	clientPkg, err := client.LoadPackageV2(t.Context(), descriptor)
	require.NoError(t, err)
	inProcessPkg, err := NewPluginLoader(pctx).LoadPackageV2(t.Context(), descriptor)
	require.NoError(t, err)

	clientSpec, err := clientPkg.MarshalSpec()
	require.NoError(t, err)
	inProcessSpec, err := inProcessPkg.MarshalSpec()
	require.NoError(t, err)
	assert.Equal(t, inProcessSpec, clientSpec)
}

// Tests that when the schema lacks a top-level version, GetSchema falls back to the bind-based path so the
// plugin version is defaulted into the schema, and the client sees the same package as an in-process load.
func TestLoaderServerRawSchemaBytesNoVersionFallback(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	rawSchema := []byte(`{ "name": "test",` +
		` "resources": { "test:index:Resource": { "properties": { "foo": { "type": "string" } } } } }`)
	pctx := mockSchemaHost(t, "test", semver.MustParse("1.2.3"), schemaProvider(rawSchema))
	client := serveLoader(t, NewPluginLoader(pctx))

	resp, err := client.clientRaw.GetSchema(t.Context(), &codegenrpc.GetSchemaRequest{
		Package: "test",
		Version: "1.2.3",
	})
	require.NoError(t, err)
	assert.NotEqual(t, rawSchema, resp.Schema)
	var spec PackageSpec
	require.NoError(t, json.Unmarshal(resp.Schema, &spec))
	assert.Equal(t, "1.2.3", spec.Version)

	version := semver.MustParse("1.2.3")
	descriptor := &PackageDescriptor{Name: "test", Version: &version}

	clientRef, err := client.LoadPackageReferenceV2(t.Context(), descriptor)
	require.NoError(t, err)
	inProcessRef, err := NewPluginLoader(pctx).LoadPackageReferenceV2(t.Context(), descriptor)
	require.NoError(t, err)
	assert.Equal(t, &version, clientRef.Version())
	assert.Equal(t, inProcessRef.Version(), clientRef.Version())
}

// Tests that parameterized packages take the raw-bytes path: the provider is parameterized exactly as before
// and the parameterized schema is served verbatim.
func TestLoaderServerRawSchemaBytesParameterized(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	rawSchema := []byte(`{ "name": "sub",  "version": "3.0.0" }`)
	provider := schemaProvider(rawSchema)
	provider.ParameterizeF = func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
		assert.Equal(t, &plugin.ParameterizeValue{
			Name:    "sub",
			Version: semver.MustParse("3.0.0"),
			Value:   []byte("testdata"),
		}, req.Parameters)
		return plugin.ParameterizeResponse{
			Name:    "sub",
			Version: semver.MustParse("3.0.0"),
		}, nil
	}
	pctx := mockSchemaHost(t, "base", semver.MustParse("1.0.0"), provider)
	client := serveLoader(t, NewPluginLoader(pctx))

	resp, err := client.clientRaw.GetSchema(t.Context(), &codegenrpc.GetSchemaRequest{
		Package: "base",
		Version: "1.0.0",
		Parameterization: &codegenrpc.Parameterization{
			Name:    "sub",
			Version: "3.0.0",
			Value:   []byte("testdata"),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, rawSchema, resp.Schema)

	version := semver.MustParse("1.0.0")
	ref, err := client.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{
		Name:    "base",
		Version: &version,
		Parameterization: &ParameterizationDescriptor{
			Name:    "sub",
			Version: semver.MustParse("3.0.0"),
			Value:   []byte("testdata"),
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "sub", ref.Name())
}

// Tests that pre-seeded entry cache entries are served via the bind-based path instead of the raw-bytes path.
// `pulumi package add` seeds the loader with file-based schemas whose plugins don't exist; the raw path must not
// bypass them.
func TestLoaderServerCachedEntryBypassesRawPath(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())
	t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "true")

	pkg, err := ImportSpec(PackageSpec{
		Name:    "mypkg",
		Version: "0.1.0",
		Resources: map[string]ResourceSpec{
			"mypkg:index:Resource": {},
		},
	}, nil, NewNullLoader(), ValidationOptions{})
	require.NoError(t, err)

	// A host whose plugin resolution always fails: the cached entry is the only way to serve the schema.
	host := &plugin.MockHost{
		ResolvePluginF: func(_ *plugin.Context, spec workspace.PluginDescriptor) (*workspace.PluginInfo, error) {
			return nil, workspace.NewMissingError(spec, false)
		},
	}
	pctx, err := plugin.NewContextWithHost(t.Context(), nil, nil, host, "", "", nil)
	require.NoError(t, err)
	loader := NewCachedLoaderWithEntries(
		NewPluginLoader(pctx),
		map[string]PackageReference{pkg.Reference().Identity(): pkg.Reference()},
	)
	client := serveLoader(t, loader)

	version := semver.MustParse("0.1.0")
	ref, err := client.LoadPackageReferenceV2(t.Context(), &PackageDescriptor{Name: "mypkg", Version: &version})
	require.NoError(t, err)
	assert.Equal(t, "mypkg", ref.Name())
	assert.Equal(t, &version, ref.Version())
}

// Tests that empty schemas still surface the getSchemaNotImplemented error instead of empty bytes.
func TestLoaderServerRawSchemaBytesEmptySchema(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	pctx := mockSchemaHost(t, "test", semver.MustParse("1.2.3"), schemaProvider([]byte(" { } ")))
	client := serveLoader(t, NewPluginLoader(pctx))

	_, err := client.clientRaw.GetSchema(t.Context(), &codegenrpc.GetSchemaRequest{
		Package: "test",
		Version: "1.2.3",
	})
	require.ErrorContains(t, err, getSchemaNotImplemented{}.Error())
}

func TestHasTopLevelVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		schema   string
		expected bool
	}{
		{`{"name":"test","version":"1.0.0"}`, true},
		{`{"version":"1.0.0"}`, true},
		{`{ "version" : "1.0.0" }`, true},
		{`{"resources":{"a":{"version":"9.9.9"}},"version":"1.0.0"}`, true},
		{`{"name":"test"}`, false},
		{`{"name":"test","version":""}`, false},
		{`{"name":"test","version":null}`, false},
		{`{"name":"test","version":{"x":1}}`, false},
		{`{"resources":{"a":{"version":"9.9.9"}}}`, false},
		{`{"meta":[{"version":"9.9.9"}],"name":"test"}`, false},
		{`[]`, false},
		{`{}`, false},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, hasTopLevelVersion([]byte(c.schema)), "schema: %s", c.schema)
	}
}
