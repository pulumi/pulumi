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

	plug, err := newPlugin(ctx, ctx.Pwd, path, fmt.Sprintf("%v (analyzer)", name),
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
		return nil, fmt.Errorf("could not start policy pack %q because the built-in analyzer "+
			"plugin that runs policy plugins is missing. This might occur when the plugin "+
			"directory is not on your $PATH, or when the installed version of the Pulumi SDK "+
			"does not support resource policies", string(name))
	}

	// The `pulumi-analyzer-policy` plugin is a script that looks for the '@pulumi/pulumi/cmd/run-policy-pack'
	// node module and runs it with node. To allow non-node Pulumi programs (e.g. Python, .NET, Go, etc.) to
	// run node policy packs, we must set the plugin's pwd to the policy pack directory instead of the Pulumi
	// program directory, so that the '@pulumi/pulumi/cmd/run-policy-pack' module from the policy pack's
	// node_modules is used.
	pwd := policyPackPath
	plug, err := newPlugin(ctx, pwd, pluginPath, fmt.Sprintf("%v (analyzer)", name),
		[]string{host.ServerAddr(), policyPackPath})
	if err != nil {
		if err == errRunPolicyModuleNotFound {
			return nil, fmt.Errorf("it looks like the policy pack's dependencies are not installed; "+
				"try running npm install or yarn install in %q", policyPackPath)
		}
		return nil, errors.Wrapf(err, "policy pack %q failed to start", string(name))
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
func (a *analyzer) Analyze(r AnalyzerResource) ([]AnalyzeDiagnostic, error) {
	urn, t, name, props := r.URN, r.Type, r.Name, r.Properties

	label := fmt.Sprintf("%s.Analyze(%s)", a.label(), t)
	logging.V(7).Infof("%s executing (#props=%d)", label, len(props))
	mprops, err := MarshalProperties(props, MarshalOptions{KeepUnknowns: true, KeepSecrets: true})
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Analyze(a.ctx.Request(), &pulumirpc.AnalyzeRequest{
		Urn:        string(urn),
		Type:       string(t),
		Name:       string(name),
		Properties: mprops,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return nil, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s success: failures=#%d", label, len(failures))

	diags, err := convertDiagnostics(failures)
	if err != nil {
		return nil, errors.Wrap(err, "converting analysis results")
	}
	return diags, nil
}

// AnalyzeStack analyzes all resources in a stack at the end of the update operation.
func (a *analyzer) AnalyzeStack(resources []AnalyzerResource) ([]AnalyzeDiagnostic, error) {
	logging.V(7).Infof("%s.AnalyzeStack(#resources=%d) executing", a.label(), len(resources))

	protoResources := make([]*pulumirpc.AnalyzerResource, len(resources))
	for idx, resource := range resources {
		props, err := MarshalProperties(resource.Properties, MarshalOptions{KeepUnknowns: true, KeepSecrets: true})
		if err != nil {
			return nil, errors.Wrap(err, "marshalling properties")
		}

		protoResources[idx] = &pulumirpc.AnalyzerResource{
			Urn:        string(resource.URN),
			Type:       string(resource.Type),
			Name:       string(resource.Name),
			Properties: props,
		}
	}

	resp, err := a.client.AnalyzeStack(a.ctx.Request(), &pulumirpc.AnalyzeStackRequest{
		Resources: protoResources,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		// Handle the case where we the policy pack doesn't implement a recent enough
		// AnalyzerService to support the AnalyzeStack method. Ignore the error as it
		// just means the analyzer isn't capable of this specific type of check.
		if rpcError.Code() == codes.Unimplemented {
			logging.V(7).Infof("%s.AnalyzeStack(...) is unimplemented, skipping: err=%v", a.label(), rpcError)
			return nil, nil
		}

		logging.V(7).Infof("%s.AnalyzeStack(...) failed: err=%v", a.label(), rpcError)
		return nil, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s.AnalyzeStack(...) success: failures=#%d", a.label(), len(failures))

	diags, err := convertDiagnostics(failures)
	if err != nil {
		return nil, errors.Wrap(err, "converting analysis results")
	}
	return diags, nil
}

// GetAnalyzerInfo returns metadata about the policies contained in this analyzer plugin.
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

func convertDiagnostics(protoDiagnostics []*pulumirpc.AnalyzeDiagnostic) ([]AnalyzeDiagnostic, error) {
	diagnostics := make([]AnalyzeDiagnostic, len(protoDiagnostics))
	for idx := range protoDiagnostics {
		protoD := protoDiagnostics[idx]

		enforcementLevel, err := convertEnforcementLevel(protoD.EnforcementLevel)
		if err != nil {
			return nil, err
		}

		diagnostics[idx] = AnalyzeDiagnostic{
			PolicyName:        protoD.PolicyName,
			PolicyPackName:    protoD.PolicyPackName,
			PolicyPackVersion: protoD.PolicyPackVersion,
			Description:       protoD.Description,
			Message:           protoD.Message,
			Tags:              protoD.Tags,
			EnforcementLevel:  enforcementLevel,
			URN:               resource.URN(protoD.Urn),
		}
	}

	return diagnostics, nil
}
