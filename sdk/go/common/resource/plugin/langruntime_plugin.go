// Copyright 2016, Pulumi Corporation.
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

package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/hcl/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// langhost reflects a language host plugin, loaded dynamically for a single language/runtime pair.
type langhost struct {
	ctx     *Context
	runtime string
	plug    *Plugin
	client  pulumirpc.LanguageRuntimeClient
}

// NewLanguageRuntime binds to a language's runtime plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewLanguageRuntime(host Host, ctx *Context, runtime, workingDirectory string,
) (LanguageRuntime, error) {
	attachPort, err := GetLanguageAttachPort(runtime)
	if err != nil {
		return nil, err
	}

	var plug *Plugin
	var client pulumirpc.LanguageRuntimeClient
	if attachPort != nil {
		port := *attachPort

		handshake := func(
			ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
		) (*pulumirpc.LanguageHandshakeResponse, error) {
			req := &pulumirpc.LanguageHandshakeRequest{
				EngineAddress: host.ServerAddr(),
				// If we're attaching then we don't know the root or program directory.
				RootDirectory:    nil,
				ProgramDirectory: nil,
			}
			return languageHandshake(ctx, bin, prefix, conn, req)
		}

		conn, handshakeResponse, err := dialPlugin(
			ctx.Base(),
			port,
			"pulumi-language-"+runtime,
			runtime+" (Language Plugin)",
			handshake,
			langRuntimePluginDialOptions(ctx, runtime),
		)
		if err != nil {
			return nil, err
		}
		if handshakeResponse == nil {
			return nil, errors.New("language did not return handshake response, attaching via " +
				"an attach port is not yet supported for this language",
			)
		}

		plug = &Plugin{
			Conn: conn,
			// Nothing to kill.
			Kill: func() error {
				return nil
			},
		}

		client = pulumirpc.NewLanguageRuntimeClient(plug.Conn)
	} else {
		path, err := workspace.GetPluginPath(
			ctx.baseContext, ctx.Diag,
			workspace.PluginDescriptor{
				Name: strings.ReplaceAll(runtime, tokens.QNameDelimiter, "_"),
				Kind: apitype.LanguagePlugin,
			},
			host.GetProjectPlugins(),
		)
		if err != nil {
			return nil, err
		}

		contract.Assertf(path != "", "unexpected empty path for language plugin %s", runtime)

		args, err := buildArgsForNewPlugin(host)
		if err != nil {
			return nil, err
		}

		plug, _, err = newPlugin(
			ctx,
			workingDirectory,
			path,
			runtime,
			apitype.LanguagePlugin,
			args,
			nil, /*env*/
			testConnection,
			langRuntimePluginDialOptions(ctx, runtime),
			host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: runtime}),
		)
		if err != nil {
			return nil, err
		}

		client = pulumirpc.NewLanguageRuntimeClient(plug.Conn)
	}

	contract.Assertf(plug != nil, "unexpected nil language plugin for %s", runtime)

	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		plug:    plug,
		client:  client,
	}, nil
}

// Checks PULUMI_DEBUG_LANGUAGES environment variable for any overrides for the
// language identified by name. If the user has requested to attach to a live
// language plugin, returns the port number from the env var.
//
// For example, `PULUMI_DEBUG_LANGUAGES=go:12345,dotnet:678` will result in 12345 for go and 678 for dotnet.
func GetLanguageAttachPort(runtime string) (*int, error) {
	var optAttach string

	if languagesEnvVar, has := os.LookupEnv("PULUMI_DEBUG_LANGUAGES"); has {
		for _, provider := range strings.Split(languagesEnvVar, ",") {
			parts := strings.SplitN(provider, ":", 2)

			if parts[0] == runtime {
				optAttach = parts[1]
				break
			}
		}
	}

	if optAttach == "" {
		return nil, nil
	}

	port, err := strconv.Atoi(optAttach)
	if err != nil {
		return nil, fmt.Errorf("Expected a numeric port, got %s in PULUMI_DEBUG_LANGUAGES: %w",
			optAttach, err)
	}
	return &port, nil
}

func langRuntimePluginDialOptions(ctx *Context, runtime string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.TracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]any{
			"mode": "client",
			"kind": "language",
		}
		if runtime != "" {
			metadata["runtime"] = runtime
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

func buildArgsForNewPlugin(host Host) ([]string, error) {
	// NOTE: positional argument for the server address must come last
	args := []string{host.ServerAddr()}
	return args, nil
}

func NewLanguageRuntimeClient(ctx *Context, runtime string, client pulumirpc.LanguageRuntimeClient) LanguageRuntime {
	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		client:  client,
	}
}

// GetRequiredPackages computes the complete set of anticipated plugins required by a program.
func (h *langhost) GetRequiredPackages(info ProgramInfo) ([]workspace.PackageDescriptor, error) {
	slog.Info("langhost.GetRequiredPackages executing", "runtime", h.runtime, "info", info)

	minfo, err := info.Marshal()
	if err != nil {
		return nil, err
	}

	resp, err := h.client.GetRequiredPackages(h.ctx.Request(), &pulumirpc.GetRequiredPackagesRequest{
		Info: minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GetRequiredPackages failed", "runtime", h.runtime, "info", info, "err", rpcError)

		// It's possible this is just an older language host, prior to the emergence of the GetRequiredPackages
		// method.  In such cases, fallback to using GetRequiredPlugins and don't report any parameterized packages.
		if rpcError.Code() == codes.Unimplemented {
			plugins, err := h.getRequiredPlugins(info)
			if err != nil {
				return nil, err
			}
			packages := make([]workspace.PackageDescriptor, len(plugins))
			for i, plugin := range plugins {
				packages[i] = workspace.PackageDescriptor{
					PluginDescriptor: plugin,
				}
			}
			return packages, nil
		}

		return nil, rpcError
	}

	results := slice.Prealloc[workspace.PackageDescriptor](len(resp.Packages))
	for _, info := range resp.Packages {
		var version *semver.Version
		if v := info.GetVersion(); v != "" {
			sv, err := semver.ParseTolerant(v)
			if err != nil {
				return nil, fmt.Errorf("illegal semver returned by language host: %s@%s: %w", info.GetName(), v, err)
			}
			version = &sv
		}
		if !apitype.IsPluginKind(info.Kind) {
			return nil, fmt.Errorf("unrecognized plugin kind: %s", info.Kind)
		}
		var parameterization *workspace.Parameterization
		if info.Parameterization != nil {
			sv, err := semver.ParseTolerant(info.Parameterization.Version)
			if err != nil {
				return nil, fmt.Errorf(
					"illegal semver returned by language host: %s@%s: %w",
					info.GetName(), info.Parameterization.Version, err)
			}

			parameterization = &workspace.Parameterization{
				Name:    info.Parameterization.Name,
				Version: sv,
				Value:   info.Parameterization.Value,
			}
		}

		results = append(results, workspace.PackageDescriptor{
			PluginDescriptor: workspace.PluginDescriptor{
				Name:              info.Name,
				Kind:              apitype.PluginKind(info.Kind),
				Version:           version,
				PluginDownloadURL: info.Server,
				Checksums:         info.Checksums,
			},
			Parameterization: parameterization,
		})
	}

	slog.Info("langhost.GetRequiredPackages success", "runtime", h.runtime, "info", info, "versions", len(results))
	return results, nil
}

// getRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (h *langhost) getRequiredPlugins(info ProgramInfo) ([]workspace.PluginDescriptor, error) {
	slog.Info("langhost.GetRequiredPlugins executing", "runtime", h.runtime, "info", info)

	minfo, err := info.Marshal()
	if err != nil {
		return nil, err
	}

	// this is deprecated and will be removed in a future release, but we use it for backcompat for now until
	// all language hosts are update to GetRequiredPackages.
	resp, err := h.client.GetRequiredPlugins(h.ctx.Request(), &pulumirpc.GetRequiredPluginsRequest{ //nolint:staticcheck
		Project: "deprecated",
		Pwd:     info.ProgramDirectory(),
		Program: info.EntryPoint(),
		Info:    minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GetRequiredPlugins failed", "runtime", h.runtime, "info", info, "err", rpcError)

		// It's possible this is just an older language host, prior to the emergence of the GetRequiredPlugins
		// method.  In such cases, we will silently error (with the above log left behind).
		if rpcError.Code() == codes.Unimplemented {
			return nil, nil
		}

		return nil, rpcError
	}

	results := slice.Prealloc[workspace.PluginDescriptor](len(resp.GetPlugins()))
	for _, info := range resp.GetPlugins() {
		var version *semver.Version
		if v := info.GetVersion(); v != "" {
			sv, err := semver.ParseTolerant(v)
			if err != nil {
				return nil, fmt.Errorf("illegal semver returned by language host: %s@%s: %w", info.GetName(), v, err)
			}
			version = &sv
		}
		if !apitype.IsPluginKind(info.Kind) {
			return nil, fmt.Errorf("unrecognized plugin kind: %s", info.Kind)
		}
		results = append(results, workspace.PluginDescriptor{
			Name:              info.Name,
			Kind:              apitype.PluginKind(info.Kind),
			Version:           version,
			PluginDownloadURL: info.Server,
			Checksums:         info.Checksums,
		})
	}

	slog.Info("langhost.GetRequiredPlugins success", "runtime", h.runtime, "info", info, "versions", len(results))
	return results, nil
}

// Run executes a program in the language runtime for planning or deployment purposes.  If
// info.DryRun is true, the code must not assume that side-effects or final values resulting from
// resource deployments are actually available.  If it is false, on the other hand, a real
// deployment is occurring and it may safely depend on these.
func (h *langhost) Run(info RunInfo) (string, bool, error) {
	slog.Info("langhost.Run executing",
		"runtime", h.runtime,
		"pwd", info.Pwd,
		"info", info.Info,
		"args", len(info.Args),
		"project", info.Project,
		"stack", info.Stack,
		"config", len(info.Config),
		"dryrun", info.DryRun)
	config := make(map[string]string, len(info.Config))
	for k, v := range info.Config {
		config[k.String()] = v
	}
	configSecretKeys := make([]string, len(info.ConfigSecretKeys))
	for i, k := range info.ConfigSecretKeys {
		configSecretKeys[i] = k.String()
	}

	minfo, err := info.Info.Marshal()
	if err != nil {
		return "", false, err
	}

	resp, err := h.client.Run(h.ctx.Request(), &pulumirpc.RunRequest{
		MonitorAddress:   info.MonitorAddress,
		Pwd:              info.Pwd,
		Program:          info.Info.EntryPoint(),
		Args:             info.Args,
		Project:          info.Project,
		Stack:            info.Stack,
		Config:           config,
		ConfigSecretKeys: configSecretKeys,
		DryRun:           info.DryRun,
		QueryMode:        info.QueryMode,
		Parallel:         info.Parallel,
		Organization:     info.Organization,
		Info:             minfo,
		LoaderTarget:     info.LoaderAddress,
		AttachDebugger:   info.AttachDebugger,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.Run failed",
			"runtime", h.runtime,
			"pwd", info.Pwd,
			"info", info.Info,
			"dryrun", info.DryRun,
			"err", rpcError)
		return "", false, rpcError
	}

	progerr := resp.GetError()
	bail := resp.GetBail()
	slog.Info("langhost.Run success",
		"runtime", h.runtime,
		"pwd", info.Pwd,
		"info", info.Info,
		"dryrun", info.DryRun,
		"progerr", progerr,
		"bail", bail)
	return progerr, bail, nil
}

// GetPluginInfo returns this plugin's information.
func (h *langhost) GetPluginInfo() (PluginInfo, error) {
	slog.Info("langhost.GetPluginInfo executing", "runtime", h.runtime)

	resp, err := h.client.GetPluginInfo(h.ctx.Request(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GetPluginInfo failed", "runtime", h.runtime, "err", rpcError)
		return PluginInfo{}, rpcError
	}
	vers := resp.Version

	plugInfo := PluginInfo{}

	if vers != "" {
		sv, err := semver.ParseTolerant(vers)
		if err != nil {
			return PluginInfo{}, err
		}
		plugInfo.Version = &sv
	}

	return plugInfo, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (h *langhost) Close() error {
	if h.plug != nil {
		return h.plug.Close()
	}
	return nil
}

func (h *langhost) InstallDependencies(request InstallDependenciesRequest) (
	io.Reader,
	io.Reader,
	<-chan error,
	error,
) {
	slog.Info("langhost.InstallDependencies executing", "runtime", h.runtime, "request", request)

	minfo, err := request.Info.Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	resp, err := h.client.InstallDependencies(h.ctx.Request(), &pulumirpc.InstallDependenciesRequest{
		Directory:               request.Info.ProgramDirectory(),
		IsTerminal:              cmdutil.GetGlobalColorization() != colors.Never,
		Info:                    minfo,
		UseLanguageVersionTools: request.UseLanguageVersionTools,
		IsPlugin:                request.IsPlugin,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.InstallDependencies failed", "runtime", h.runtime, "request", request, "err", rpcError)

		// It's possible this is just an older language host, prior to the emergence of the InstallDependencies
		// method.  In such cases, we will silently error (with the above log left behind).
		if rpcError.Code() == codes.Unimplemented {
			return nil, nil, nil, nil
		}

		return nil, nil, nil, rpcError
	}

	outr, outw := io.Pipe()
	errr, errw := io.Pipe()
	done := make(chan error, 1)

	go func() {
		defer close(done)

		for {
			slog.Debug("langhost.InstallDependencies waiting for dependency installation messages",
				"runtime", h.runtime, "request", request)

			msg, err := resp.Recv()
			if err != nil {
				if err == io.EOF {
					contract.IgnoreClose(outw)
					contract.IgnoreClose(errw)

					done <- nil
					break
				}

				rpcError := rpcerror.Convert(err)
				slog.Info("langhost.InstallDependencies failed",
					"runtime", h.runtime, "request", request, "err", rpcError)

				contract.IgnoreError(outw.CloseWithError(rpcError))
				contract.IgnoreError(errw.CloseWithError(rpcError))

				done <- rpcError
				break
			}

			slog.Debug("langhost.InstallDependencies got dependency installation response",
				"runtime", h.runtime, "request", request, "msg", msg)

			stdoutLen := len(msg.Stdout)
			if stdoutLen > 0 {
				n, err := outw.Write(msg.Stdout)
				contract.AssertNoErrorf(err, "failed to write to stdout pipe: %v", err)
				contract.Assertf(n == stdoutLen, "wrote fewer bytes (%d) than expected (%d)", n, stdoutLen)
			}

			stderrLen := len(msg.Stderr)
			if stderrLen > 0 {
				n, err := errw.Write(msg.Stderr)
				contract.AssertNoErrorf(err, "failed to write to stderr pipe: %v", err)
				contract.Assertf(n == stderrLen, "wrote fewer bytes (%d) than expected (%d)", n, stderrLen)
			}
		}
	}()

	slog.Info("langhost.InstallDependencies success", "runtime", h.runtime, "request", request)
	return outr, errr, done, nil
}

func (h *langhost) RuntimeOptionsPrompts(info ProgramInfo) ([]RuntimeOptionPrompt, error) {
	slog.Info("langhost.RuntimeOptionsPrompts executing", "runtime", h.runtime)

	minfo, err := info.Marshal()
	if err != nil {
		return []RuntimeOptionPrompt{}, err
	}

	resp, err := h.client.RuntimeOptionsPrompts(h.ctx.Request(), &pulumirpc.RuntimeOptionsRequest{
		Info: minfo,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			slog.Info("langhost.RuntimeOptionsPrompts not implemented, returning no prompts", "runtime", h.runtime)
			return []RuntimeOptionPrompt{}, nil
		}
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.RuntimeOptionsPrompts failed", "runtime", h.runtime, "err", rpcError)
		return []RuntimeOptionPrompt{}, rpcError
	}

	prompts := []RuntimeOptionPrompt{}
	for _, prompt := range resp.Prompts {
		newPrompt, err := UnmarshallRuntimeOptionPrompt(prompt)
		if err != nil {
			return []RuntimeOptionPrompt{}, err
		}
		prompts = append(prompts, newPrompt)
	}

	slog.Info("langhost.RuntimeOptionsPrompts success", "runtime", h.runtime)
	return prompts, nil
}

func (h *langhost) Template(info ProgramInfo, projectName tokens.PackageName) error {
	slog.Info("langhost.Template executing", "runtime", h.runtime)

	minfo, err := info.Marshal()
	if err != nil {
		return err
	}

	_, err = h.client.Template(h.ctx.Request(), &pulumirpc.TemplateRequest{
		Info:        minfo,
		ProjectName: string(projectName),
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			slog.Info("langhost.Template not implemented", "runtime", h.runtime)
			return nil
		}
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.Template failed", "runtime", h.runtime, "err", rpcError)
		return rpcError
	}

	slog.Info("langhost.Template success", "runtime", h.runtime)
	return nil
}

func (h *langhost) About(info ProgramInfo) (AboutInfo, error) {
	slog.Info("langhost.About executing", "runtime", h.runtime)
	minfo, err := info.Marshal()
	if err != nil {
		return AboutInfo{}, err
	}
	resp, err := h.client.About(h.ctx.Request(), &pulumirpc.AboutRequest{
		Info: minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.About failed", "runtime", h.runtime, "err", rpcError)

		return AboutInfo{}, rpcError
	}

	result := AboutInfo{
		Executable: resp.Executable,
		Version:    resp.Version,
		Metadata:   resp.Metadata,
	}

	slog.Info("langhost.About success", "runtime", h.runtime)
	return result, nil
}

func (h *langhost) GetProgramDependencies(info ProgramInfo, transitiveDependencies bool) ([]DependencyInfo, error) {
	minfo, err := info.Marshal()
	if err != nil {
		return nil, err
	}

	slog.Info("langhost.GetProgramDependencies executing", "runtime", h.runtime, "info", info.String())
	resp, err := h.client.GetProgramDependencies(h.ctx.Request(), &pulumirpc.GetProgramDependenciesRequest{
		TransitiveDependencies: transitiveDependencies,
		Info:                   minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GetProgramDependencies failed",
			"runtime", h.runtime, "info", info.String(), "err", rpcError)

		return nil, rpcError
	}

	results := slice.Prealloc[DependencyInfo](len(resp.GetDependencies()))
	for _, dep := range resp.GetDependencies() {
		results = append(results, DependencyInfo{
			Name:    dep.Name,
			Version: dep.Version,
		})
	}

	slog.Info("langhost.GetProgramDependencies success",
		"runtime", h.runtime, "info", info.String(), "versions", len(results))
	return results, nil
}

func (h *langhost) RunPlugin(ctx context.Context, info RunPluginInfo) (
	io.Reader, io.Reader, *promise.Promise[int32], error,
) {
	slog.Info("langhost.RunPlugin executing", "runtime", h.runtime, "info", info.Info.String())

	minfo, err := info.Info.Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	rctx, kill := context.WithCancel(ctx)

	resp, err := h.client.RunPlugin(rctx, &pulumirpc.RunPluginRequest{
		Pwd:            info.WorkingDirectory,
		Args:           info.Args,
		Env:            info.Env,
		Info:           minfo,
		Kind:           info.Kind,
		AttachDebugger: info.AttachDebugger,
		LoaderTarget:   info.LoaderAddress,
	})
	if err != nil {
		// If there was an error starting the plugin kill the context for this request to ensure any lingering
		// connection terminates.
		kill()
		return nil, nil, nil, err
	}

	outr, outw := io.Pipe()
	errr, errw := io.Pipe()

	cts := &promise.CompletionSource[int32]{}

	go func() {
		for {
			slog.Debug("waiting for plugin message")
			msg, err := resp.Recv()
			if err != nil {
				// If there was an error receiving then signal that the plugin has exited.
				// If err is just EOF then the plugin has exited normally, and we can exitcode 0
				err1 := outw.Close()
				err2 := errw.Close()
				if errors.Is(err, io.EOF) {
					cts.Fulfill(0)
				} else {
					// We need this condition because although `Join` will ignore nil errors it won't return the
					// original error if it's the only one. That is `Join(err, nil, nil) != err`. Because of that our
					// later "is this a grpc error" check doesn't work because it sees a `joinError` instead of a
					// `grpcError`.
					if err1 != nil || err2 != nil {
						err = errors.Join(err, err1, err2)
					}
					cts.Reject(err)
				}
				kill()
				break
			}

			slog.Debug("got plugin response", "msg", msg)

			if value, ok := msg.Output.(*pulumirpc.RunPluginResponse_Stdout); ok {
				n, err := outw.Write(value.Stdout)
				contract.AssertNoErrorf(err, "failed to write to stdout pipe: %v", err)
				contract.Assertf(n == len(value.Stdout), "wrote fewer bytes (%d) than expected (%d)", n, len(value.Stdout))
			} else if value, ok := msg.Output.(*pulumirpc.RunPluginResponse_Stderr); ok {
				n, err := errw.Write(value.Stderr)
				contract.AssertNoErrorf(err, "failed to write to stderr pipe: %v", err)
				contract.Assertf(n == len(value.Stderr), "wrote fewer bytes (%d) than expected (%d)", n, len(value.Stderr))
			} else if code, ok := msg.Output.(*pulumirpc.RunPluginResponse_Exitcode); ok {
				// If stdout and stderr are empty we've flushed and are returning the exit code
				err1 := outw.Close()
				err2 := errw.Close()
				err = errors.Join(err1, err2)
				if err != nil {
					cts.Reject(err)
				} else {
					cts.Fulfill(code.Exitcode)
				}
				kill()
				break
			}
		}
	}()

	return outr, errr, cts.Promise(), nil
}

func (h *langhost) GenerateProject(
	sourceDirectory, targetDirectory, project string, strict bool,
	loaderTarget string, localDependencies map[string]string,
) (hcl.Diagnostics, error) {
	slog.Info("langhost.GenerateProject executing", "runtime", h.runtime)
	resp, err := h.client.GenerateProject(h.ctx.Request(), &pulumirpc.GenerateProjectRequest{
		SourceDirectory:   sourceDirectory,
		TargetDirectory:   targetDirectory,
		Project:           project,
		Strict:            strict,
		LoaderTarget:      loaderTarget,
		LocalDependencies: localDependencies,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GenerateProject failed", "runtime", h.runtime, "err", rpcError)
		return nil, rpcError
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	slog.Info("langhost.GenerateProject success", "runtime", h.runtime)
	return diags, nil
}

func (h *langhost) GeneratePackage(
	directory string, schema string, extraFiles map[string][]byte,
	loaderTarget string, localDependencies map[string]string,
	local bool,
) (hcl.Diagnostics, error) {
	slog.Info("langhost.GeneratePackage executing", "runtime", h.runtime)
	resp, err := h.client.GeneratePackage(h.ctx.Request(), &pulumirpc.GeneratePackageRequest{
		Directory:         directory,
		Schema:            schema,
		ExtraFiles:        extraFiles,
		LoaderTarget:      loaderTarget,
		LocalDependencies: localDependencies,
		Local:             local,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GeneratePackage failed", "runtime", h.runtime, "err", rpcError)
		return nil, rpcError
	}

	slog.Info("langhost.GeneratePackage success", "runtime", h.runtime)

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	return diags, nil
}

func (h *langhost) GenerateProgram(program map[string]string, loaderTarget string, strict bool,
) (map[string][]byte, hcl.Diagnostics, error) {
	slog.Info("langhost.GenerateProgram executing", "runtime", h.runtime)
	resp, err := h.client.GenerateProgram(h.ctx.Request(), &pulumirpc.GenerateProgramRequest{
		Source:       program,
		LoaderTarget: loaderTarget,
		Strict:       strict,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.GenerateProgram failed", "runtime", h.runtime, "err", rpcError)
		return nil, nil, rpcError
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	slog.Info("langhost.GenerateProgram success", "runtime", h.runtime)
	return resp.Source, diags, nil
}

func (h *langhost) Pack(
	packageDirectory string, destinationDirectory string,
) (string, error) {
	slog.Info("langhost.Pack executing",
		"runtime", h.runtime,
		"packageDirectory", packageDirectory,
		"destinationDirectory", destinationDirectory)

	// Always send absolute paths to the plugin, as it may be running in a different working directory.
	packageDirectory, err := filepath.Abs(packageDirectory)
	if err != nil {
		return "", err
	}
	destinationDirectory, err = filepath.Abs(destinationDirectory)
	if err != nil {
		return "", err
	}

	req, err := h.client.Pack(h.ctx.Request(), &pulumirpc.PackRequest{
		PackageDirectory:     packageDirectory,
		DestinationDirectory: destinationDirectory,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.Pack failed",
			"runtime", h.runtime,
			"packageDirectory", packageDirectory,
			"destinationDirectory", destinationDirectory,
			"err", rpcError)
		return "", rpcError
	}

	slog.Info("langhost.Pack success",
		"runtime", h.runtime,
		"packageDirectory", packageDirectory,
		"destinationDirectory", destinationDirectory,
		"artifactPath", req.ArtifactPath)
	return req.ArtifactPath, nil
}

func languageHandshake(
	ctx context.Context,
	bin string,
	prefix string,
	conn *grpc.ClientConn,
	req *pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	client := pulumirpc.NewLanguageRuntimeClient(conn)
	_, err := client.Handshake(ctx, &pulumirpc.LanguageHandshakeRequest{
		EngineAddress:    req.EngineAddress,
		RootDirectory:    req.RootDirectory,
		ProgramDirectory: req.ProgramDirectory,
	})
	if err != nil {
		status, ok := status.FromError(err)
		if ok && status.Code() == codes.Unimplemented {
			// If the language host doesn't implement Handshake, that's fine -- we'll
			// fall back to existing behaviour.
			slog.Info("Handshake not supported", "bin", bin)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to handshake with '%v': %w", bin, err)
	}

	slog.Info("Handshake success", "bin", bin)
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

func (h *langhost) Link(
	info ProgramInfo, deps []workspace.LinkablePackageDescriptor, loaderTarget string,
) (string, error) {
	slog.Info("langhost.Link executing", "runtime", h.runtime, "info", info, "deps", deps)

	minfo, err := info.Marshal()
	if err != nil {
		return "", err
	}

	packages := []*pulumirpc.LinkRequest_LinkDependency{}
	for _, dep := range deps {
		version := ""
		if dep.Descriptor.Version != nil {
			version = dep.Descriptor.Version.String()
		}

		var parameterization *pulumirpc.PackageParameterization
		if dep.Descriptor.Parameterization != nil {
			parameterization = &pulumirpc.PackageParameterization{
				Name:    dep.Descriptor.Parameterization.Name,
				Version: dep.Descriptor.Parameterization.Version.String(),
				Value:   dep.Descriptor.Parameterization.Value,
			}
		}

		packages = append(packages, &pulumirpc.LinkRequest_LinkDependency{
			Path: dep.Path,
			Package: &pulumirpc.PackageDependency{
				Name:             dep.Descriptor.Name,
				Version:          version,
				Server:           dep.Descriptor.PluginDownloadURL,
				Kind:             string(dep.Descriptor.Kind),
				Parameterization: parameterization,
			},
		})
	}

	res, err := h.client.Link(h.ctx.Request(), &pulumirpc.LinkRequest{
		Info:         minfo,
		Packages:     packages,
		LoaderTarget: loaderTarget,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		slog.Info("langhost.Link failed", "runtime", h.runtime, "info", info, "deps", deps, "err", rpcError)
		return "", rpcError
	}

	slog.Info("langhost.Link success", "runtime", h.runtime, "info", info, "deps", deps)
	return res.ImportInstructions, nil
}

func (h *langhost) Cancel() error {
	slog.Info("langhost.Cancel executing", "runtime", h.runtime)

	_, err := h.client.Cancel(h.ctx.Request(), &emptypb.Empty{})
	if err != nil {
		status, ok := status.FromError(err)
		if ok && status.Code() == codes.Unimplemented {
			slog.Info("langhost.Cancel not implemented by language runtime, skipping", "runtime", h.runtime)
			return nil
		}

		return fmt.Errorf("failed to cancel language runtime: %w", err)
	}

	slog.Info("langhost.Cancel success", "runtime", h.runtime)
	return nil
}
