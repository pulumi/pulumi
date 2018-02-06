// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"strings"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
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
func NewLanguageRuntime(host Host, ctx *Context, runtime string, monitorAddr string) (LanguageRuntime, error) {
	// Load the plugin's path by using the standard workspace logic.
	path, err := workspace.GetPluginPath(
		workspace.LanguagePlugin, strings.Replace(runtime, tokens.QNameDelimiter, "_", -1), nil)
	if err != nil {
		return nil, err
	} else if path == "" {
		return nil, errors.Errorf("no plugin for language %s found in the workspace or on your $PATH", runtime)
	}

	plug, err := newPlugin(ctx, path, runtime, []string{monitorAddr, host.ServerAddr()})
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

func (h *langhost) Runtime() string { return h.runtime }

// Run executes a program in the language runtime for planning or deployment purposes.  If info.DryRun is true,
// the code must not assume that side-effects or final values resulting from resource deployments are actually
// available.  If it is false, on the other hand, a real deployment is occurring and it may safely depend on these.
func (h *langhost) Run(info RunInfo) (string, error) {
	glog.V(7).Infof("langhost[%v].Run(pwd=%v,program=%v,#args=%v,proj=%s,stack=%v,#config=%v,dryrun=%v) executing",
		h.runtime, info.Pwd, info.Program, len(info.Args), info.Project, info.Stack, len(info.Config), info.DryRun)
	config := make(map[string]string)
	for k, v := range info.Config {
		config[string(k)] = v
	}
	resp, err := h.client.Run(h.ctx.Request(), &pulumirpc.RunRequest{
		Pwd:      info.Pwd,
		Program:  info.Program,
		Args:     info.Args,
		Project:  info.Project,
		Stack:    info.Stack,
		Config:   config,
		DryRun:   info.DryRun,
		Parallel: int32(info.Parallel),
	})
	if err != nil {
		glog.V(7).Infof("langhost[%v].Run(pwd=%v,program=%v,...,dryrun=%v) failed: err=%v",
			h.runtime, info.Pwd, info.Program, info.DryRun, err)
		return "", err
	}

	progerr := resp.GetError()
	glog.V(7).Infof("langhost[%v].RunPlan(pwd=%v,program=%v,...,dryrun=%v) success: progerr=%v",
		h.runtime, info.Pwd, info.Program, info.DryRun, progerr)
	return progerr, nil
}

// GetPluginInfo returns this plugin's information.
func (h *langhost) GetPluginInfo() (Info, error) {
	glog.V(7).Infof("langhost[%v].GetPluginInfo() executing", h.runtime)
	resp, err := h.client.GetPluginInfo(h.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		glog.V(7).Infof("langhost[%v].GetPluginInfo() failed: err=%v", h.runtime, err)
		return Info{}, err
	}
	return Info{
		Name:    h.plug.Bin,
		Type:    LanguageType,
		Version: resp.Version,
	}, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (h *langhost) Close() error {
	return h.plug.Close()
}
