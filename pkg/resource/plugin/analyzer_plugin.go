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
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/pkg/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx    *Context
	name   tokens.QName
	plug   *plugin
	client pulumirpc.AnalyzerClient
}

var _ Analyzer = (*analyzer)(nil)

// NewAnalyzer binds to a given analyzer's plugin by name and creates a gRPC connection to it.  If the associated plugin
// could not be found by name on the PATH, or an error occurs while creating the child process, an error is returned.
func NewAnalyzer(host Host, ctx *Context, name tokens.QName) (Analyzer, error) {
	// Load the plugin's path by using the standard workspace logic.
	_, path, err := workspace.GetPluginPath(
		workspace.AnalyzerPlugin, strings.Replace(string(name), tokens.QNameDelimiter, "_", -1), nil)
	if err != nil {
		return nil, rpcerror.Convert(err)
	} else if path == "" {
		return nil, workspace.NewMissingError(workspace.PluginInfo{
			Kind: workspace.AnalyzerPlugin,
			Name: string(name),
		})
	}

	plug, err := newPlugin(ctx, path, fmt.Sprintf("%v (analyzer)", name),
		[]string{host.ServerAddr(), ctx.Pwd})
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil analyzer plugin for %s", name)

	return &analyzer{
		ctx:    ctx,
		name:   name,
		plug:   plug,
		client: pulumirpc.NewAnalyzerClient(plug.Conn),
	}, nil
}

const policyAnalyzerName = "policy"

// NewPolicyAnalyzer boots the nodejs analyzer plugin located at `policyPackpath`
func NewPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, policyPackPath string) (Analyzer, error) {

	// Load the policy-booting analyzer plugin (i.e., `pulumi-analyzer-${policyAnalyzerName}`).
	_, pluginPath, err := workspace.GetPluginPath(
		workspace.AnalyzerPlugin, policyAnalyzerName, nil)
	if err != nil {
		return nil, rpcerror.Convert(err)
	} else if pluginPath == "" {
		return nil, workspace.NewMissingError(workspace.PluginInfo{
			Kind: workspace.AnalyzerPlugin,
			Name: string(name),
		})
	}

	plug, err := newPlugin(ctx, pluginPath, fmt.Sprintf("%v (analyzer)", name),
		[]string{host.ServerAddr(), policyPackPath})
	if err != nil {
		return nil, err
	}
	contract.Assertf(plug != nil, "unexpected nil analyzer plugin for %s", name)

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
func (a *analyzer) Analyze(
	t tokens.Type, props resource.PropertyMap) ([]AnalyzeDiagnostic, error) {

	label := fmt.Sprintf("%s.Analyze(%s)", a.label(), t)
	logging.V(7).Infof("%s executing (#props=%d)", label, len(props))
	mprops, err := MarshalProperties(props, MarshalOptions{})
	if err != nil {
		return nil, err
	}

	fmt.Println("Analyzing")
	resp, err := a.client.Analyze(a.ctx.Request(), &pulumirpc.AnalyzeRequest{
		Type:       string(t),
		Properties: mprops,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return nil, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s success: failures=#%d", label, len(failures))

	diags := []AnalyzeDiagnostic{}
	for _, failure := range failures {
		enforcementLevel, err := convertEnforcementLevel(failure.EnforcementLevel)
		if err != nil {
			return nil, err
		}

		diags = append(diags, AnalyzeDiagnostic{
			PolicyName:        failure.PolicyName,
			PolicyPackName:    failure.PolicyPackName,
			PolicyPackVersion: failure.PolicyPackVersion,
			Description:       failure.Description,
			Message:           failure.Message,
			Tags:              failure.Tags,
			EnforcementLevel:  enforcementLevel,
		})
	}

	return diags, nil
}

// GetPluginInfo returns this plugin's information.
func (a *analyzer) GetAnalyzerInfo() (AnalyzerInfo, error) {
	label := fmt.Sprintf("%s.GetAnalyzerInfo()", a.label())
	logging.V(7).Infof("%s executing", label)
	resp, err := a.client.GetAnalyzerInfo(a.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", a.label(), rpcError)
		return AnalyzerInfo{}, rpcError
	}

	policies := []apitype.Policy{}
	for _, p := range resp.GetPolicies() {
		enforcementLevel, err := convertEnforcementLevel(p.EnforcementLevel)
		if err != nil {
			return AnalyzerInfo{}, err
		}

		policies = append(policies, apitype.Policy{
			Name:             p.GetName(),
			DisplayName:      p.GetDisplayName(),
			Description:      p.GetDescription(),
			EnforcementLevel: enforcementLevel,
			Message:          p.GetMessage(),
		})
	}

	return AnalyzerInfo{
		Name:        resp.GetName(),
		DisplayName: resp.GetDisplayName(),
		Policies:    policies,
	}, nil
}

// GetPluginInfo returns this plugin's information.
func (a *analyzer) GetPluginInfo() (workspace.PluginInfo, error) {
	label := fmt.Sprintf("%s.GetPluginInfo()", a.label())
	logging.V(7).Infof("%s executing", label)
	resp, err := a.client.GetPluginInfo(a.ctx.Request(), &pbempty.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", a.label(), rpcError)
		return workspace.PluginInfo{}, rpcError
	}

	var version *semver.Version
	if v := resp.Version; v != "" {
		sv, err := semver.ParseTolerant(v)
		if err != nil {
			return workspace.PluginInfo{}, err
		}
		version = &sv
	}

	return workspace.PluginInfo{
		Name:    string(a.name),
		Path:    a.plug.Bin,
		Kind:    workspace.AnalyzerPlugin,
		Version: version,
	}, nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
}

func convertEnforcementLevel(el pulumirpc.EnforcementLevel) (apitype.EnforcementLevel, error) {
	switch el {
	case pulumirpc.EnforcementLevel_ADVISORY:
		return apitype.Advisory, nil
	case pulumirpc.EnforcementLevel_MANDATORY:
		return apitype.Mandatory, nil

	default:
		return "", fmt.Errorf("Invalid enforcement level %d", el)
	}
}
