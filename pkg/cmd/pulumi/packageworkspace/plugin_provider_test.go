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

package packageworkspace

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type staticSchemaProvider struct {
	plugin.UnimplementedProvider

	schema []byte
}

func (p *staticSchemaProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: p.schema}, nil
}

// `pulumi package add my-provider --server https://example.com/downloads` records
// pluginDownloadURL in the Pulumi.yaml packages section, and the SDK it generates embeds
// that URL in its schema (packages.SchemaFromSchemaSource injects it). When the generated
// SDK is deleted and `pulumi install` regenerates it from the packages section, the schema
// is fetched through [pluginProvider] instead, which must inject the same URL — otherwise
// the regenerated SDK silently loses the download URL and `pulumi up` cannot resolve the
// provider plugin.
func TestGetSchemaInjectsPluginDownloadURL(t *testing.T) {
	t.Parallel()

	provider := pluginProvider{
		Provider: &staticSchemaProvider{schema: []byte(`{"name":"my-provider","version":"1.2.3"}`)},
		originalSpec: workspace.PackageSpec{
			Source:            "my-provider",
			Version:           "1.2.3",
			PluginDownloadURL: "https://example.com/downloads",
		},
	}

	resp, err := provider.GetSchema(t.Context(), plugin.GetSchemaRequest{})
	require.NoError(t, err)

	var spec schema.PackageSpec
	require.NoError(t, json.Unmarshal(resp.Schema, &spec))
	require.Equal(t, "https://example.com/downloads", spec.PluginDownloadURL)
}

type closeRecordingProvider struct {
	plugin.UnimplementedProvider

	events *[]string
}

func (p *closeRecordingProvider) Close() error {
	*p.events = append(*p.events, "provider closed")
	return nil
}

// Closing a provider from RunPackage must also release the plugin context it was started in.
func TestPluginProviderCloseReleasesContext(t *testing.T) {
	t.Parallel()

	var events []string
	var pctx *plugin.Context
	host := &plugin.MockHost{
		ReleaseContextF: func(ctx *plugin.Context) error {
			require.Same(t, pctx, ctx)
			events = append(events, "context released")
			return nil
		},
	}
	d := diagtest.LogSink(t)
	pctx, err := plugin.NewContextWithHost(t.Context(), d, d, host, "", "", nil)
	require.NoError(t, err)

	provider := pluginProvider{Provider: &closeRecordingProvider{events: &events}, pctx: pctx}
	require.NoError(t, provider.Close())
	require.Equal(t, []string{"provider closed", "context released"}, events)
}
