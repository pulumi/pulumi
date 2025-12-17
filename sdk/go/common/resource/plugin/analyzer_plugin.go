// Copyright 2016-2025, Pulumi Corporation.
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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/blang/semver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx     *Context
	name    tokens.QName
	plug    *Plugin
	client  pulumirpc.AnalyzerClient
	version string

	// The description from the policy pack's PulumiPolicy.yaml file (if present).
	description string

	// Cached result of the first call to GetAnalyzerInfo, which will be returned from subsequent calls.
	info *AnalyzerInfo
	// Cached map of policy name to severity for quick lookup by policy name.
	policyNameToSeverity map[string]apitype.PolicySeverity
}

var _ Analyzer = (*analyzer)(nil)

// NewAnalyzer binds to a given analyzer's plugin by name and creates a gRPC connection to it.  If the associated plugin
// could not be found by name on the PATH, or an error occurs while creating the child process, an error is returned.
func NewAnalyzer(host Host, ctx *Context, name tokens.QName) (Analyzer, error) {
	// Load the plugin's path by using the standard workspace logic.
	path, err := workspace.GetPluginPath(
		ctx.baseContext,
		ctx.Diag,
		workspace.PluginDescriptor{
			Name: strings.ReplaceAll(string(name), tokens.QNameDelimiter, "_"),
			Kind: apitype.AnalyzerPlugin,
		},
		host.GetProjectPlugins())
	if err != nil {
		return nil, rpcerror.Convert(err)
	}
	contract.Assertf(path != "", "unexpected empty path for analyzer plugin %s", name)

	dialOpts := rpcutil.OpenTracingInterceptorDialOptions()

	plug, _, err := newPlugin(ctx, ctx.Pwd, path, fmt.Sprintf("%v (analyzer)", name),
		apitype.AnalyzerPlugin, []string{host.ServerAddr(), ctx.Pwd}, nil, /*env*/
		testConnection, dialOpts, host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: string(name)}))
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

// NewPolicyAnalyzer boots the analyzer plugin located at `policyPackpath`. `hasPlugin` is a function that allows the
// caller to configure how it is determined if the language plugin is available. If nil it will default to looking for
// the plugin by path.
func NewPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, policyPackPath string, opts *PolicyAnalyzerOptions,
	hasPlugin func(workspace.PluginDescriptor) bool,
) (Analyzer, error) {
	projPath := filepath.Join(policyPackPath, "PulumiPolicy.yaml")
	proj, err := workspace.LoadPolicyPack(projPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Pulumi policy project located at %q: %w", policyPackPath, err)
	}

	handshake := func(
		ctx context.Context, bin string, prefix string, conn *grpc.ClientConn,
	) (*pulumirpc.AnalyzerHandshakeResponse, error) {
		// For analyzers the root directory and program directory are the location of the PulumiPolicy.yaml _not_ the
		// location of the shim plugin.
		dir := policyPackPath
		client := pulumirpc.NewAnalyzerClient(conn)

		req := pulumirpc.AnalyzerHandshakeRequest{
			EngineAddress:    host.ServerAddr(),
			RootDirectory:    &dir,
			ProgramDirectory: &dir,
		}

		res, err := client.Handshake(ctx, &req)
		if err != nil {
			status, ok := status.FromError(err)
			if ok && status.Code() == codes.Unimplemented {
				// If the provider doesn't implement Handshake, that's fine -- we'll fall back to existing behavior.
				logging.V(7).Infof("Handshake: not supported by '%v'", bin)
				return nil, nil
			}
			return nil, fmt.Errorf("failed to handshake with '%v': %w", bin, err)
		}

		logging.V(7).Infof("Handshake: success [%v]", bin)
		return res, nil
	}

	// This first section is a back compatibility bit for the old way of running analyzer plugins where we would look
	// for a plugin called "pulumi-analyzer-policy-<runtime>" and invoke that plugin with two arguments, the engine
	// address and the policy pack path. We still do this for python and nodejs, but not for other actual "languages"
	// (i.e. things with language plugins), but have to leave this in to ensure things like
	// https://github.com/pulumi/pulumi-policy-opa continue to work (although in time they could probably be moved to
	// just be language runtimes like the rest).

	var plug *Plugin
	var foundLanguagePlugin bool
	// Try to load the language plugin for the runtime, except for python and node that _for now_ continue using the
	// legacy behavior.
	if proj.Runtime.Name() != "python" && proj.Runtime.Name() != "nodejs" {
		if hasPlugin == nil {
			hasPlugin = func(spec workspace.PluginDescriptor) bool {
				path, err := workspace.GetPluginPath(
					ctx.baseContext,
					ctx.Diag,
					spec,
					host.GetProjectPlugins())
				return err == nil && path != ""
			}
		}

		foundLanguagePlugin = hasPlugin(workspace.PluginDescriptor{Name: proj.Runtime.Name(), Kind: apitype.LanguagePlugin})
	}
	if !foundLanguagePlugin {
		// Couldn't get a language plugin, fall back to the old behavior

		// For historical reasons, the Node.js plugin name is just "policy".
		// All other languages have the runtime appended, e.g. "policy-<runtime>".
		policyAnalyzerName := "policy"
		if !strings.EqualFold(proj.Runtime.Name(), "nodejs") {
			policyAnalyzerName = "policy-" + proj.Runtime.Name()
		}

		// Load the policy-booting analyzer plugin (i.e., `pulumi-analyzer-${policyAnalyzerName}`).
		var pluginPath string
		pluginPath, err = workspace.GetPluginPath(
			ctx.baseContext, ctx.Diag,
			workspace.PluginDescriptor{Name: policyAnalyzerName, Kind: apitype.AnalyzerPlugin}, host.GetProjectPlugins())

		var e *workspace.MissingError
		if errors.As(err, &e) {
			return nil, fmt.Errorf("could not start policy pack %q because the built-in analyzer "+
				"plugin that runs policy plugins is missing. This might occur when the plugin "+
				"directory is not on your $PATH, or when the installed version of the Pulumi SDK "+
				"does not support resource policies", string(name))
		} else if err != nil {
			return nil, err
		}

		// The `pulumi-analyzer-policy` plugin is a script that looks for the '@pulumi/pulumi/cmd/run-policy-pack'
		// node module and runs it with node. To allow non-node Pulumi programs (e.g. Python, .NET, Go, etc.) to
		// run node policy packs, we must set the plugin's pwd to the policy pack directory instead of the Pulumi
		// program directory, so that the '@pulumi/pulumi/cmd/run-policy-pack' module from the policy pack's
		// node_modules is used.
		pwd := policyPackPath

		args := []string{host.ServerAddr(), "."}
		for k, v := range proj.Runtime.Options() {
			if vstr := fmt.Sprintf("%v", v); vstr != "" {
				args = append(args, fmt.Sprintf("-%s=%s", k, vstr))
			}
		}

		// Create the environment variables from the options.
		var env []string
		env, err = constructEnv(opts, proj.Runtime.Name())
		if err != nil {
			return nil, err
		}

		plug, _, err = newPlugin(ctx, pwd, pluginPath, fmt.Sprintf("%v (analyzer)", name),
			apitype.AnalyzerPlugin, args, env, handshake,
			analyzerPluginDialOptions(ctx, fmt.Sprintf("%v", name)),
			host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: string(name)}))
	} else {
		// Else we _did_ get a lanuage plugin so just use RunPlugin to invoke the policy pack.

		plug, _, err = newPlugin(ctx, ctx.Pwd, policyPackPath, fmt.Sprintf("%v (analyzer)", name),
			apitype.AnalyzerPlugin, []string{host.ServerAddr()}, os.Environ(),
			handshake, analyzerPluginDialOptions(ctx, string(name)),
			host.AttachDebugger(DebugSpec{Type: DebugTypePlugin, Name: string(name)}))
	}

	if err != nil {
		// The original error might have been wrapped before being returned from newPlugin. So we look for
		// the root cause of the error. This won't work if we switch to Go 1.13's new approach to wrapping.

		if errors.Is(err, errRunPolicyModuleNotFound) {
			return nil, fmt.Errorf("it looks like the policy pack's dependencies are not installed; "+
				"try running npm install or yarn install in %q", policyPackPath)
		}
		if errors.Is(err, errPluginNotFound) {
			return nil, fmt.Errorf("policy pack not found at %q", name)
		}
		return nil, fmt.Errorf("policy pack %q failed to start: %w", string(name), err)
	}
	contract.Assertf(plug != nil, "unexpected nil analyzer plugin for %s", name)

	client := pulumirpc.NewAnalyzerClient(plug.Conn)

	// We call Configure on the analyzer plugin if we've been given options. We might not have options. For example
	// example when running `pulumi policy publish`, we are not running in the context of a project or stack.
	if opts != nil {
		req := &pulumirpc.AnalyzerStackConfigureRequest{
			Stack:        opts.Stack,
			Project:      opts.Project,
			Organization: opts.Organization,
			Tags:         opts.Tags,
			DryRun:       opts.DryRun,
		}
		mconfig := make(map[string]string, len(opts.Config))
		for k, v := range opts.Config {
			mconfig[k.String()] = v
		}
		req.Config = mconfig
		mkeys := make([]string, 0, len(opts.ConfigSecretKeys))
		for _, k := range opts.ConfigSecretKeys {
			mkeys = append(mkeys, k.String())
		}
		req.ConfigSecretKeys = mkeys

		_, err = client.ConfigureStack(ctx.Request(), req)
		if err != nil {
			status, ok := status.FromError(err)
			if ok && status.Code() == codes.Unimplemented {
				// If the analyzer doesn't implement StackConfigure, that's fine -- we'll fall back to existing
				// behavior.
				logging.V(7).Infof("StackConfigure: not supported by '%v'", name)
			} else {
				logging.V(7).Infof("StackConfigure: failed: err=%v", status)
				return nil, rpcerror.Convert(err)
			}
		}

		logging.V(7).Infof("StackConfigure: success [%v]", name)
	}

	var description string
	if proj.Description != nil {
		description = *proj.Description
	}

	return &analyzer{
		ctx:         ctx,
		name:        name,
		plug:        plug,
		client:      client,
		version:     proj.Version,
		description: description,
	}, nil
}

func NewAnalyzerWithClient(ctx *Context, name tokens.QName, client pulumirpc.AnalyzerClient) Analyzer {
	return &analyzer{
		ctx:    ctx,
		name:   name,
		client: client,
	}
}

func (a *analyzer) Name() tokens.QName { return a.name }

// label returns a base label for tracing functions.
func (a *analyzer) label() string {
	return fmt.Sprintf("Analyzer[%s]", a.name)
}

func (a *analyzer) requestContext() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx.Request()
}

// Analyze analyzes a single resource object, and returns any errors that it finds.
func (a *analyzer) Analyze(r AnalyzerResource) (AnalyzeResponse, error) {
	urn, t, name, props := r.URN, r.Type, r.Name, r.Properties

	label := fmt.Sprintf("%s.Analyze(%s)", a.label(), t)
	logging.V(7).Infof("%s executing (#props=%d)", label, len(props))
	mprops, err := MarshalProperties(props,
		MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
	if err != nil {
		return AnalyzeResponse{}, err
	}

	provider, err := marshalProvider(r.Provider)
	if err != nil {
		return AnalyzeResponse{}, err
	}

	resp, err := a.client.Analyze(a.requestContext(), &pulumirpc.AnalyzeRequest{
		Urn:        string(urn),
		Type:       string(t),
		Name:       name,
		Properties: mprops,
		Options:    marshalResourceOptions(r.Options),
		Provider:   provider,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return AnalyzeResponse{}, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s success: failures=#%d", label, len(failures))

	diags, err := a.convertDiagnostics(failures)
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("converting analysis results: %w", err)
	}
	return AnalyzeResponse{
		Diagnostics:   diags,
		NotApplicable: convertNotApplicable(resp.GetNotApplicable()),
	}, nil
}

// AnalyzeStack analyzes all resources in a stack at the end of the update operation.
func (a *analyzer) AnalyzeStack(resources []AnalyzerStackResource) (AnalyzeResponse, error) {
	logging.V(7).Infof("%s.AnalyzeStack(#resources=%d) executing", a.label(), len(resources))

	protoResources := make([]*pulumirpc.AnalyzerResource, len(resources))
	for idx, resource := range resources {
		props, err := MarshalProperties(resource.Properties,
			MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
		if err != nil {
			return AnalyzeResponse{}, fmt.Errorf("marshalling properties: %w", err)
		}

		provider, err := marshalProvider(resource.Provider)
		if err != nil {
			return AnalyzeResponse{}, err
		}

		propertyDeps := make(map[string]*pulumirpc.AnalyzerPropertyDependencies)
		for pk, pd := range resource.PropertyDependencies {
			// Skip properties that have no dependencies.
			if len(pd) == 0 {
				continue
			}

			pdeps := slice.Prealloc[string](1)
			for _, d := range pd {
				pdeps = append(pdeps, string(d))
			}
			propertyDeps[string(pk)] = &pulumirpc.AnalyzerPropertyDependencies{
				Urns: pdeps,
			}
		}

		protoResources[idx] = &pulumirpc.AnalyzerResource{
			Urn:                  string(resource.URN),
			Type:                 string(resource.Type),
			Name:                 resource.Name,
			Properties:           props,
			Options:              marshalResourceOptions(resource.Options),
			Provider:             provider,
			Parent:               string(resource.Parent),
			Dependencies:         convertURNs(resource.Dependencies),
			PropertyDependencies: propertyDeps,
		}
	}

	resp, err := a.client.AnalyzeStack(a.requestContext(), &pulumirpc.AnalyzeStackRequest{
		Resources: protoResources,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		// Handle the case where we the policy pack doesn't implement a recent enough
		// AnalyzerService to support the AnalyzeStack method. Ignore the error as it
		// just means the analyzer isn't capable of this specific type of check.
		if rpcError.Code() == codes.Unimplemented {
			logging.V(7).Infof("%s.AnalyzeStack(...) is unimplemented, skipping: err=%v", a.label(), rpcError)
			return AnalyzeResponse{}, nil
		}

		logging.V(7).Infof("%s.AnalyzeStack(...) failed: err=%v", a.label(), rpcError)
		return AnalyzeResponse{}, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s.AnalyzeStack(...) success: failures=#%d", a.label(), len(failures))

	diags, err := a.convertDiagnostics(failures)
	if err != nil {
		return AnalyzeResponse{}, fmt.Errorf("converting analysis results: %w", err)
	}
	return AnalyzeResponse{
		Diagnostics:   diags,
		NotApplicable: convertNotApplicable(resp.GetNotApplicable()),
	}, nil
}

// Remediate is given the opportunity to transform a single resource, and returns its new properties.
func (a *analyzer) Remediate(r AnalyzerResource) (RemediateResponse, error) {
	urn, t, name, props := r.URN, r.Type, r.Name, r.Properties

	label := fmt.Sprintf("%s.Remediate(%s)", a.label(), t)
	logging.V(7).Infof("%s executing (#props=%d)", label, len(props))
	mprops, err := MarshalProperties(props,
		MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: false})
	if err != nil {
		return RemediateResponse{}, err
	}

	provider, err := marshalProvider(r.Provider)
	if err != nil {
		return RemediateResponse{}, err
	}

	resp, err := a.client.Remediate(a.requestContext(), &pulumirpc.AnalyzeRequest{
		Urn:        string(urn),
		Type:       string(t),
		Name:       name,
		Properties: mprops,
		Options:    marshalResourceOptions(r.Options),
		Provider:   provider,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)

		// Handle the case where we the policy pack doesn't implement a recent enough to implement Transform.
		if rpcError.Code() == codes.Unimplemented {
			logging.V(7).Infof("%s.Transform(...) is unimplemented, skipping: err=%v", a.label(), rpcError)
			return RemediateResponse{}, nil
		}

		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return RemediateResponse{}, rpcError
	}

	remediations := resp.GetRemediations()
	results := make([]Remediation, len(remediations))
	for i, r := range remediations {
		tprops, err := UnmarshalProperties(r.GetProperties(),
			MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: false})
		if err != nil {
			return RemediateResponse{}, err
		}

		// The version from PulumiPolicy.yaml is used, if set, over the version from the diagnostic.
		policyPackVersion := r.GetPolicyPackVersion()
		if a.version != "" {
			policyPackVersion = a.version
		}

		results[i] = Remediation{
			PolicyName:        r.GetPolicyName(),
			Description:       r.GetDescription(),
			PolicyPackName:    r.GetPolicyPackName(),
			PolicyPackVersion: policyPackVersion,
			Properties:        tprops,
			Diagnostic:        r.GetDiagnostic(),
		}
	}

	logging.V(7).Infof("%s success: #remediations=%d", label, len(results))
	return RemediateResponse{
		Remediations:  results,
		NotApplicable: convertNotApplicable(resp.GetNotApplicable()),
	}, nil
}

// GetAnalyzerInfo returns metadata about the policies contained in this analyzer plugin.
func (a *analyzer) GetAnalyzerInfo() (AnalyzerInfo, error) {
	// Return the cached result, if available.
	if a.info != nil {
		return *a.info, nil
	}

	label := a.label() + ".GetAnalyzerInfo()"
	logging.V(7).Infof("%s executing", label)
	resp, err := a.client.GetAnalyzerInfo(a.requestContext(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", a.label(), rpcError)
		return AnalyzerInfo{}, rpcError
	}

	rpcPolicies := resp.GetPolicies()
	policies := make([]AnalyzerPolicyInfo, len(rpcPolicies))
	for i, p := range rpcPolicies {
		enforcementLevel, err := convertEnforcementLevel(p.EnforcementLevel)
		if err != nil {
			return AnalyzerInfo{}, err
		}

		var schema *AnalyzerPolicyConfigSchema
		if resp.GetSupportsConfig() {
			schema = convertConfigSchema(p.GetConfigSchema())

			// Inject `enforcementLevel` into the schema.
			if schema == nil {
				schema = &AnalyzerPolicyConfigSchema{}
			}
			if schema.Properties == nil {
				schema.Properties = map[string]JSONSchema{}
			}
			schema.Properties["enforcementLevel"] = JSONSchema{
				"type": "string",
				"enum": []string{"advisory", "mandatory", "remediate", "disabled"},
			}
		}

		policies[i] = AnalyzerPolicyInfo{
			Name:             p.GetName(),
			DisplayName:      p.GetDisplayName(),
			Description:      p.GetDescription(),
			EnforcementLevel: enforcementLevel,
			Message:          p.GetMessage(),
			ConfigSchema:     schema,
			Type:             convertPolicyType(p.PolicyType),
			Severity:         convertSeverity(p.GetSeverity()),
			Framework:        convertComplianceFramework(p.GetFramework()),
			Tags:             p.GetTags(),
			RemediationSteps: p.GetRemediationSteps(),
			URL:              p.GetUrl(),
		}
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})

	initialConfig, err := convertPolicyConfig(resp.GetInitialConfig())
	if err != nil {
		return AnalyzerInfo{}, err
	}

	// The version from PulumiPolicy.yaml is used, if set, over the version from the response.
	version := resp.GetVersion()
	if a.version != "" {
		version = a.version
		logging.V(7).Infof("Using version %q from PulumiPolicy.yaml", version)
	}

	// The description from the gRPC call is preferred, but if it's not set, fall back to the
	// description from PulumiPolicy.yaml.
	description := resp.GetDescription()
	if description == "" {
		description = a.description
	}

	// Cache the result for subsequent calls.
	info := AnalyzerInfo{
		Name:           resp.GetName(),
		DisplayName:    resp.GetDisplayName(),
		Version:        version,
		SupportsConfig: resp.GetSupportsConfig(),
		Policies:       policies,
		InitialConfig:  initialConfig,
		Description:    description,
		Readme:         resp.GetReadme(),
		Provider:       resp.GetProvider(),
		Tags:           resp.GetTags(),
		Repository:     resp.GetRepository(),
	}
	a.info = &info
	return info, nil
}

// GetPluginInfo returns this plugin's information.
func (a *analyzer) GetPluginInfo() (PluginInfo, error) {
	label := a.label() + ".GetPluginInfo()"
	logging.V(7).Infof("%s executing", label)
	resp, err := a.client.GetPluginInfo(a.requestContext(), &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", a.label(), rpcError)
		return PluginInfo{}, rpcError
	}

	var version *semver.Version
	if v := resp.Version; v != "" {
		sv, err := semver.ParseTolerant(v)
		if err != nil {
			return PluginInfo{}, err
		}
		version = &sv
	}

	return PluginInfo{
		Version: version,
	}, nil
}

func (a *analyzer) Configure(policyConfig map[string]AnalyzerPolicyConfig) error {
	label := a.label() + ".Configure(...)"
	logging.V(7).Infof("%s executing", label)

	if len(policyConfig) == 0 {
		logging.V(7).Infof("%s returning early, no config specified", label)
		return nil
	}

	c := make(map[string]*pulumirpc.PolicyConfig)

	for k, v := range policyConfig {
		if !v.EnforcementLevel.IsValid() {
			return fmt.Errorf("invalid enforcement level %q", v.EnforcementLevel)
		}

		props, err := structpb.NewStruct(v.Properties)
		if err != nil {
			return fmt.Errorf("marshalling properties: %w", err)
		}

		c[k] = &pulumirpc.PolicyConfig{
			EnforcementLevel: marshalEnforcementLevel(v.EnforcementLevel),
			Properties:       props,
		}
	}

	_, err := a.client.Configure(a.requestContext(), &pulumirpc.ConfigureAnalyzerRequest{
		PolicyConfig: c,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return rpcError
	}

	// Update the cached analyzer info with updated enforcement level from config.
	if a.info != nil {
		for i, p := range a.info.Policies {
			if newConfig, ok := policyConfig[p.Name]; ok {
				a.info.Policies[i].EnforcementLevel = newConfig.EnforcementLevel
			}
		}
	}

	return nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	if a.plug == nil {
		return nil
	}
	return a.plug.Close()
}

// Cancel signals the analyzer to gracefully shut down and abort any ongoing analysis operations.
func (a *analyzer) Cancel(ctx context.Context) error {
	label := a.label() + ".Cancel()"
	logging.V(7).Infof("%s executing", label)

	_, err := a.client.Cancel(ctx, &emptypb.Empty{})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(8).Infof("%s failed: err=%v", label, rpcError)
		if rpcError.Code() == codes.Unimplemented {
			return nil
		}
	}

	return err
}

// getPolicySeverity returns the severity for the given policy name, or PolicySeverityUnspecified if the policy name
// is not found.
func (a *analyzer) getPolicySeverity(policyName string) apitype.PolicySeverity {
	m := a.policyNameToSeverity
	if m == nil {
		// Get the info to populate the map. This will return a cached value if we've already called it.
		info, err := a.GetAnalyzerInfo()
		if err != nil {
			logging.V(7).Infof("%s.getPolicySeverity(%q): failed to get analyzer info: %v", a.label(), policyName, err)
			return apitype.PolicySeverityUnspecified
		}

		m = make(map[string]apitype.PolicySeverity, len(info.Policies))
		for _, p := range info.Policies {
			if p.Severity != apitype.PolicySeverityUnspecified {
				m[p.Name] = p.Severity
			}
		}
		a.policyNameToSeverity = m
	}
	// This will return PolicySeverityUnspecified (empty string value) if the policy name is not found.
	return m[policyName]
}

func analyzerPluginDialOptions(ctx *Context, name string) []grpc.DialOption {
	dialOpts := append(
		rpcutil.OpenTracingInterceptorDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions(),
	)

	if ctx.DialOptions != nil {
		metadata := map[string]any{
			"mode": "client",
			"kind": "analyzer",
		}
		if name != "" {
			metadata["name"] = name
		}
		dialOpts = append(dialOpts, ctx.DialOptions(metadata)...)
	}

	return dialOpts
}

func marshalResourceOptions(opts AnalyzerResourceOptions) *pulumirpc.AnalyzerResourceOptions {
	secs := make([]string, len(opts.AdditionalSecretOutputs))
	for idx := range opts.AdditionalSecretOutputs {
		secs[idx] = string(opts.AdditionalSecretOutputs[idx])
	}

	var deleteBeforeReplace bool
	if opts.DeleteBeforeReplace != nil {
		deleteBeforeReplace = *opts.DeleteBeforeReplace
	}

	result := &pulumirpc.AnalyzerResourceOptions{
		Protect:                    opts.Protect,
		IgnoreChanges:              opts.IgnoreChanges,
		DeleteBeforeReplace:        deleteBeforeReplace,
		DeleteBeforeReplaceDefined: opts.DeleteBeforeReplace != nil,
		AdditionalSecretOutputs:    secs,
		Aliases:                    convertAliases(opts.Aliases, opts.AliasURNs),
		CustomTimeouts: &pulumirpc.AnalyzerResourceOptions_CustomTimeouts{
			Create: opts.CustomTimeouts.Create,
			Update: opts.CustomTimeouts.Update,
			Delete: opts.CustomTimeouts.Delete,
		},
		Parent: string(opts.Parent),
	}
	return result
}

func marshalProvider(provider *AnalyzerProviderResource) (*pulumirpc.AnalyzerProviderResource, error) {
	if provider == nil {
		return nil, nil
	}

	props, err := MarshalProperties(provider.Properties,
		MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
	if err != nil {
		return nil, fmt.Errorf("marshalling properties: %w", err)
	}

	return &pulumirpc.AnalyzerProviderResource{
		Urn:        string(provider.URN),
		Type:       string(provider.Type),
		Name:       provider.Name,
		Properties: props,
	}, nil
}

func marshalEnforcementLevel(el apitype.EnforcementLevel) pulumirpc.EnforcementLevel {
	switch el {
	case apitype.Advisory:
		return pulumirpc.EnforcementLevel_ADVISORY
	case apitype.Mandatory:
		return pulumirpc.EnforcementLevel_MANDATORY
	case apitype.Remediate:
		return pulumirpc.EnforcementLevel_REMEDIATE
	case apitype.Disabled:
		return pulumirpc.EnforcementLevel_DISABLED
	}
	contract.Failf("Unrecognized enforcement level %s", el)
	return 0
}

func convertURNs(urns []resource.URN) []string {
	result := make([]string, len(urns))
	for idx := range urns {
		result[idx] = string(urns[idx])
	}
	return result
}

func convertAlias(alias resource.Alias) string {
	return string(alias.GetURN())
}

func convertAliases(aliases []resource.Alias, aliasURNs []resource.URN) []string {
	result := make([]string, len(aliases)+len(aliasURNs))
	for idx, alias := range aliases {
		result[idx] = convertAlias(alias)
	}
	for idx, aliasURN := range aliasURNs {
		result[idx+len(aliases)] = convertAlias(resource.Alias{URN: aliasURN})
	}
	return result
}

func convertEnforcementLevel(el pulumirpc.EnforcementLevel) (apitype.EnforcementLevel, error) {
	switch el {
	case pulumirpc.EnforcementLevel_ADVISORY:
		return apitype.Advisory, nil
	case pulumirpc.EnforcementLevel_MANDATORY:
		return apitype.Mandatory, nil
	case pulumirpc.EnforcementLevel_REMEDIATE:
		return apitype.Remediate, nil
	case pulumirpc.EnforcementLevel_DISABLED:
		return apitype.Disabled, nil

	default:
		return "", fmt.Errorf("invalid enforcement level %d", el)
	}
}

func convertPolicyType(t pulumirpc.PolicyType) AnalyzerPolicyType {
	switch t {
	case pulumirpc.PolicyType_POLICY_TYPE_UNKNOWN:
		return AnalyzerPolicyTypeUnknown
	case pulumirpc.PolicyType_POLICY_TYPE_RESOURCE:
		return AnalyzerPolicyTypeResource
	case pulumirpc.PolicyType_POLICY_TYPE_STACK:
		return AnalyzerPolicyTypeStack
	}
	return AnalyzerPolicyTypeUnknown
}

func convertSeverity(s pulumirpc.PolicySeverity) apitype.PolicySeverity {
	switch s {
	case pulumirpc.PolicySeverity_POLICY_SEVERITY_LOW:
		return apitype.PolicySeverityLow
	case pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM:
		return apitype.PolicySeverityMedium
	case pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH:
		return apitype.PolicySeverityHigh
	case pulumirpc.PolicySeverity_POLICY_SEVERITY_CRITICAL:
		return apitype.PolicySeverityCritical
	case pulumirpc.PolicySeverity_POLICY_SEVERITY_UNSPECIFIED:
		fallthrough
	default:
		return apitype.PolicySeverityUnspecified
	}
}

func convertConfigSchema(schema *pulumirpc.PolicyConfigSchema) *AnalyzerPolicyConfigSchema {
	if schema == nil {
		return nil
	}

	props := make(map[string]JSONSchema)
	for k, v := range schema.GetProperties().AsMap() {
		s := v.(map[string]any)
		props[k] = JSONSchema(s)
	}

	return &AnalyzerPolicyConfigSchema{
		Properties: props,
		Required:   schema.GetRequired(),
	}
}

func convertComplianceFramework(framework *pulumirpc.PolicyComplianceFramework) *AnalyzerPolicyComplianceFramework {
	if framework == nil {
		return nil
	}

	return &AnalyzerPolicyComplianceFramework{
		Name:          framework.GetName(),
		Version:       framework.GetVersion(),
		Reference:     framework.GetReference(),
		Specification: framework.GetSpecification(),
	}
}

func (a *analyzer) convertDiagnostics(protoDiagnostics []*pulumirpc.AnalyzeDiagnostic) ([]AnalyzeDiagnostic, error) {
	diagnostics := make([]AnalyzeDiagnostic, len(protoDiagnostics))
	for idx := range protoDiagnostics {
		protoD := protoDiagnostics[idx]

		// The version from PulumiPolicy.yaml is used, if set, over the version from the diagnostic.
		policyPackVersion := protoD.PolicyPackVersion
		if a.version != "" {
			policyPackVersion = a.version
		}

		enforcementLevel, err := convertEnforcementLevel(protoD.EnforcementLevel)
		if err != nil {
			return nil, err
		}

		severity := convertSeverity(protoD.Severity)
		if severity == apitype.PolicySeverityUnspecified {
			// If the severity is not specified in the diagnostic, try to get it from the policy info.
			severity = a.getPolicySeverity(protoD.PolicyName)
		}

		diagnostics[idx] = AnalyzeDiagnostic{
			PolicyName:        protoD.PolicyName,
			PolicyPackName:    protoD.PolicyPackName,
			PolicyPackVersion: policyPackVersion,
			Description:       protoD.Description,
			Message:           protoD.Message,
			EnforcementLevel:  enforcementLevel,
			URN:               resource.URN(protoD.Urn),
			Severity:          severity,
		}
	}

	return diagnostics, nil
}

func convertPolicyConfig(config map[string]*pulumirpc.PolicyConfig) (map[string]AnalyzerPolicyConfig, error) {
	result := make(map[string]AnalyzerPolicyConfig)
	for k, v := range config {
		enforcementLevel, err := convertEnforcementLevel(v.GetEnforcementLevel())
		if err != nil {
			return nil, err
		}
		result[k] = AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       v.GetProperties().AsMap(),
		}
	}
	return result, nil
}

func convertNotApplicable(protoNotApplicable []*pulumirpc.PolicyNotApplicable) []PolicyNotApplicable {
	return slice.Map(protoNotApplicable, func(p *pulumirpc.PolicyNotApplicable) PolicyNotApplicable {
		return PolicyNotApplicable{
			PolicyName: p.PolicyName,
			Reason:     p.Reason,
		}
	})
}

// constructEnv creates a slice of key/value pairs to be used as the environment for the policy pack process. Each entry
// is of the form "key=value". Config is passed as an environment variable (including unecrypted secrets), similar to
// how config is passed to each language runtime plugin.
func constructEnv(opts *PolicyAnalyzerOptions, runtime string) ([]string, error) {
	env := os.Environ()

	maybeAppendEnv := func(k, v string) {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}

	config, err := constructConfig(opts)
	if err != nil {
		return nil, err
	}
	maybeAppendEnv("PULUMI_CONFIG", config)

	if opts != nil {
		// Set both PULUMI_NODEJS_* and PULUMI_* environment variables for Node.js. The Node.js
		// SDK currently looks for the PULUMI_NODEJS_* variants only, but we'd like to move to
		// using the more general PULUMI_* variants for all languages to avoid special casing
		// like this, and setting the PULUMI_* variants for Node.js is the first step.
		if runtime == "nodejs" {
			maybeAppendEnv("PULUMI_NODEJS_ORGANIZATION", opts.Organization)
			maybeAppendEnv("PULUMI_NODEJS_PROJECT", opts.Project)
			maybeAppendEnv("PULUMI_NODEJS_STACK", opts.Stack)
			maybeAppendEnv("PULUMI_NODEJS_DRY_RUN", strconv.FormatBool(opts.DryRun))
		}

		maybeAppendEnv("PULUMI_ORGANIZATION", opts.Organization)
		maybeAppendEnv("PULUMI_PROJECT", opts.Project)
		maybeAppendEnv("PULUMI_STACK", opts.Stack)
		maybeAppendEnv("PULUMI_DRY_RUN", strconv.FormatBool(opts.DryRun))
	}

	return env, nil
}

// constructConfig JSON-serializes the configuration data.
func constructConfig(opts *PolicyAnalyzerOptions) (string, error) {
	if opts == nil || opts.Config == nil {
		return "", nil
	}

	config := make(map[string]string)
	for k, v := range opts.Config {
		config[k.String()] = v
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(configJSON), nil
}
