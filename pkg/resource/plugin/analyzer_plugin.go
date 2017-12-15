// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	pbempty "github.com/golang/protobuf/ptypes/empty"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

const AnalyzerPluginPrefix = "pulumi-analyzer-"

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx    *Context
	name   tokens.QName
	plug   *plugin
	client pulumirpc.AnalyzerClient
}

// NewAnalyzer binds to a given analyzer's plugin by name and creates a gRPC connection to it.  If the associated plugin
// could not be found by name on the PATH, or an error occurs while creating the child process, an error is returned.
func NewAnalyzer(host Host, ctx *Context, name tokens.QName) (Analyzer, error) {
	// Go ahead and attempt to load the plugin from the PATH.
	srvexe := AnalyzerPluginPrefix + strings.Replace(string(name), tokens.QNameDelimiter, "_", -1)
	plug, err := newPlugin(ctx, srvexe, fmt.Sprintf("%v (analyzer)", name), []string{host.ServerAddr()})
	if err != nil {
		return nil, err
	} else if plug == nil {
		return nil, nil
	}

	return &analyzer{
		ctx:    ctx,
		name:   name,
		plug:   plug,
		client: pulumirpc.NewAnalyzerClient(plug.Conn),
	}, nil
}

func (a *analyzer) Name() tokens.QName { return a.name }

// label returns a base label for tracing functions.
func (a *analyzer) label() string {
	return fmt.Sprintf("Analyzer[%s]", a.name)
}

// Analyze analyzes a single resource object, and returns any errors that it finds.
func (a *analyzer) Analyze(t tokens.Type, props resource.PropertyMap) ([]AnalyzeFailure, error) {
	label := fmt.Sprintf("%s.Analyze(%s)", a.label(), t)
	glog.V(7).Infof("%s executing (#props=%d)", label, len(props))
	mprops, err := MarshalProperties(props, MarshalOptions{})
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Analyze(a.ctx.Request(), &pulumirpc.AnalyzeRequest{
		Type:       string(t),
		Properties: mprops,
	})
	if err != nil {
		glog.V(7).Infof("%s failed: err=%v", label, err)
		return nil, err
	}

	var failures []AnalyzeFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, AnalyzeFailure{
			Property: resource.PropertyKey(failure.Property),
			Reason:   failure.Reason,
		})
	}
	glog.V(7).Infof("%s success: failures=#%d", label, len(failures))
	return failures, nil
}

// GetPluginInfo returns this plugin's information.
func (a *analyzer) GetPluginInfo() (Info, error) {
	label := fmt.Sprintf("%s.GetPluginInfo()", a.label())
	glog.V(7).Infof("%s executing", label)
	resp, err := a.client.GetPluginInfo(a.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		glog.V(7).Infof("%s failed: err=%v", a.label(), err)
		return Info{}, err
	}
	return Info{
		Name:    a.plug.Bin,
		Type:    AnalyzerType,
		Version: resp.Version,
	}, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
}
