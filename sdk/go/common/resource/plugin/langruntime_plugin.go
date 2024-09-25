// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// langhost reflects a language host plugin, loaded dynamically for a single language/runtime pair.
type langhost struct {
	ctx     *Context
	runtime string
	plug    *plugin
	client  pulumirpc.LanguageRuntimeClient
}

// NewLanguageRuntime binds to a language's runtime plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewLanguageRuntime(host Host, ctx *Context, runtime, workingDirectory string, info ProgramInfo,
) (LanguageRuntime, error) {
	path, err := workspace.GetPluginPath(ctx.Diag,
		apitype.LanguagePlugin, strings.ReplaceAll(runtime, tokens.QNameDelimiter, "_"), nil, host.GetProjectPlugins())
	if err != nil {
		return nil, err
	}

	contract.Assertf(path != "", "unexpected empty path for language plugin %s", runtime)

	args, err := buildArgsForNewPlugin(host, info.RootDirectory(), info.Options())
	if err != nil {
		return nil, err
	}

	plug, err := newPlugin(ctx, workingDirectory, path, runtime,
		apitype.LanguagePlugin, args, nil /*env*/, langRuntimePluginDialOptions(ctx, runtime))
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil language plugin for %s", runtime)

	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		plug:    plug,
		client:  pulumirpc.NewLanguageRuntimeClient(plug.Conn),
	}, nil
}

func langRuntimePluginDialOptions(ctx *Context, runtime string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]interface{}{
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

func buildArgsForNewPlugin(host Host, root string, options map[string]interface{}) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	args := slice.Prealloc[string](len(options))

	for k, v := range options {
		args = append(args, fmt.Sprintf("-%s=%v", k, v))
	}

	args = append(args, "-root="+filepath.Clean(root))

	// NOTE: positional argument for the server addresss must come last
	args = append(args, host.ServerAddr())

	return args, nil
}

func NewLanguageRuntimeClient(ctx *Context, runtime string, client pulumirpc.LanguageRuntimeClient) LanguageRuntime {
	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		client:  client,
	}
}

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (h *langhost) GetRequiredPlugins(info ProgramInfo) ([]workspace.PluginSpec, error) {
	logging.V(7).Infof("langhost[%v].GetRequiredPlugins(%s) executing",
		h.runtime, info)

	minfo, err := info.Marshal()
	if err != nil {
		return nil, err
	}

	resp, err := h.client.GetRequiredPlugins(h.ctx.Request(), &pulumirpc.GetRequiredPluginsRequest{
		Project: "deprecated",
		Pwd:     info.ProgramDirectory(),
		Program: info.EntryPoint(),
		Info:    minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].GetRequiredPlugins(%s) failed: err=%v",
			h.runtime, info, rpcError)

		// It's possible this is just an older language host, prior to the emergence of the GetRequiredPlugins
		// method.  In such cases, we will silently error (with the above log left behind).
		if rpcError.Code() == codes.Unimplemented {
			return nil, nil
		}

		return nil, rpcError
	}

	results := slice.Prealloc[workspace.PluginSpec](len(resp.GetPlugins()))
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
		results = append(results, workspace.PluginSpec{
			Name:              info.Name,
			Kind:              apitype.PluginKind(info.Kind),
			Version:           version,
			PluginDownloadURL: info.Server,
			Checksums:         info.Checksums,
		})
	}

	logging.V(7).Infof("langhost[%v].GetRequiredPlugins(%s) success: #versions=%d",
		h.runtime, info, len(results))
	return results, nil
}

// Run executes a program in the language runtime for planning or deployment purposes.  If
// info.DryRun is true, the code must not assume that side-effects or final values resulting from
// resource deployments are actually available.  If it is false, on the other hand, a real
// deployment is occurring and it may safely depend on these.
func (h *langhost) Run(info RunInfo) (string, bool, error) {
	logging.V(7).Infof("langhost[%v].Run(pwd=%v,%s,#args=%v,proj=%s,stack=%v,#config=%v,dryrun=%v) executing",
		h.runtime, info.Pwd, info.Info, len(info.Args), info.Project, info.Stack, len(info.Config), info.DryRun)
	config := make(map[string]string, len(info.Config))
	for k, v := range info.Config {
		config[k.String()] = v
	}
	configSecretKeys := make([]string, len(info.ConfigSecretKeys))
	for i, k := range info.ConfigSecretKeys {
		configSecretKeys[i] = k.String()
	}
	configPropertyMap, err := MarshalProperties(info.ConfigPropertyMap,
		MarshalOptions{RejectUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
	if err != nil {
		return "", false, err
	}

	minfo, err := info.Info.Marshal()
	if err != nil {
		return "", false, err
	}

	resp, err := h.client.Run(h.ctx.Request(), &pulumirpc.RunRequest{
		MonitorAddress:    info.MonitorAddress,
		Pwd:               info.Pwd,
		Program:           info.Info.EntryPoint(),
		Args:              info.Args,
		Project:           info.Project,
		Stack:             info.Stack,
		Config:            config,
		ConfigSecretKeys:  configSecretKeys,
		ConfigPropertyMap: configPropertyMap,
		DryRun:            info.DryRun,
		QueryMode:         info.QueryMode,
		Parallel:          info.Parallel,
		Organization:      info.Organization,
		Info:              minfo,
		LoaderTarget:      info.LoaderAddress,
		AttachDebugger:    info.AttachDebugger,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].Run(pwd=%v,%s,...,dryrun=%v) failed: err=%v",
			h.runtime, info.Pwd, info.Info, info.DryRun, rpcError)
		return "", false, rpcError
	}

	progerr := resp.GetError()
	bail := resp.GetBail()
	logging.V(7).Infof("langhost[%v].Run(pwd=%v,%s,...,dryrun=%v) success: progerr=%v, bail=%v",
		h.runtime, info.Pwd, info.Info, info.DryRun, progerr, bail)
	return progerr, bail, nil
}

// GetPluginInfo returns this plugin's information.
func (h *langhost) GetPluginInfo() (workspace.PluginInfo, error) {
	logging.V(7).Infof("langhost[%v].GetPluginInfo() executing", h.runtime)

	plugInfo := workspace.PluginInfo{
		Name: h.runtime,
		Kind: apitype.LanguagePlugin,
	}

	if h.plug != nil {
		plugInfo.Path = h.plug.Bin
	}

	resp, err := h.client.GetPluginInfo(h.ctx.Request(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].GetPluginInfo() failed: err=%v", h.runtime, rpcError)
		return workspace.PluginInfo{}, rpcError
	}
	vers := resp.Version

	if vers != "" {
		sv, err := semver.ParseTolerant(vers)
		if err != nil {
			return workspace.PluginInfo{}, err
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

func (h *langhost) InstallDependencies(request InstallDependenciesRequest) error {
	logging.V(7).Infof("langhost[%v].InstallDependencies(%s) executing",
		h.runtime, request)

	minfo, err := request.Info.Marshal()
	if err != nil {
		return err
	}

	resp, err := h.client.InstallDependencies(h.ctx.Request(), &pulumirpc.InstallDependenciesRequest{
		Directory:               request.Info.ProgramDirectory(),
		IsTerminal:              cmdutil.GetGlobalColorization() != colors.Never,
		Info:                    minfo,
		UseLanguageVersionTools: request.UseLanguageVersionTools,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].InstallDependencies(%s) failed: err=%v",
			h.runtime, request, rpcError)

		// It's possible this is just an older language host, prior to the emergence of the InstallDependencies
		// method.  In such cases, we will silently error (with the above log left behind).
		if rpcError.Code() == codes.Unimplemented {
			return nil
		}

		return rpcError
	}

	for {
		output, err := resp.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			rpcError := rpcerror.Convert(err)
			logging.V(7).Infof("langhost[%v].InstallDependencies(%s) failed: err=%v",
				h.runtime, request, rpcError)
			return rpcError
		}

		if len(output.Stdout) != 0 {
			os.Stdout.Write(output.Stdout)
		}

		if len(output.Stderr) != 0 {
			os.Stderr.Write(output.Stderr)
		}
	}

	logging.V(7).Infof("langhost[%v].InstallDependencies(%s) success",
		h.runtime, request)
	return nil
}

func (h *langhost) RuntimeOptionsPrompts(info ProgramInfo) ([]RuntimeOptionPrompt, error) {
	logging.V(7).Infof("langhost[%v].RuntimeOptionsPrompts() executing", h.runtime)

	minfo, err := info.Marshal()
	if err != nil {
		return []RuntimeOptionPrompt{}, err
	}

	resp, err := h.client.RuntimeOptionsPrompts(h.ctx.Request(), &pulumirpc.RuntimeOptionsRequest{
		Info: minfo,
	})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			logging.V(7).Infof("langhost[%v].RuntimeOptionsPrompts() not implemented, returning no prompts", h.runtime)
			return []RuntimeOptionPrompt{}, nil
		}
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].RuntimeOptionsPrompts() failed: err=%v", h.runtime, rpcError)
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

	logging.V(7).Infof("langhost[%v].RuntimeOptionsPrompts() success", h.runtime)
	return prompts, nil
}

func (h *langhost) About(info ProgramInfo) (AboutInfo, error) {
	logging.V(7).Infof("langhost[%v].About() executing", h.runtime)
	minfo, err := info.Marshal()
	if err != nil {
		return AboutInfo{}, err
	}
	resp, err := h.client.About(h.ctx.Request(), &pulumirpc.AboutRequest{
		Info: minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].About() failed: err=%v", h.runtime, rpcError)

		return AboutInfo{}, rpcError
	}

	result := AboutInfo{
		Executable: resp.Executable,
		Version:    resp.Version,
		Metadata:   resp.Metadata,
	}

	logging.V(7).Infof("langhost[%v].About() success", h.runtime)
	return result, nil
}

func (h *langhost) GetProgramDependencies(info ProgramInfo, transitiveDependencies bool) ([]DependencyInfo, error) {
	prefix := fmt.Sprintf("langhost[%v].GetProgramDependencies(%s)", h.runtime, info.String())
	minfo, err := info.Marshal()
	if err != nil {
		return nil, err
	}

	logging.V(7).Infof("%s executing", prefix)
	resp, err := h.client.GetProgramDependencies(h.ctx.Request(), &pulumirpc.GetProgramDependenciesRequest{
		TransitiveDependencies: transitiveDependencies,
		Info:                   minfo,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", prefix, rpcError)

		return nil, rpcError
	}

	results := slice.Prealloc[DependencyInfo](len(resp.GetDependencies()))
	for _, dep := range resp.GetDependencies() {
		results = append(results, DependencyInfo{
			Name:    dep.Name,
			Version: dep.Version,
		})
	}

	logging.V(7).Infof("%s success: #versions=%d", prefix, len(results))
	return results, nil
}

func (h *langhost) RunPlugin(info RunPluginInfo) (io.Reader, io.Reader, context.CancelFunc, error) {
	logging.V(7).Infof("langhost[%v].RunPlugin(%s) executing",
		h.runtime, info.Info.String())

	minfo, err := info.Info.Marshal()
	if err != nil {
		return nil, nil, nil, err
	}

	ctx, kill := context.WithCancel(h.ctx.Request())

	resp, err := h.client.RunPlugin(ctx, &pulumirpc.RunPluginRequest{
		Pwd:  minfo.GetProgramDirectory(),
		Args: info.Args,
		Env:  info.Env,
		Info: minfo,
	})
	if err != nil {
		// If there was an error starting the plugin kill the context for this request to ensure any lingering
		// connection terminates.
		kill()
		return nil, nil, nil, err
	}

	outr, outw := io.Pipe()
	errr, errw := io.Pipe()

	go func() {
		for {
			logging.V(10).Infoln("Waiting for plugin message")
			msg, err := resp.Recv()
			if err != nil {
				contract.IgnoreError(outw.CloseWithError(err))
				contract.IgnoreError(errw.CloseWithError(err))
				break
			}

			logging.V(10).Infoln("Got plugin response: ", msg)

			if value, ok := msg.Output.(*pulumirpc.RunPluginResponse_Stdout); ok {
				n, err := outw.Write(value.Stdout)
				contract.AssertNoErrorf(err, "failed to write to stdout pipe: %v", err)
				contract.Assertf(n == len(value.Stdout), "wrote fewer bytes (%d) than expected (%d)", n, len(value.Stdout))
			} else if value, ok := msg.Output.(*pulumirpc.RunPluginResponse_Stderr); ok {
				n, err := errw.Write(value.Stderr)
				contract.AssertNoErrorf(err, "failed to write to stderr pipe: %v", err)
				contract.Assertf(n == len(value.Stderr), "wrote fewer bytes (%d) than expected (%d)", n, len(value.Stderr))
			} else if _, ok := msg.Output.(*pulumirpc.RunPluginResponse_Exitcode); ok {
				// If stdout and stderr are empty we've flushed and are returning the exit code
				outw.Close()
				errw.Close()
				break
			}
		}
	}()

	return outr, errr, kill, nil
}

func (h *langhost) GenerateProject(
	sourceDirectory, targetDirectory, project string, strict bool,
	loaderTarget string, localDependencies map[string]string,
) (hcl.Diagnostics, error) {
	logging.V(7).Infof("langhost[%v].GenerateProject() executing", h.runtime)
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
		logging.V(7).Infof("langhost[%v].GenerateProject() failed: err=%v", h.runtime, rpcError)
		return nil, rpcError
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	logging.V(7).Infof("langhost[%v].GenerateProject() success", h.runtime)
	return diags, nil
}

func (h *langhost) GeneratePackage(
	directory string, schema string, extraFiles map[string][]byte,
	loaderTarget string, localDependencies map[string]string,
	local bool,
) (hcl.Diagnostics, error) {
	logging.V(7).Infof("langhost[%v].GeneratePackage() executing", h.runtime)
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
		logging.V(7).Infof("langhost[%v].GeneratePackage() failed: err=%v", h.runtime, rpcError)
		return nil, rpcError
	}

	logging.V(7).Infof("langhost[%v].GeneratePackage() success", h.runtime)

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	return diags, nil
}

func (h *langhost) GenerateProgram(program map[string]string, loaderTarget string, strict bool,
) (map[string][]byte, hcl.Diagnostics, error) {
	logging.V(7).Infof("langhost[%v].GenerateProgram() executing", h.runtime)
	resp, err := h.client.GenerateProgram(h.ctx.Request(), &pulumirpc.GenerateProgramRequest{
		Source:       program,
		LoaderTarget: loaderTarget,
		Strict:       strict,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].GenerateProgram() failed: err=%v", h.runtime, rpcError)
		return nil, nil, rpcError
	}

	// Translate the rpc diagnostics into hcl.Diagnostics.
	var diags hcl.Diagnostics
	for _, rpcDiag := range resp.Diagnostics {
		diags = append(diags, RPCDiagnosticToHclDiagnostic(rpcDiag))
	}

	logging.V(7).Infof("langhost[%v].GenerateProgram() success", h.runtime)
	return resp.Source, diags, nil
}

func (h *langhost) Pack(
	packageDirectory string, destinationDirectory string,
) (string, error) {
	label := fmt.Sprintf("langhost[%v].Pack(%s, %s)", h.runtime, packageDirectory, destinationDirectory)
	logging.V(7).Infof("%s executing", label)

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
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return "", rpcError
	}

	logging.V(7).Infof("%s success: artifactPath=%s", label, req.ArtifactPath)
	return req.ArtifactPath, nil
}
