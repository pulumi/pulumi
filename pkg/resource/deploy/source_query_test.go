// Copyright 2016-2024, Pulumi Corporation.
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
	"io"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestQuerySource_Trivial_Wait(t *testing.T) {
	t.Parallel()

	// Trivial querySource returns immediately with `Wait()`, even with multiple invocations.

	// Success case.
	var called1 bool
	resmon1 := mockResmon{
		CancelF: func() error {
			called1 = true
			return nil
		},
	}
	qs1, _ := newTestQuerySource(&resmon1, func(*querySource) error {
		return nil
	})

	qs1.forkRun()

	err := qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, called1)

	// Can be called twice.
	err = qs1.Wait()
	assert.NoError(t, err)

	// Failure case.
	var called2 bool
	resmon2 := mockResmon{
		CancelF: func() error {
			called2 = true
			return nil
		},
	}
	qs2, _ := newTestQuerySource(&resmon2, func(*querySource) error {
		return errors.New("failed")
	})

	qs2.forkRun()

	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, called2)

	// Can be called twice.
	err = qs2.Wait()
	assert.False(t, result.IsBail(err))
	assert.Error(t, err)
	assert.False(t, called2)
}

func TestQuerySource_Async_Wait(t *testing.T) {
	t.Parallel()

	// `Wait()` executes asynchronously.

	// Success case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	var called1 bool
	resmon1 := mockResmon{
		CancelF: func() error {
			called1 = true
			return nil
		},
	}
	qs1Start, qs1StartAck := make(chan interface{}), make(chan interface{})
	qs1, _ := newTestQuerySource(&resmon1, func(*querySource) error {
		qs1Start <- struct{}{}
		<-qs1StartAck
		return nil
	})

	qs1.forkRun()

	// Wait until querySource starts, then acknowledge starting.
	<-qs1Start
	go func() {
		qs1StartAck <- struct{}{}
	}()

	// Wait for querySource to complete.
	err := qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, called1)

	err = qs1.Wait()
	assert.NoError(t, err)
	assert.False(t, called1)

	var called2 bool
	resmon2 := mockResmon{
		CancelF: func() error {
			called2 = true
			return nil
		},
	}
	// Cancellation case.
	//
	//    test blocks until querySource signals execution has started
	// -> querySource blocks until test acknowledges querySource's signal
	// -> test blocks on `Wait()` until querySource completes.
	qs2Start, qs2StartAck := make(chan interface{}), make(chan interface{})
	qs2, cancelQs2 := newTestQuerySource(&resmon2, func(*querySource) error {
		qs2Start <- struct{}{}
		// Block forever.
		<-qs2StartAck
		return nil
	})

	qs2.forkRun()

	// Wait until querySource starts, then cancel.
	<-qs2Start
	go func() {
		cancelQs2()
	}()

	// Wait for querySource to complete.
	err = qs2.Wait()
	assert.NoError(t, err)
	assert.True(t, called2)

	err = qs2.Wait()
	assert.NoError(t, err)
	assert.True(t, called2)
}

func TestQueryResourceMonitor_UnsupportedOperations(t *testing.T) {
	t.Parallel()

	rm := &queryResmon{}

	_, err := rm.ReadResource(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support reading resources")

	_, err = rm.RegisterResource(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support creating, updating, or deleting resources")

	_, err = rm.RegisterResourceOutputs(context.Background(), nil)
	assert.EqualError(t, err, "Query mode does not support registering resource operations")
}

func TestQueryResourceMonitor(t *testing.T) {
	t.Parallel()
	t.Run("newQueryResourceMonitor", func(t *testing.T) {
		t.Parallel()
		t.Run("bad decrypter", func(t *testing.T) {
			t.Parallel()
			providerRegErrChan := make(chan error, 1)
			expectedErr := errors.New("expected error")
			resmon, err := newQueryResourceMonitor(
				nil, nil, nil, nil, nil, providerRegErrChan, nil, &EvalRunInfo{
					Proj: &workspace.Project{
						Name: "expected-project",
					},
					Target: &Target{
						Config: config.Map{
							config.MustMakeKey("test", "secret"):  config.NewSecureValue("secret-value"),
							config.MustMakeKey("test", "regular"): config.NewValue("regular-value"),
						},
						Decrypter: &decrypterMock{
							DecryptValueF: func(
								ctx context.Context, ciphertext string,
							) (string, error) {
								return "", expectedErr
							},
						},
					},
				},
			)
			_ = resmon
			assert.ErrorIs(t, err, expectedErr)
		})
		t.Run("ok", func(t *testing.T) {
			t.Parallel()
			providerRegErrChan := make(chan error, 1)
			resmon, err := newQueryResourceMonitor(
				nil, nil, nil, nil, nil, providerRegErrChan, nil, &EvalRunInfo{
					Proj: &workspace.Project{
						Name: "expected-project",
					},
					Target: &Target{
						Name: tokens.MustParseStackName("expected-name"),
					},
				},
			)
			assert.NoError(t, err)
			assert.Equal(t, "expected-project", resmon.callInfo.Project)
			assert.Equal(t, "expected-name", resmon.callInfo.Stack)
		})
	})
	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("expected-error")
		done := make(chan error, 1)
		done <- expectedErr
		rm := &queryResmon{
			cancel: make(chan bool),
			done:   done,
		}
		assert.ErrorIs(t, rm.Cancel(), expectedErr)
	})
	t.Run("Invoke", func(t *testing.T) {
		t.Parallel()
		t.Run("bad provider request", func(t *testing.T) {
			t.Parallel()
			t.Run("bad version", func(t *testing.T) {
				t.Parallel()

				rm := &queryResmon{}
				_, err := rm.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
					Tok:     "pkgA:index:func",
					Version: "bad-version",
				})
				assert.ErrorContains(t, err, "No Major.Minor.Patch elements found")
			})
			t.Run("default provider error", func(t *testing.T) {
				t.Parallel()

				providerRegChan := make(chan *registerResourceEvent, 1)
				requests := make(chan defaultProviderRequest, 1)
				rm := &queryResmon{
					reg: &providers.Registry{},
					defaultProviders: &defaultProviders{
						requests:        requests,
						providerRegChan: providerRegChan,
					},
				}
				wg := &sync.WaitGroup{}
				wg.Add(1)
				expectedErr := errors.New("expected error")
				// Needed so defaultProviders.handleRequest() doesn't hang.
				go func() {
					req := <-rm.defaultProviders.requests
					req.response <- defaultProviderResponse{
						err: expectedErr,
					}
					wg.Done()
				}()
				_, err := rm.Invoke(context.Background(), &pulumirpc.ResourceInvokeRequest{
					Tok:     "pkgA:index:func",
					Version: "1.0.0",
				})
				wg.Wait()
				assert.ErrorIs(t, err, expectedErr)
			})
		})
	})
}

//
// Test querySource constructor.
//

func newTestQuerySource(mon SourceResourceMonitor,
	runLangPlugin func(*querySource) error,
) (*querySource, context.CancelFunc) {
	cancel, cancelFunc := context.WithCancel(context.Background())

	return &querySource{
		mon:               mon,
		runLangPlugin:     runLangPlugin,
		langPluginFinChan: make(chan error),
		cancel:            cancel,
	}, cancelFunc
}

//
// Mock resource monitor.
//

type mockResmon struct {
	AddressF func() string

	CancelF func() error

	InvokeF func(ctx context.Context,
		req *pulumirpc.ResourceInvokeRequest) (*pulumirpc.InvokeResponse, error)

	CallF func(ctx context.Context,
		req *pulumirpc.ResourceCallRequest) (*pulumirpc.CallResponse, error)

	ReadResourceF func(ctx context.Context,
		req *pulumirpc.ReadResourceRequest) (*pulumirpc.ReadResourceResponse, error)

	RegisterResourceF func(ctx context.Context,
		req *pulumirpc.RegisterResourceRequest) (*pulumirpc.RegisterResourceResponse, error)

	RegisterResourceOutputsF func(ctx context.Context,
		req *pulumirpc.RegisterResourceOutputsRequest) (*emptypb.Empty, error)

	AbortChanF func() <-chan bool
}

var _ SourceResourceMonitor = (*mockResmon)(nil)

func (rm *mockResmon) AbortChan() <-chan bool {
	if rm.AbortChanF != nil {
		return rm.AbortChanF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Address() string {
	if rm.AddressF != nil {
		return rm.AddressF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Cancel() error {
	if rm.CancelF != nil {
		return rm.CancelF()
	}
	panic("not implemented")
}

func (rm *mockResmon) Invoke(ctx context.Context,
	req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	if rm.InvokeF != nil {
		return rm.InvokeF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) Call(ctx context.Context,
	req *pulumirpc.ResourceCallRequest,
) (*pulumirpc.CallResponse, error) {
	if rm.CallF != nil {
		return rm.CallF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) ReadResource(ctx context.Context,
	req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	if rm.ReadResourceF != nil {
		return rm.ReadResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResource(ctx context.Context,
	req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if rm.RegisterResourceF != nil {
		return rm.RegisterResourceF(ctx, req)
	}
	panic("not implemented")
}

func (rm *mockResmon) RegisterResourceOutputs(ctx context.Context,
	req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	if rm.RegisterResourceOutputsF != nil {
		return rm.RegisterResourceOutputsF(ctx, req)
	}
	panic("not implemented")
}

func TestQuerySource(t *testing.T) {
	t.Parallel()
	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		var called bool
		providerRegErrChan := make(chan error, 1)
		expectedErr := errors.New("expected error")
		providerRegErrChan <- expectedErr
		src := &querySource{
			providerRegErrChan: providerRegErrChan,
			// Required to not nil ptr dereference.
			cancel: context.Background(),
			mon: &mockResmon{
				CancelF: func() error {
					called = true
					return nil
				},
			},
		}
		err := src.Wait()
		assert.ErrorIs(t, err, expectedErr)
		assert.True(t, called)
	})
}

func TestRunLangPlugin(t *testing.T) {
	t.Parallel()
	t.Run("failed to launch language host", func(t *testing.T) {
		t.Parallel()

		assert.ErrorContains(t, runLangPlugin(&querySource{
			plugctx: &plugin.Context{
				Host: &mockHost{
					LanguageRuntimeF: func(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
						return nil, errors.New("expected error")
					},
				},
			},
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj: &workspace.Project{
					Runtime: workspace.NewProjectRuntimeInfo("stuff", map[string]interface{}{}),
				},
			},
		}), "failed to launch language host")
	})
	t.Run("bad decrypter", func(t *testing.T) {
		t.Parallel()
		expectedErr := errors.New("expected error")
		err := runLangPlugin(&querySource{
			plugctx: &plugin.Context{
				Host: &mockHost{
					LanguageRuntimeF: func(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
						return &mockLanguageRuntime{}, nil
					},
				},
			},
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj: &workspace.Project{
					Runtime: workspace.NewProjectRuntimeInfo("stuff", map[string]interface{}{}),
				},
				Target: &Target{
					Config: config.Map{
						config.MustMakeKey("test", "secret"):  config.NewSecureValue("secret-value"),
						config.MustMakeKey("test", "regular"): config.NewValue("regular-value"),
					},
					Decrypter: &decrypterMock{
						DecryptValueF: func(
							ctx context.Context, ciphertext string,
						) (string, error) {
							return "", expectedErr
						},
					},
				},
			},
		})
		assert.ErrorIs(t, err, expectedErr)
	})
	t.Run("bail successfully", func(t *testing.T) {
		t.Parallel()
		err := runLangPlugin(&querySource{
			plugctx: &plugin.Context{
				Host: &mockHost{
					LanguageRuntimeF: func(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
						return &mockLanguageRuntime{
							RunF: func(info plugin.RunInfo) (string, bool, error) {
								return "bail should override progerr", true /* bail */, nil
							},
						}, nil
					},
				},
			},
			// Prevent nilptr dereference.
			mon: &mockResmon{
				AddressF: func() string { return "" },
			},
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj: &workspace.Project{
					Runtime: workspace.NewProjectRuntimeInfo("stuff", map[string]interface{}{}),
				},
			},
		})
		assert.ErrorContains(t, err, "run bailed")
	})
	t.Run("progerr", func(t *testing.T) {
		t.Parallel()
		err := runLangPlugin(&querySource{
			plugctx: &plugin.Context{
				Host: &mockHost{
					LanguageRuntimeF: func(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
						return &mockLanguageRuntime{
							RunF: func(info plugin.RunInfo) (string, bool, error) {
								return "expected progerr", false /* bail */, nil
							},
						}, nil
					},
				},
			},
			// Prevent nilptr dereference.
			mon: &mockResmon{
				AddressF: func() string { return "" },
			},
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Pwd:         "/",
				Program:     ".",
				Proj: &workspace.Project{
					Runtime: workspace.NewProjectRuntimeInfo("stuff", map[string]interface{}{}),
				},
			},
		})
		assert.ErrorContains(t, err, "expected progerr")
	})
	t.Run("langhost is run correctly", func(t *testing.T) {
		t.Parallel()
		var runCalled bool
		err := runLangPlugin(&querySource{
			plugctx: &plugin.Context{
				Host: &mockHost{
					LanguageRuntimeF: func(runtime string, p plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
						return &mockLanguageRuntime{
							RunF: func(info plugin.RunInfo) (string, bool, error) {
								runCalled = true
								assert.Equal(t, "expected-address", info.MonitorAddress)
								assert.Equal(t, "expected-stack", info.Stack)
								assert.Equal(t, "expected-project", info.Project)
								assert.Equal(t, "/expected-pwd", info.Pwd)
								assert.Equal(t, "/expected-pwd", info.Info.ProgramDirectory())
								assert.Equal(t, "expected-program", info.Info.EntryPoint())
								assert.Equal(t, []string{"expected", "args"}, info.Args)
								assert.Equal(t, "secret-value", info.Config[config.MustMakeKey("test", "secret")])
								assert.Equal(t, "regular-value", info.Config[config.MustMakeKey("test", "regular")])
								assert.True(t, info.QueryMode)
								assert.True(t, info.DryRun)
								// Disregard Parallel argument.
								assert.Equal(t, "expected-organization", info.Organization)
								return "", false, nil
							},
						}, nil
					},
				},
			},
			// Prevent nilptr dereference.
			mon: &mockResmon{
				AddressF: func() string { return "expected-address" },
			},
			runinfo: &EvalRunInfo{
				ProjectRoot: "/",
				Proj: &workspace.Project{
					Name:    "expected-project",
					Runtime: workspace.NewProjectRuntimeInfo("stuff", map[string]interface{}{}),
				},
				Pwd:     "/expected-pwd",
				Program: "expected-program",
				Args:    []string{"expected", "args"},
				Target: &Target{
					Config: config.Map{
						config.MustMakeKey("test", "secret"):  config.NewSecureValue("secret-value"),
						config.MustMakeKey("test", "regular"): config.NewValue("regular-value"),
					},
					Name:         tokens.MustParseStackName("expected-stack"),
					Organization: "expected-organization",
					Decrypter: &decrypterMock{
						DecryptValueF: func(
							ctx context.Context, ciphertext string,
						) (string, error) {
							return ciphertext, nil
						},
					},
				},
			},
		})
		assert.NoError(t, err)
		assert.True(t, runCalled)
	})
}

type mockHost struct {
	ServerAddrF func() string

	LogF func(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	LogStatusF func(sev diag.Severity, urn resource.URN, msg string, streamID int32)

	AnalyzerF func(nm tokens.QName) (plugin.Analyzer, error)

	PolicyAnalyzerF func(name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error)

	ListAnalyzersF func() []plugin.Analyzer

	ProviderF func(descriptor workspace.PackageDescriptor) (plugin.Provider, error)

	CloseProviderF func(provider plugin.Provider) error

	LanguageRuntimeF func(language string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error)

	EnsurePluginsF func(plugins []workspace.PluginSpec, kinds plugin.Flags) error

	ResolvePluginF func(kind apitype.PluginKind, name string, version *semver.Version) (*workspace.PluginInfo, error)

	GetProjectPluginsF func() []workspace.ProjectPlugin

	SignalCancellationF func() error

	CloseF func() error
}

var _ plugin.Host = (*mockHost)(nil)

func (h *mockHost) ServerAddr() string {
	if h.ServerAddrF != nil {
		return h.ServerAddrF()
	}
	panic("unimplemented")
}

func (h *mockHost) Log(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if h.LogF != nil {
		h.LogF(sev, urn, msg, streamID)
		return
	}
	panic("unimplemented")
}

func (h *mockHost) LogStatus(sev diag.Severity, urn resource.URN, msg string, streamID int32) {
	if h.LogStatusF != nil {
		h.LogStatusF(sev, urn, msg, streamID)
		return
	}
	panic("unimplemented")
}

func (h *mockHost) Analyzer(nm tokens.QName) (plugin.Analyzer, error) {
	if h.AnalyzerF != nil {
		return h.Analyzer(nm)
	}
	panic("unimplemented")
}

func (h *mockHost) PolicyAnalyzer(
	name tokens.QName, path string, opts *plugin.PolicyAnalyzerOptions,
) (plugin.Analyzer, error) {
	if h.PolicyAnalyzerF != nil {
		return h.PolicyAnalyzerF(name, path, opts)
	}
	panic("unimplemented")
}

func (h *mockHost) ListAnalyzers() []plugin.Analyzer {
	if h.ListAnalyzersF != nil {
		return h.ListAnalyzersF()
	}
	panic("unimplemented")
}

func (h *mockHost) Provider(descriptor workspace.PackageDescriptor) (plugin.Provider, error) {
	if h.ProviderF != nil {
		return h.ProviderF(descriptor)
	}
	panic("unimplemented")
}

func (h *mockHost) CloseProvider(provider plugin.Provider) error {
	if h.CloseProviderF != nil {
		return h.CloseProviderF(provider)
	}
	panic("unimplemented")
}

func (h *mockHost) LanguageRuntime(runtime string, info plugin.ProgramInfo) (plugin.LanguageRuntime, error) {
	if h.LanguageRuntimeF != nil {
		return h.LanguageRuntimeF(runtime, info)
	}
	panic("unimplemented")
}

func (h *mockHost) EnsurePlugins(plugins []workspace.PluginSpec, kinds plugin.Flags) error {
	if h.EnsurePluginsF != nil {
		return h.EnsurePluginsF(plugins, kinds)
	}
	panic("unimplemented")
}

func (h *mockHost) ResolvePlugin(
	kind apitype.PluginKind, name string, version *semver.Version,
) (*workspace.PluginInfo, error) {
	if h.ResolvePluginF != nil {
		return h.ResolvePluginF(kind, name, version)
	}
	panic("unimplemented")
}

func (h *mockHost) GetProjectPlugins() []workspace.ProjectPlugin {
	if h.GetProjectPluginsF != nil {
		return h.GetProjectPluginsF()
	}
	panic("unimplemented")
}

func (h *mockHost) SignalCancellation() error {
	if h.SignalCancellationF != nil {
		return h.SignalCancellationF()
	}
	panic("unimplemented")
}

// Close reclaims any resources associated with the host.
func (h *mockHost) Close() error {
	if h.CloseF != nil {
		return h.CloseF()
	}
	panic("unimplemented")
}

func (h *mockHost) StartDebugging(plugin.DebuggingInfo) error {
	panic("unimplemented")
}

type mockLanguageRuntime struct {
	CloseF func() error

	GetRequiredPackagesF func(info plugin.ProgramInfo) ([]workspace.PackageDescriptor, error)

	RunF func(info plugin.RunInfo) (string, bool, error)

	GetPluginInfoF func() (workspace.PluginInfo, error)

	InstallDependenciesF func(options plugin.InstallDependenciesRequest) (io.Reader, io.Reader, <-chan error, error)

	RuntimeOptionsPromptsF func(info plugin.ProgramInfo) ([]plugin.RuntimeOptionPrompt, error)

	AboutF func(info plugin.ProgramInfo) (plugin.AboutInfo, error)

	GetProgramDependenciesF func(
		info plugin.ProgramInfo, transitiveDependencies bool,
	) ([]plugin.DependencyInfo, error)

	RunPluginF func(
		info plugin.RunPluginInfo,
	) (io.Reader, io.Reader, context.CancelFunc, error)

	GenerateProjectF func(
		sourceDirectory, targetDirectory, project string,
		strict bool, loaderTarget string, localDependencies map[string]string,
	) (hcl.Diagnostics, error)

	GeneratePackageF func(
		directory string, schema string,
		extraFiles map[string][]byte, loaderTarget string, localDependencies map[string]string,
		local bool,
	) (hcl.Diagnostics, error)

	GenerateProgramF func(
		program map[string]string,
		loaderTarget string,
	) (map[string][]byte, hcl.Diagnostics, error)

	PackF func(
		packageDirectory string,
		destinationDirectory string,
	) (string, error)
}

var _ plugin.LanguageRuntime = (*mockLanguageRuntime)(nil)

func (rt *mockLanguageRuntime) Close() error {
	if rt.CloseF != nil {
		return rt.CloseF()
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GetRequiredPackages(info plugin.ProgramInfo) ([]workspace.PackageDescriptor, error) {
	if rt.GetRequiredPackagesF != nil {
		return rt.GetRequiredPackagesF(info)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) Run(info plugin.RunInfo) (string, bool, error) {
	if rt.RunF != nil {
		return rt.RunF(info)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GetPluginInfo() (workspace.PluginInfo, error) {
	if rt.GetPluginInfoF != nil {
		return rt.GetPluginInfoF()
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) InstallDependencies(
	request plugin.InstallDependenciesRequest,
) (io.Reader, io.Reader, <-chan error, error) {
	if rt.InstallDependenciesF != nil {
		return rt.InstallDependenciesF(request)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) RuntimeOptionsPrompts(info plugin.ProgramInfo) ([]plugin.RuntimeOptionPrompt, error) {
	if rt.RuntimeOptionsPromptsF != nil {
		return rt.RuntimeOptionsPromptsF(info)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) About(info plugin.ProgramInfo) (plugin.AboutInfo, error) {
	if rt.AboutF != nil {
		return rt.AboutF(info)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GetProgramDependencies(
	info plugin.ProgramInfo, transitiveDependencies bool,
) ([]plugin.DependencyInfo, error) {
	if rt.GetProgramDependenciesF != nil {
		return rt.GetProgramDependenciesF(info, transitiveDependencies)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) RunPlugin(
	info plugin.RunPluginInfo,
) (io.Reader, io.Reader, context.CancelFunc, error) {
	if rt.RunPluginF != nil {
		return rt.RunPluginF(info)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GenerateProject(
	sourceDirectory, targetDirectory, project string,
	strict bool, loaderTarget string, localDependencies map[string]string,
) (hcl.Diagnostics, error) {
	if rt.GenerateProjectF != nil {
		return rt.GenerateProjectF(
			sourceDirectory, targetDirectory, project, strict, loaderTarget, localDependencies)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GeneratePackage(
	directory string, schema string, extraFiles map[string][]byte,
	loaderTarget string, localDependencies map[string]string,
	local bool,
) (hcl.Diagnostics, error) {
	if rt.GeneratePackageF != nil {
		return rt.GeneratePackageF(directory, schema, extraFiles, loaderTarget, localDependencies, local)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) GenerateProgram(
	program map[string]string, loaderTarget string, strict bool,
) (map[string][]byte, hcl.Diagnostics, error) {
	if rt.GenerateProgramF != nil {
		return rt.GenerateProgramF(program, loaderTarget)
	}
	panic("unimplemented")
}

func (rt *mockLanguageRuntime) Pack(
	packageDirectory string, destinationDirectory string,
) (string, error) {
	if rt.PackF != nil {
		return rt.PackF(packageDirectory, destinationDirectory)
	}
	panic("unimplemented")
}
