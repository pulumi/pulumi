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

package deploytest

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNewAnalyzerLoaderWithHost(t *testing.T) {
	t.Parallel()
	a := NewAnalyzerLoaderWithHost("pkgA", nil)
	assert.Equal(t, apitype.PluginKind("analyzer"), a.kind)
	assert.Equal(t, "pkgA", a.name)
	assert.Equal(t, semver.Version{}, a.version)
	assert.Equal(t, false, a.useGRPC)
}

func TestHostEngine(t *testing.T) {
	t.Parallel()
	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()
		t.Run("GetRootResource", func(t *testing.T) {
			t.Parallel()
			engine := &hostEngine{}
			req := &pulumirpc.GetRootResourceRequest{}

			_, err := engine.GetRootResource(context.Background(), req)
			assert.ErrorContains(t, err, "unsupported")
		})

		t.Run("SetRootResource", func(t *testing.T) {
			t.Parallel()
			engine := &hostEngine{}
			req := &pulumirpc.SetRootResourceRequest{}

			_, err := engine.SetRootResource(context.Background(), req)
			assert.ErrorContains(t, err, "unsupported")
		})
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		t.Run("Log", func(t *testing.T) {
			t.Parallel()
			tests := []struct {
				name           string
				req            *pulumirpc.LogRequest
				expectedError  error
				expectedOutput *emptypb.Empty
			}{
				{
					name:           "DebugSeverity",
					req:            &pulumirpc.LogRequest{Severity: pulumirpc.LogSeverity_DEBUG},
					expectedOutput: &emptypb.Empty{},
				},
				{
					name:           "InfoSeverity",
					req:            &pulumirpc.LogRequest{Severity: pulumirpc.LogSeverity_INFO},
					expectedOutput: &emptypb.Empty{},
				},
				{
					name:           "WarningSeverity",
					req:            &pulumirpc.LogRequest{Severity: pulumirpc.LogSeverity_INFO},
					expectedOutput: &emptypb.Empty{},
				},
				{
					name:           "ErrorSeverity",
					req:            &pulumirpc.LogRequest{Severity: pulumirpc.LogSeverity_INFO},
					expectedOutput: &emptypb.Empty{},
				},
				{
					name:          "InvalidSeverity",
					req:           &pulumirpc.LogRequest{Severity: 99999},
					expectedError: fmt.Errorf("Unrecognized logging severity: %v", 99999),
				},
			}

			hostEngine := &hostEngine{
				sink:       &NoopSink{},
				statusSink: &NoopSink{},
			}

			for _, ephemeral := range []bool{true, false} {
				for _, tt := range tests {
					tt := tt
					tt.req.Ephemeral = ephemeral
					t.Run(tt.name, func(t *testing.T) { //nolint:paralleltest // golangci-lint v2 upgrade
						output, err := hostEngine.Log(context.Background(), tt.req)
						assert.Equal(t, tt.expectedError, err)
						assert.Equal(t, tt.expectedOutput, output)
					})
				}
			}
		})
	})
}

func TestPluginHostProvider(t *testing.T) {
	t.Parallel()
	t.Run("Could not find plugin", func(t *testing.T) {
		t.Parallel()
		expectedVersion := semver.MustParse("1.0.0")
		host := &pluginHost{}
		_, err := host.Provider(workspace.PackageDescriptor{
			PluginSpec: workspace.PluginSpec{
				Name:    "pkgA",
				Version: &expectedVersion,
			},
		})
		assert.ErrorContains(t, err, "Could not find plugin for (pkgA, 1.0.0)")
	})
	t.Run("error: plugin host is shutting down", func(t *testing.T) {
		t.Parallel()
		t.Run("Provider", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			_, err := host.Provider(workspace.PackageDescriptor{
				PluginSpec: workspace.PluginSpec{
					Name:    "pkgA",
					Version: &semver.Version{},
				},
			})
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
		t.Run("LanguageRuntime", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			programInfo := plugin.NewProgramInfo("/", "/", ".", nil)
			_, err := host.LanguageRuntime("", programInfo)
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
		t.Run("SignalCancellation", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			err := host.SignalCancellation()
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
		t.Run("Analyzer", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			_, err := host.Analyzer("")
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
		t.Run("CloseProvider", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			err := host.CloseProvider(nil)
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
		t.Run("EnsurePlugins", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			assert.ErrorIs(t, host.EnsurePlugins(nil, 0), ErrHostIsClosed)
		})
		t.Run("PolicyAnalyzer", func(t *testing.T) {
			t.Parallel()
			host := &pluginHost{closed: true}
			_, err := host.PolicyAnalyzer("", "", nil)
			assert.ErrorIs(t, err, ErrHostIsClosed)
		})
	})
	t.Run("GetRequiredPackages (language runtime is shutting down)", func(t *testing.T) {
		t.Parallel()
		host := &pluginHost{
			closed: true,
			languageRuntime: &languageRuntime{
				closed: true,
			},
		}

		_, err := host.GetRequiredPackages(plugin.ProgramInfo{}, 0)
		assert.ErrorIs(t, err, ErrLanguageRuntimeIsClosed)
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		host := &pluginHost{closed: true}
		assert.NoError(t, host.Close())
		// Is idempotent.
		assert.NoError(t, host.Close())
	})
	t.Run("Log", func(t *testing.T) {
		t.Parallel()
		t.Run("closed", func(t *testing.T) {
			t.Parallel()
			t.Run("Log", func(t *testing.T) {
				t.Parallel()
				var called bool
				host := &pluginHost{
					closed: true,
					sink: &NoopSink{
						LogfF: func(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
							called = true
						},
					},
				}
				host.Log(diag.Debug, "", "", 0)
				assert.False(t, called)
			})
			t.Run("LogStatus", func(t *testing.T) {
				t.Parallel()
				var called bool
				host := &pluginHost{
					closed: true,
					statusSink: &NoopSink{
						LogfF: func(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
							called = true
						},
					},
				}
				host.LogStatus(diag.Debug, "", "", 0)
				assert.False(t, called)
			})
		})
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			t.Run("Log", func(t *testing.T) {
				t.Parallel()
				var called bool
				host := &pluginHost{
					sink: &NoopSink{
						LogfF: func(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
							called = true
						},
					},
				}
				host.Log(diag.Debug, "", "", 0)
				assert.True(t, called)
			})
			t.Run("LogStatus", func(t *testing.T) {
				t.Parallel()
				var called bool
				host := &pluginHost{
					statusSink: &NoopSink{
						LogfF: func(sev diag.Severity, diag *diag.Diag, args ...interface{}) {
							called = true
						},
					},
				}
				host.LogStatus(diag.Debug, "", "", 0)
				assert.True(t, called)
			})
		})
	})
}
