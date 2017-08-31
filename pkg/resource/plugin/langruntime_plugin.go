// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/pulumi/pulumi-fabric/pkg/tokens"
	"github.com/pulumi/pulumi-fabric/pkg/workspace"
	lumirpc "github.com/pulumi/pulumi-fabric/sdk/proto/go"
)

const LanguagePluginPrefix = "lumi-langhost-"

// langhost reflects a language host plugin, loaded dynamically for a single language/runtime pair.
type langhost struct {
	ctx     *Context
	runtime string
	plug    *plugin
	client  lumirpc.LanguageRuntimeClient
}

// NewLanguageRuntime binds to a language's runtime plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewLanguageRuntime(host Host, ctx *Context, runtime string) (LanguageRuntime, error) {
	// Setup the search paths; first, the naked name (found on the PATH); next, the fully qualified name.
	srvexe := LanguagePluginPrefix + strings.Replace(runtime, tokens.QNameDelimiter, "_", -1)
	paths := []string{
		srvexe, // naked PATH.
		filepath.Join(workspace.InstallRoot(), srvexe), // qualified name.
	}

	// Now go ahead and attempt to load the plugin.
	plug, err := newPlugin(host, ctx, paths, fmt.Sprintf("langhost[%v]", runtime))
	if err != nil {
		return nil, err
	}

	return &langhost{
		ctx:     ctx,
		runtime: runtime,
		plug:    plug,
		client:  lumirpc.NewLanguageRuntimeClient(plug.Conn),
	}, nil
}

func (h *langhost) Runtime() string { return h.runtime }

// Run executes a program in the language runtime for planning or deployment purposes.  If info.DryRun is true,
// the code must not assume that side-effects or final values resulting from resource deployments are actually
// available.  If it is false, on the other hand, a real deployment is occurring and it may safely depend on these.
func (h *langhost) Run(info RunInfo) (string, error) {
	glog.V(7).Infof("langhost[%v].Run(pwd=%v,program=%v,#args=%v,#config=%v,dryrun=%v) executing",
		h.runtime, info.Pwd, info.Program, len(info.Args), len(info.Config), info.DryRun)
	req := lumirpc.RunRequest(info)
	resp, err := h.client.Run(h.ctx.Request(), &req)
	if err != nil {
		glog.V(7).Infof("resource[%v].Run(pwd=%v,program=%v,...,dryrun=%v) failed: err=%v",
			info.Pwd, info.Program, info.DryRun, err)
		return "", err
	}

	progerr := resp.GetError()
	glog.V(7).Infof("resource[%v].RunPlan(pwd=%v,program=%v,...,dryrun=%v) success: progerr=%v",
		info.Pwd, info.Program, info.DryRun, progerr)
	return progerr, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (h *langhost) Close() error {
	return h.plug.Close()
}
