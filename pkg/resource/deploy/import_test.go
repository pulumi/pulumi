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

package deploy

import (
	"context"
	"errors"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/urn"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/stretchr/testify/assert"
)

func TestImportDeployment(t *testing.T) {
	t.Parallel()
	t.Run("NewImportDeployment", func(t *testing.T) {
		t.Parallel()
		t.Run("error in migrate providers", func(t *testing.T) {
			t.Parallel()
			var decrypterCalled bool
			_, err := NewImportDeployment(&plugin.Context{}, &Options{}, nil, &Target{
				Snapshot: &Snapshot{
					Resources: []*resource.State{
						{
							URN:    "urn:pulumi:stack::project::type::oldName",
							Custom: true,
						},
					},
				},
				Name: tokens.MustParseStackName("target-name"),
				Config: config.Map{
					config.MustMakeKey("", "secret"): config.NewSecureValue("secret"),
				},
				Decrypter: &decrypterMock{
					DecryptValueF: func(ctx context.Context, ciphertext string) (string, error) {
						decrypterCalled = true
						return "", errors.New("expected fail")
					},
				},
			}, "projectName", nil)
			assert.ErrorContains(t, err, "could not fetch configuration for default provider")
			assert.True(t, decrypterCalled)
		})
	})
}

func TestImporter(t *testing.T) {
	t.Parallel()
	t.Run("registerProviders", func(t *testing.T) {
		t.Parallel()
		t.Run("incorrect package type specified", func(t *testing.T) {
			t.Parallel()
			i := &importer{
				deployment: &Deployment{
					imports: []Import{
						{
							Type: "::",
						},
					},
					target: &Target{
						Name: tokens.MustParseStackName("stack-name"),
					},
					source: &nullSource{
						project: "project-name",
					},
				},
			}
			_, _, err := i.registerProviders(context.Background())
			assert.ErrorContains(t, err, "incorrect package type specified")
		})
		t.Run("ensure provider is called correctly", func(t *testing.T) {
			t.Parallel()
			version := semver.MustParse("1.0.0")
			expectedErr := errors.New("expected error")
			i := &importer{
				deployment: &Deployment{
					goals: &gsync.Map[urn.URN, *resource.Goal]{},
					ctx:   &plugin.Context{Diag: &deploytest.NoopSink{}},
					target: &Target{
						Name: tokens.MustParseStackName("stack-name"),
					},
					source: &nullSource{},
					providers: providers.NewRegistry(&mockHost{
						ProviderF: func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
							assert.Equal(t, "foo", descriptor.Name)
							assert.Equal(t, "1.0.0", descriptor.Version.String())
							return nil, expectedErr
						},
					}, true, nil),
					imports: []Import{
						{
							Version:           &version,
							PluginDownloadURL: "download-url",
							PluginChecksums: map[string][]byte{
								"a": {},
								"b": {},
								"c": {},
							},
							Type: "foo:bar:Bar",
						},
					},
				},
			}
			_, _, err := i.registerProviders(context.Background())
			assert.ErrorIs(t, err, expectedErr)
		})
	})
	t.Run("importResources", func(t *testing.T) {
		t.Parallel()
		t.Run("registerExistingResources", func(t *testing.T) {
			t.Parallel()
			t.Run("ok", func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				i := &importer{
					executor: &stepExecutor{
						ctx: ctx,
					},
					deployment: &Deployment{
						prev: &Snapshot{
							Resources: []*resource.State{
								{
									URN: "some-urn",
								},
							},
						},
						goals:  &gsync.Map[urn.URN, *resource.Goal]{},
						source: &nullSource{},
						target: &Target{},
						imports: []Import{
							{},
						},
					},
				}
				assert.NoError(t, i.importResources(ctx))
			})
		})
		t.Run("getOrCreateStackResource", func(t *testing.T) {
			t.Parallel()
			t.Run("ok", func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				i := &importer{
					executor: &stepExecutor{
						ctx: ctx,
					},
					deployment: &Deployment{
						source: &nullSource{},
						target: &Target{
							Name: tokens.MustParseStackName("stack-name"),
						},
						imports: []Import{
							{},
						},
					},
				}
				assert.NoError(t, i.importResources(ctx))
			})
			t.Run("ignore existing delete resources", func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				i := &importer{
					executor: &stepExecutor{
						ctx: ctx,
					},
					deployment: &Deployment{
						prev: &Snapshot{
							Resources: []*resource.State{
								{
									Delete: true,
								},
							},
						},
						// goals is left nil as nothing should be added to it.
						goals:  nil,
						source: &nullSource{},
						target: &Target{
							Name: tokens.MustParseStackName("stack-name"),
						},
						imports: []Import{
							{},
						},
					},
				}
				assert.NoError(t, i.importResources(ctx))
			})
		})
	})
}

func TestImporterParameterizedProvider(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	version := semver.MustParse("1.2.3")
	mockProvider := plugin.MockProvider{
		ParameterizeF: func(ctx context.Context, paramReq plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error) {
			pValue, ok := paramReq.Parameters.(*plugin.ParameterizeValue)
			assert.True(t, ok)
			assert.Equal(t, pValue, &plugin.ParameterizeValue{
				Name:    "ParameterizationName",
				Version: semver.MustParse("1.2.3"),
				Value:   []byte("parameterization-value"),
			})
			return plugin.ParameterizeResponse{
				Name:    "ParameterizationName",
				Version: semver.MustParse("1.2.3"),
			}, nil
		},
		CloseF: func() error {
			return nil
		},
		CheckConfigF: func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
			return plugin.CheckConfigResponse{}, nil
		},
	}
	i := &importer{
		executor: &stepExecutor{
			ctx: ctx,
		},
		deployment: &Deployment{
			goals: &gsync.Map[urn.URN, *resource.Goal]{},
			ctx:   &plugin.Context{Diag: &deploytest.NoopSink{}},
			target: &Target{
				Name: tokens.MustParseStackName("stack-name"),
			},
			source: &nullSource{},
			providers: providers.NewRegistry(&mockHost{
				ProviderF: func(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
					assert.Equal(t, "foo", descriptor.Name)
					assert.Equal(t, "1.0.0", descriptor.Version.String())
					return &mockProvider, nil
				},
				CloseProviderF: func(provider plugin.Provider) error {
					return nil
				},
			}, true, nil),
			imports: []Import{
				{
					Version:           &version,
					PluginDownloadURL: "download-url",
					PluginChecksums: map[string][]byte{
						"a": {},
						"b": {},
						"c": {},
					},
					Type: "ParameterizationName:bar:Bar",
					Parameterization: &Parameterization{
						PluginName:    "foo",
						PluginVersion: semver.MustParse("1.0.0"),
						Value:         []byte("parameterization-value"),
					},
				},
			},
		},
	}
	_, _, err := i.registerProviders(context.Background())
	assert.NoError(t, err)
}
