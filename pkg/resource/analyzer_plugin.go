// Copyright 2016 Pulumi, Inc. All rights reserved.

package resource

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/sdk/go/pkg/cocorpc"
)

const analyzerPrefix = "coco-analyzer"

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx    *Context
	name   tokens.QName
	plug   *plugin
	client cocorpc.AnalyzerClient
}

// NewAnalyzer binds to a given analyzer's plugin by name and creates a gRPC connection to it.  If the associated plugin
// could not be found by name on the PATH, or an error occurs while creating the child process, an error is returned.
func NewAnalyzer(ctx *Context, name tokens.QName) (Analyzer, error) {
	// Search for the analyzer on the path.
	srvexe := analyzerPrefix + "-" + strings.Replace(string(name), tokens.QNameDelimiter, "_", -1)

	// Now go ahead and attempt to load the plugin.
	plug, err := newPlugin(ctx, []string{srvexe}, fmt.Sprintf("analyzer[%v]", name))
	if err != nil {
		return nil, err
	}

	return &analyzer{
		ctx:    ctx,
		name:   name,
		plug:   plug,
		client: cocorpc.NewAnalyzerClient(plug.Conn),
	}, nil
}

func (a *analyzer) Name() tokens.QName { return a.name }

// Analyze analyzes an entire project/stack/snapshot, and returns any errors that it finds.
func (a *analyzer) Analyze(url pack.PackageURL) ([]AnalyzeFailure, error) {
	glog.V(7).Infof("analyzer[%v].Analyze(url=%v) executing", a.name, url)
	req := &cocorpc.AnalyzeRequest{
		Pkg: url.String(),
	}

	resp, err := a.client.Analyze(a.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("analyzer[%v].Analyze(url=%v) failed: err=%v", a.name, url, err)
		return nil, err
	}

	var failures []AnalyzeFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, AnalyzeFailure{failure.Reason})
	}
	glog.V(7).Infof("analyzer[%v].Analyze(url=%v) success: failures=#%v", a.name, url, len(failures))
	return failures, nil
}

// AnalyzeResource analyzes a single resource object, and returns any errors that it finds.
func (a *analyzer) AnalyzeResource(t tokens.Type, props PropertyMap) ([]AnalyzeResourceFailure, error) {
	glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,#props=%v) executing", a.name, t, len(props))
	req := &cocorpc.AnalyzeResourceRequest{
		Type: string(t),
		Properties: MarshalProperties(a.ctx, props, MarshalOptions{
			PermitOlds: true, // permit old URNs, since this is pre-update.
			RawURNs:    true, // often used during URN creation; IDs won't be ready.
		}),
	}

	resp, err := a.client.AnalyzeResource(a.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,...) failed: err=%v", a.name, t, err)
		return nil, err
	}

	var failures []AnalyzeResourceFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, AnalyzeResourceFailure{PropertyKey(failure.Property), failure.Reason})
	}
	glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,...) success: failures=#%v", a.name, t, len(failures))
	return failures, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
}
