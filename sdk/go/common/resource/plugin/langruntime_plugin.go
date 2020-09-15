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
	"fmt"
	"strings"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v2/proto/go"
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
func NewLanguageRuntime(host Host, ctx *Context, runtime string,
	options map[string]interface{}) (LanguageRuntime, error) {

	_, path, err := workspace.GetPluginPath(
		workspace.LanguagePlugin, strings.Replace(runtime, tokens.QNameDelimiter, "_", -1), nil)
	if err != nil {
		return nil, err
	} else if path == "" {
		return nil, workspace.NewMissingError(workspace.PluginInfo{
			Kind: workspace.LanguagePlugin,
			Name: runtime,
		})
	}

	var args []string
	for k, v := range options {
		args = append(args, fmt.Sprintf("-%s=%v", k, v))
	}
	args = append(args, host.ServerAddr())

	plug, err := newPlugin(ctx, ctx.Pwd, path, runtime, args, nil /*env*/)
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

func NewLanguageRuntimeClient(ctx *Context, runtime string, client pulumirpc.LanguageRuntimeClient) LanguageRuntime {
	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		client:  client,
	}
}

func (h *langhost) Runtime() string { return h.runtime }

// GetRequiredPlugins computes the complete set of anticipated plugins required by a program.
func (h *langhost) GetRequiredPlugins(info ProgInfo) ([]workspace.PluginInfo, error) {
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

	var results []workspace.PluginInfo
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
		results = append(results, workspace.PluginInfo{
			Name:      info.GetName(),
			Kind:      workspace.PluginKind(info.GetKind()),
			Version:   version,
			ServerURL: info.GetServer(),
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
	config := make(map[string]string)
	for k, v := range info.Config {
		config[k.String()] = v
	}
	resp, err := h.client.Run(h.ctx.Request(), &pulumirpc.RunRequest{
		MonitorAddress: info.MonitorAddress,
		Pwd:            info.Pwd,
		Program:        info.Program,
		Args:           info.Args,
		Project:        info.Project,
		Stack:          info.Stack,
		Config:         config,
		DryRun:         info.DryRun,
		QueryMode:      info.QueryMode,
		Parallel:       int32(info.Parallel),
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
