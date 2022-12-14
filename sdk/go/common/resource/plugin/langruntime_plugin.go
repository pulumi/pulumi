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
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
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
func NewLanguageRuntime(host Host, ctx *Context, root, pwd, runtime string,
	options map[string]interface{}) (LanguageRuntime, error) {

	path, err := workspace.GetPluginPath(
		workspace.LanguagePlugin, strings.Replace(runtime, tokens.QNameDelimiter, "_", -1), nil, host.GetProjectPlugins())
	if err != nil {
		return nil, err
	}

	contract.Assert(path != "")

	args, err := buildArgsForNewPlugin(host, root, options)
	if err != nil {
		return nil, err
	}

	plug, err := newPlugin(ctx, pwd, path, runtime,
		workspace.LanguagePlugin, args, nil /*env*/, langRuntimePluginDialOptions(ctx, runtime))
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
		grpc.WithInsecure(),
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
	var args []string

	for k, v := range options {
		args = append(args, fmt.Sprintf("-%s=%v", k, v))
	}

	args = append(args, fmt.Sprintf("-root=%s", filepath.Clean(root)))

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

func (h *langhost) Runtime() string { return h.runtime }

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (h *langhost) GetRequiredPlugins(info ProgInfo) ([]workspace.PluginSpec, error) {
	proj := string(info.Proj.Name)
	logging.V(7).Infof("langhost[%v].GetRequiredPlugins(proj=%s,pwd=%s,program=%s) executing",
		h.runtime, proj, info.Pwd, info.Program)
	resp, err := h.client.GetRequiredPlugins(h.ctx.Request(), &pulumirpc.GetRequiredPluginsRequest{
		Project: proj,
		Pwd:     info.Pwd,
		Program: info.Program,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].GetRequiredPlugins(proj=%s,pwd=%s,program=%s) failed: err=%v",
			h.runtime, proj, info.Pwd, info.Program, rpcError)

		// It's possible this is just an older language host, prior to the emergence of the GetRequiredPlugins
		// method.  In such cases, we will silently error (with the above log left behind).
		if rpcError.Code() == codes.Unimplemented {
			return nil, nil
		}

		return nil, rpcError
	}

	var results []workspace.PluginSpec
	for _, info := range resp.GetPlugins() {
		var version *semver.Version
		if v := info.GetVersion(); v != "" {
			sv, err := semver.ParseTolerant(v)
			if err != nil {
				return nil, errors.Wrapf(err, "illegal semver returned by language host: %s@%s", info.GetName(), v)
			}
			version = &sv
		}
		if !workspace.IsPluginKind(info.GetKind()) {
			return nil, errors.Errorf("unrecognized plugin kind: %s", info.GetKind())
		}
		results = append(results, workspace.PluginSpec{
			Name:              info.GetName(),
			Kind:              workspace.PluginKind(info.GetKind()),
			Version:           version,
			PluginDownloadURL: info.GetServer(),
		})
	}

	logging.V(7).Infof("langhost[%v].GetRequiredPlugins(proj=%s,pwd=%s,program=%s) success: #versions=%d",
		h.runtime, proj, info.Pwd, info.Program, len(results))
	return results, nil
}

// Run executes a program in the language runtime for planning or deployment purposes.  If
// info.DryRun is true, the code must not assume that side-effects or final values resulting from
// resource deployments are actually available.  If it is false, on the other hand, a real
// deployment is occurring and it may safely depend on these.
func (h *langhost) Run(info RunInfo) (string, bool, error) {
	logging.V(7).Infof("langhost[%v].Run(pwd=%v,program=%v,#args=%v,proj=%s,stack=%v,#config=%v,dryrun=%v) executing",
		h.runtime, info.Pwd, info.Program, len(info.Args), info.Project, info.Stack, len(info.Config), info.DryRun)
	config := make(map[string]string, len(info.Config))
	for k, v := range info.Config {
		config[k.String()] = v
	}
	configSecretKeys := make([]string, len(info.ConfigSecretKeys))
	for i, k := range info.ConfigSecretKeys {
		configSecretKeys[i] = k.String()
	}
	resp, err := h.client.Run(h.ctx.Request(), &pulumirpc.RunRequest{
		MonitorAddress:   info.MonitorAddress,
		Pwd:              info.Pwd,
		Program:          info.Program,
		Args:             info.Args,
		Project:          info.Project,
		Stack:            info.Stack,
		Config:           config,
		ConfigSecretKeys: configSecretKeys,
		DryRun:           info.DryRun,
		QueryMode:        info.QueryMode,
		Parallel:         int32(info.Parallel),
		Organization:     info.Organization,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].Run(pwd=%v,program=%v,...,dryrun=%v) failed: err=%v",
			h.runtime, info.Pwd, info.Program, info.DryRun, rpcError)
		return "", false, rpcError
	}

	progerr := resp.GetError()
	bail := resp.GetBail()
	logging.V(7).Infof("langhost[%v].RunPlan(pwd=%v,program=%v,...,dryrun=%v) success: progerr=%v",
		h.runtime, info.Pwd, info.Program, info.DryRun, progerr)
	return progerr, bail, nil
}

// GetPluginInfo returns this plugin's information.
func (h *langhost) GetPluginInfo() (workspace.PluginInfo, error) {
	logging.V(7).Infof("langhost[%v].GetPluginInfo() executing", h.runtime)

	plugInfo := workspace.PluginInfo{
		Name: h.runtime,
		Kind: workspace.LanguagePlugin,
	}

	plugInfo.Path = h.plug.Bin

	resp, err := h.client.GetPluginInfo(h.ctx.Request(), &pbempty.Empty{})
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

func (h *langhost) InstallDependencies(directory string) error {
	logging.V(7).Infof("langhost[%v].InstallDependencies(directory=%s) executing",
		h.runtime, directory)
	resp, err := h.client.InstallDependencies(h.ctx.Request(), &pulumirpc.InstallDependenciesRequest{
		Directory:  directory,
		IsTerminal: cmdutil.GetGlobalColorization() != colors.Never,
	})

	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("langhost[%v].InstallDependencies(directory=%s) failed: err=%v",
			h.runtime, directory, rpcError)

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
			logging.V(7).Infof("langhost[%v].InstallDependencies(directory=%s) failed: err=%v",
				h.runtime, directory, rpcError)
			return rpcError
		}

		if len(output.Stdout) != 0 {
			os.Stdout.Write(output.Stdout)
		}

		if len(output.Stderr) != 0 {
			os.Stderr.Write(output.Stderr)
		}
	}

	logging.V(7).Infof("langhost[%v].InstallDependencies(directory=%s) success",
		h.runtime, directory)
	return nil

}

func (h *langhost) About() (AboutInfo, error) {
	logging.V(7).Infof("langhost[%v].About() executing", h.runtime)
	resp, err := h.client.About(h.ctx.Request(), &pbempty.Empty{})
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

func (h *langhost) GetProgramDependencies(info ProgInfo, transitiveDependencies bool) ([]DependencyInfo, error) {
	proj := string(info.Proj.Name)
	prefix := fmt.Sprintf("langhost[%v].GetProgramDependencies(proj=%s,pwd=%s,program=%s,transitiveDependencies=%t)",
		h.runtime, proj, info.Pwd, info.Program, transitiveDependencies)

	logging.V(7).Infof("%s executing", prefix)
	resp, err := h.client.GetProgramDependencies(h.ctx.Request(), &pulumirpc.GetProgramDependenciesRequest{
		Project:                proj,
		Pwd:                    info.Pwd,
		Program:                info.Program,
		TransitiveDependencies: transitiveDependencies,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", prefix, rpcError)

		return nil, rpcError
	}

	var results []DependencyInfo
	for _, dep := range resp.GetDependencies() {
		var version semver.Version
		if v := dep.Version; v != "" {
			version, err = semver.ParseTolerant(v)
			if err != nil {
				return nil, errors.Wrapf(err, "illegal semver returned by language host: %s@%s", dep.Name, v)
			}
		}
		results = append(results, DependencyInfo{
			Name:    dep.Name,
			Version: version,
		})
	}

	logging.V(7).Infof("%s success: #versions=%d", prefix, len(results))
	return results, nil
}

func (h *langhost) RunPlugin(info RunPluginInfo) (io.Reader, io.Reader, context.CancelFunc, error) {
	logging.V(7).Infof("langhost[%v].RunPlugin(pwd=%s,program=%s) executing",
		h.runtime, info.Pwd, info.Program)

	ctx, kill := context.WithCancel(h.ctx.Request())

	resp, err := h.client.RunPlugin(ctx, &pulumirpc.RunPluginRequest{
		Pwd:     info.Pwd,
		Program: info.Program,
		Args:    info.Args,
		Env:     info.Env,
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
				contract.AssertNoError(err)
				contract.Assert(n == len(value.Stdout))
			} else if value, ok := msg.Output.(*pulumirpc.RunPluginResponse_Stderr); ok {
				n, err := errw.Write(value.Stderr)
				contract.AssertNoError(err)
				contract.Assert(n == len(value.Stderr))
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
