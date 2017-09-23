// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package plugin

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	lumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

const AnalyzerPluginPrefix = "pulumi-analyzer-"

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx    *Context
	name   tokens.QName
	plug   *plugin
	client lumirpc.AnalyzerClient
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
		client: lumirpc.NewAnalyzerClient(plug.Conn),
	}, nil
}

func (a *analyzer) Name() tokens.QName { return a.name }

// Analyze analyzes a single resource object, and returns any errors that it finds.
func (a *analyzer) Analyze(t tokens.Type, props resource.PropertyMap) ([]AnalyzeFailure, error) {
	glog.V(7).Infof("analyzer[%v].Analyze(t=%v,#props=%v) executing", a.name, t, len(props))
	mprops, err := MarshalProperties(props, MarshalOptions{})
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Analyze(a.ctx.Request(), &lumirpc.AnalyzeRequest{
		Type:       string(t),
		Properties: mprops,
	})
	if err != nil {
		glog.V(7).Infof("analyzer[%v].Analyze(t=%v,...) failed: err=%v", a.name, t, err)
		return nil, err
	}

	var failures []AnalyzeFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, AnalyzeFailure{
			Property: resource.PropertyKey(failure.Property),
			Reason:   failure.Reason,
		})
	}
	glog.V(7).Infof("analyzer[%v].Analyze(t=%v,...) success: failures=#%v", a.name, t, len(failures))
	return failures, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
}
