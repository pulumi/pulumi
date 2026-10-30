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

package newcmd

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func resourcePackage(name, version string) workspace.PackageDescriptor {
	v := semver.MustParse(version)
	return workspace.PackageDescriptor{PluginDescriptor: workspace.PluginDescriptor{
		Kind:    apitype.ResourcePlugin,
		Name:    name,
		Version: &v,
	}}
}

func schemaProvider(schemaJSON string) *plugin.MockProvider {
	return &plugin.MockProvider{
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			return plugin.GetSchemaResponse{Schema: []byte(schemaJSON)}, nil
		},
	}
}

func TestCredentialsCheckEnabled(t *testing.T) {
	tests := []struct {
		name    string
		args    newArgs
		env     string
		enabled bool
	}{
		{name: "interactive", args: newArgs{interactive: true}, enabled: true},
		{name: "non-interactive", args: newArgs{interactive: false}},
		{name: "generate-only", args: newArgs{interactive: true, generateOnly: true}},
		{name: "offline", args: newArgs{interactive: true, offline: true}},
		{name: "kill switch", args: newArgs{interactive: true}, env: "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("PULUMI_SKIP_NEW_CREDENTIALS_CHECK", tt.env)
			}
			assert.Equal(t, tt.enabled, credentialsCheckEnabled(tt.args))
		})
	}
}

func TestPreflightCloudCredentialsPackages(t *testing.T) {
	t.Parallel()

	optedIn := func(name, displayName string) string {
		return `{"name":"` + name + `","displayName":"` + displayName + `","validateCredentialsOnNew":true}`
	}
	schemas := map[string]string{
		"aws":    optedIn("aws", "AWS"),
		"gcp":    optedIn("gcp", "Google Cloud"),
		"random": `{"name":"random"}`,
	}

	parameterized := resourcePackage("aws", "7.0.0")
	parameterized.Parameterization = &workspace.Parameterization{
		Name: "other", Version: semver.MustParse("1.0.0"), Value: []byte("parameterization"),
	}
	extension := resourcePackage("aws", "7.0.0")
	extension.ExtensionParameterization = &workspace.Parameterization{
		Name: "extension", Version: semver.MustParse("1.0.0"),
	}
	language := resourcePackage("nodejs", "3.0.0")
	language.Kind = apitype.LanguagePlugin

	tests := []struct {
		name     string
		packages []workspace.PackageDescriptor
		loaded   []string // providers expected to be launched, in order
		checked  []string // providers expected to have CheckConfig called
		warnings []string // display names expected in the output
	}{
		{name: "no packages"},
		{
			name:     "single opted-in package",
			packages: []workspace.PackageDescriptor{resourcePackage("aws", "7.0.0")},
			loaded:   []string{"aws"},
			checked:  []string{"aws"},
			warnings: []string{"AWS"},
		},
		{
			name:     "package that did not opt in is loaded but not checked",
			packages: []workspace.PackageDescriptor{resourcePackage("random", "4.0.0")},
			loaded:   []string{"random"},
		},
		{
			name: "every opted-in package is checked",
			packages: []workspace.PackageDescriptor{
				resourcePackage("random", "4.0.0"), resourcePackage("aws", "7.0.0"), resourcePackage("gcp", "8.0.0"),
			},
			loaded:   []string{"random", "aws", "gcp"},
			checked:  []string{"aws", "gcp"},
			warnings: []string{"AWS", "Google Cloud"},
		},
		{
			name:     "parameterized package uses base plugin and subpackage namespace",
			packages: []workspace.PackageDescriptor{parameterized},
			loaded:   []string{"aws"},
			checked:  []string{"other"},
			warnings: []string{"Other"},
		},
		{
			name:     "extension parameterized and language packages are skipped",
			packages: []workspace.PackageDescriptor{extension, language},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var loaded, checked []string
			host := &plugin.MockHost{
				ProviderF: func(_ *plugin.Context, desc workspace.PluginDescriptor, _ env.Env) (plugin.Provider, error) {
					loaded = append(loaded, desc.Name)
					prov := schemaProvider(schemas[desc.Name])
					prov.ParameterizeF = func(_ context.Context, req plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
						value, ok := req.Parameters.(*plugin.ParameterizeValue)
						require.True(t, ok)
						assert.Equal(t, "other", value.Name)
						assert.Equal(t, semver.MustParse("1.0.0"), value.Version)
						assert.Equal(t, []byte("parameterization"), value.Value)
						return plugin.ParameterizeResponse{Name: value.Name, Version: value.Version}, nil
					}
					prov.GetSchemaF = func(_ context.Context, req plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
						if req.SubpackageName == "" {
							return plugin.GetSchemaResponse{Schema: []byte(schemas[desc.Name])}, nil
						}
						assert.Equal(t, "other", req.SubpackageName)
						require.NotNil(t, req.SubpackageVersion)
						assert.Equal(t, semver.MustParse("1.0.0"), *req.SubpackageVersion)
						return plugin.GetSchemaResponse{Schema: []byte(optedIn("other", "Other"))}, nil
					}
					prov.CheckConfigF = func(_ context.Context, req plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
						name := strings.TrimPrefix(string(req.URN.Type()), "pulumi:providers:")
						checked = append(checked, name)
						if name == "aws" {
							// Plaintext config is passed; secrets are not decrypted.
							assert.Equal(t, 1, req.News.Len())
							assert.Equal(t, "us-east-1", req.News.Get("region").AsString())
						}
						assert.Equal(t, "pulumi:providers:"+name, string(req.URN.Type()))
						return plugin.CheckConfigResponse{}, status.Error(codes.Unknown, "no credentials")
					}
					return prov, nil
				},
			}

			sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
			pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
			require.NoError(t, err)
			defer pctx.Close()

			var buf bytes.Buffer
			pf := credentialsPreflight{
				host: host, pctx: pctx, stdout: &buf, opts: display.Options{Color: colors.Never},
				cfg: config.Map{
					config.MustMakeKey("aws", "region"): config.NewValue("us-east-1"),
					config.MustMakeKey("aws", "token"):  config.NewSecureValue("ciphertext"),
				},
			}
			for _, pkg := range tt.packages {
				if pkg.Kind != apitype.ResourcePlugin || pkg.ExtensionParameterization != nil {
					continue
				}
				pf.checkPackage(t.Context(), pkg)
			}

			assert.Equal(t, tt.loaded, loaded)
			assert.Equal(t, tt.checked, checked)
			for _, name := range tt.warnings {
				assert.Contains(t, buf.String(), "Could not validate your "+name+" credentials")
			}
			assert.Equal(t, len(tt.warnings), strings.Count(buf.String(), "warning:"))
		})
	}
}

func TestPreflightCloudCredentialsSkipsMismatchedParameterization(t *testing.T) {
	t.Parallel()

	desc := resourcePackage("aws", "7.0.0")
	desc.Parameterization = &workspace.Parameterization{
		Name: "other", Version: semver.MustParse("1.0.0"), Value: []byte("parameterization"),
	}
	prov := &plugin.MockProvider{
		ParameterizeF: func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			return plugin.ParameterizeResponse{Name: "unexpected", Version: semver.MustParse("2.0.0")}, nil
		},
		GetSchemaF: func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
			t.Fatal("GetSchema must not be called after a mismatched parameterization response")
			return plugin.GetSchemaResponse{}, nil
		},
	}
	host := &plugin.MockHost{
		ProviderF: func(*plugin.Context, workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
			return prov, nil
		},
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)
	defer pctx.Close()

	credentialsPreflight{host: host, pctx: pctx}.checkPackage(t.Context(), desc)
}

func TestPreflightCloudCredentialsWarning(t *testing.T) {
	t.Parallel()

	const message = "run `aws sso login` to refresh your session"
	tests := []struct {
		name   string
		color  colors.Colorization
		expect func(t *testing.T, out string)
	}{
		{
			name:  "commands in the provider's message and ours are highlighted",
			color: colors.Always,
			expect: func(t *testing.T, out string) {
				for _, command := range []string{"aws sso login", "pulumi up"} {
					assert.Contains(t, out, "`"+colors.Always.Colorize(colors.SpecCommand+command+colors.Reset)+"`")
				}
			},
		},
		{
			name:  "text is unchanged without color",
			color: colors.Never,
			expect: func(t *testing.T, out string) {
				assert.Contains(t, out, "warning: Could not validate your aws credentials:\n    "+message+"\n")
				assert.Contains(t, out, "`pulumi up` may fail until this is resolved.\n\n")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prov := schemaProvider(`{"name":"aws","validateCredentialsOnNew":true}`)
			prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
				return plugin.CheckConfigResponse{}, status.Error(codes.Unknown, message)
			}
			var buf bytes.Buffer
			preflightOne(t.Context(), t, prov, &buf, tt.color)
			tt.expect(t, buf.String())
		})
	}
}

// A check that could not run says nothing about the credentials, so it must not warn.
func TestPreflightCloudCredentialsStaysSilent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
		cancel bool
	}{
		{name: "provider did not opt in", schema: `{"name":"random"}`},
		{name: "invalid schema", schema: `not json`},
		{name: "cancelled", schema: `{"name":"aws","validateCredentialsOnNew":true}`, cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			prov := schemaProvider(tt.schema)
			prov.CheckConfigF = func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
				if tt.cancel {
					cancel()
				}
				return plugin.CheckConfigResponse{}, status.Error(codes.Canceled, "cancelled")
			}
			var buf bytes.Buffer
			preflightOne(ctx, t, prov, &buf, colors.Never)
			assert.Empty(t, buf.String())
		})
	}
}

// preflightOne runs the check for a single package backed by prov.
func preflightOne(
	ctx context.Context, t *testing.T, prov plugin.Provider, stdout io.Writer, color colors.Colorization,
) {
	t.Helper()
	host := &plugin.MockHost{
		ProviderF: func(*plugin.Context, workspace.PluginDescriptor, env.Env) (plugin.Provider, error) {
			return prov, nil
		},
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	pctx, err := plugin.NewContextWithHost(t.Context(), sink, sink, host, "", "", nil)
	require.NoError(t, err)
	defer pctx.Close()

	pf := credentialsPreflight{host: host, pctx: pctx, stdout: stdout, opts: display.Options{Color: color}}
	pf.checkPackage(ctx, resourcePackage("aws", "7.0.0"))
}
