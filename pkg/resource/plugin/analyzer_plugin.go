// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
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

	"github.com/golang/glog"

	"github.com/pulumi/lumi/pkg/pack"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/tokens"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

const analyzerPrefix = "lumi-analyzer"

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx    *Context
	name   tokens.QName
	plug   *plugin
	client lumirpc.AnalyzerClient
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
		client: lumirpc.NewAnalyzerClient(plug.Conn),
	}, nil
}

func (a *analyzer) Name() tokens.QName { return a.name }

// Analyze analyzes an entire project/stack/snapshot, and returns any errors that it finds.
func (a *analyzer) Analyze(url pack.PackageURL) ([]AnalyzeFailure, error) {
	glog.V(7).Infof("analyzer[%v].Analyze(url=%v) executing", a.name, url)
	req := &lumirpc.AnalyzeRequest{
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
func (a *analyzer) AnalyzeResource(t tokens.Type, props resource.PropertyMap) ([]AnalyzeResourceFailure, error) {
	glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,#props=%v) executing", a.name, t, len(props))
	pstr, unks := MarshalPropertiesWithUnknowns(a.ctx, props, MarshalOptions{
		OldURNs:      true, // permit old URNs, since this is pre-update.
		RawResources: true, // often used during URN creation; IDs won't be ready.
	})
	req := &lumirpc.AnalyzeResourceRequest{
		Type:       string(t),
		Properties: pstr,
		Unknowns:   unks,
	}

	resp, err := a.client.AnalyzeResource(a.ctx.Request(), req)
	if err != nil {
		glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,...) failed: err=%v", a.name, t, err)
		return nil, err
	}

	var failures []AnalyzeResourceFailure
	for _, failure := range resp.GetFailures() {
		failures = append(failures, AnalyzeResourceFailure{
			Property: resource.PropertyKey(failure.Property),
			Reason:   failure.Reason,
		})
	}
	glog.V(7).Infof("analyzer[%v].AnalyzeResource(t=%v,...) success: failures=#%v", a.name, t, len(failures))
	return failures, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
}
