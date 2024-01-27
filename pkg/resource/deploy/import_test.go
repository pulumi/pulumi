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
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
)

func TestImportDeployment(t *testing.T) {
	t.Parallel()
	t.Run("NewImportDeployment", func(t *testing.T) {
		t.Parallel()
		t.Run("error in migrate providers", func(t *testing.T) {
			t.Parallel()
			var decrypterCalled bool
			_, err := NewImportDeployment(&plugin.Context{}, &Target{
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
						return "", fmt.Errorf("expected fail")
					},
				},
			}, "projectName", nil, true)
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
					goals: &goalMap{},
					ctx:   &plugin.Context{Diag: &deploytest.NoopSink{}},
					target: &Target{
						Name: tokens.MustParseStackName("stack-name"),
					},
					source: &nullSource{},
					providers: providers.NewRegistry(&mockHost{
						ProviderF: func(pkg tokens.Package, version *semver.Version) (plugin.Provider, error) {
							assert.Equal(t, tokens.Package("foo"), pkg)
							assert.Equal(t, "1.0.0", version.String())
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
						goals:  &goalMap{},
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
