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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/blang/semver"
	pbempty "github.com/golang/protobuf/ptypes/empty"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil/rpcerror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// analyzer reflects an analyzer plugin, loaded dynamically for a single suite of checks.
type analyzer struct {
	ctx     *Context
	name    tokens.QName
	plug    *plugin
	client  pulumirpc.AnalyzerClient
	version string
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
		[]string{host.ServerAddr(), ctx.Pwd}, nil /*env*/)
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

// NewPolicyAnalyzer boots the nodejs analyzer plugin located at `policyPackpath`
func NewPolicyAnalyzer(
	host Host, ctx *Context, name tokens.QName, policyPackPath string, opts *PolicyAnalyzerOptions) (Analyzer, error) {

	projPath := filepath.Join(policyPackPath, "PulumiPolicy.yaml")
	proj, err := workspace.LoadPolicyPack(projPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load Pulumi policy project located at %q", policyPackPath)
	}

	// For historical reasons, the Node.js plugin name is just "policy".
	// All other languages have the runtime appended, e.g. "policy-<runtime>".
	policyAnalyzerName := "policy"
	if !strings.EqualFold(proj.Runtime.Name(), "nodejs") {
		policyAnalyzerName = fmt.Sprintf("policy-%s", proj.Runtime.Name())
	}

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

	// Create the environment variables from the options.
	env, err := constructEnv(opts, proj.Runtime.Name())
	if err != nil {
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

	plug, err := newPlugin(ctx, pwd, pluginPath, fmt.Sprintf("%v (analyzer)", name), args, env)
	if err != nil {
		// The original error might have been wrapped before being returned from newPlugin. So we look for
		// the root cause of the error. This won't work if we switch to Go 1.13's new approach to wrapping.
		if errors.Cause(err) == errRunPolicyModuleNotFound {
			return nil, fmt.Errorf("it looks like the policy pack's dependencies are not installed; "+
				"try running npm install or yarn install in %q", policyPackPath)
		}
		if errors.Cause(err) == errPluginNotFound {
			return nil, fmt.Errorf("policy pack not found at %q", name)
		}
		return nil, errors.Wrapf(err, "policy pack %q failed to start", string(name))
	}
	contract.Assertf(plug != nil, "unexpected nil analyzer plugin for %s", name)

	return &analyzer{
		ctx:     ctx,
		name:    name,
		plug:    plug,
		client:  pulumirpc.NewAnalyzerClient(plug.Conn),
		version: proj.Version,
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
	mprops, err := MarshalProperties(props,
		MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
	if err != nil {
		return nil, err
	}

	provider, err := marshalProvider(r.Provider)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Analyze(a.ctx.Request(), &pulumirpc.AnalyzeRequest{
		Urn:        string(urn),
		Type:       string(t),
		Name:       string(name),
		Properties: mprops,
		Options:    marshalResourceOptions(r.Options),
		Provider:   provider,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return nil, rpcError
	}

	failures := resp.GetDiagnostics()
	logging.V(7).Infof("%s success: failures=#%d", label, len(failures))

	diags, err := convertDiagnostics(failures, a.version)
	if err != nil {
		return nil, errors.Wrap(err, "converting analysis results")
	}
	return diags, nil
}

// AnalyzeStack analyzes all resources in a stack at the end of the update operation.
func (a *analyzer) AnalyzeStack(resources []AnalyzerStackResource) ([]AnalyzeDiagnostic, error) {
	logging.V(7).Infof("%s.AnalyzeStack(#resources=%d) executing", a.label(), len(resources))

	protoResources := make([]*pulumirpc.AnalyzerResource, len(resources))
	for idx, resource := range resources {
		props, err := MarshalProperties(resource.Properties,
			MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
		if err != nil {
			return nil, errors.Wrap(err, "marshalling properties")
		}

		provider, err := marshalProvider(resource.Provider)
		if err != nil {
			return nil, err
		}

		propertyDeps := make(map[string]*pulumirpc.AnalyzerPropertyDependencies)
		for pk, pd := range resource.PropertyDependencies {
			// Skip properties that have no dependencies.
			if len(pd) == 0 {
				continue
			}

			pdeps := []string{}
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
			Name:                 string(resource.Name),
			Properties:           props,
			Options:              marshalResourceOptions(resource.Options),
			Provider:             provider,
			Parent:               string(resource.Parent),
			Dependencies:         convertURNs(resource.Dependencies),
			PropertyDependencies: propertyDeps,
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

	diags, err := convertDiagnostics(failures, a.version)
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
				"enum": []string{"advisory", "mandatory", "disabled"},
			}
		}

		policies[i] = AnalyzerPolicyInfo{
			Name:             p.GetName(),
			DisplayName:      p.GetDisplayName(),
			Description:      p.GetDescription(),
			EnforcementLevel: enforcementLevel,
			Message:          p.GetMessage(),
			ConfigSchema:     schema,
		}
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})

	initialConfig := make(map[string]AnalyzerPolicyConfig)
	for k, v := range resp.GetInitialConfig() {
		enforcementLevel, err := convertEnforcementLevel(v.GetEnforcementLevel())
		if err != nil {
			return AnalyzerInfo{}, err
		}
		initialConfig[k] = AnalyzerPolicyConfig{
			EnforcementLevel: enforcementLevel,
			Properties:       unmarshalMap(v.GetProperties()),
		}
	}

	// The version from PulumiPolicy.yaml is used, if set, over the version from the response.
	version := resp.GetVersion()
	if a.version != "" {
		version = a.version
		logging.V(7).Infof("Using version %q from PulumiPolicy.yaml", version)
	}

	return AnalyzerInfo{
		Name:           resp.GetName(),
		DisplayName:    resp.GetDisplayName(),
		Version:        version,
		SupportsConfig: resp.GetSupportsConfig(),
		Policies:       policies,
		InitialConfig:  initialConfig,
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

func (a *analyzer) Configure(policyConfig map[string]AnalyzerPolicyConfig) error {
	label := fmt.Sprintf("%s.Configure(...)", a.label())
	logging.V(7).Infof("%s executing", label)

	if len(policyConfig) == 0 {
		logging.V(7).Infof("%s returning early, no config specified", label)
		return nil
	}

	c := make(map[string]*pulumirpc.PolicyConfig)

	for k, v := range policyConfig {
		if !v.EnforcementLevel.IsValid() {
			return errors.Errorf("invalid enforcement level %q", v.EnforcementLevel)
		}
		c[k] = &pulumirpc.PolicyConfig{
			EnforcementLevel: marshalEnforcementLevel(v.EnforcementLevel),
			Properties:       marshalMap(v.Properties),
		}
	}

	_, err := a.client.Configure(a.ctx.Request(), &pulumirpc.ConfigureAnalyzerRequest{
		PolicyConfig: c,
	})
	if err != nil {
		rpcError := rpcerror.Convert(err)
		logging.V(7).Infof("%s failed: err=%v", label, rpcError)
		return rpcError
	}
	return nil
}

// Close tears down the underlying plugin RPC connection and process.
func (a *analyzer) Close() error {
	return a.plug.Close()
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
		Aliases:                    convertURNs(opts.Aliases),
		CustomTimeouts: &pulumirpc.AnalyzerResourceOptions_CustomTimeouts{
			Create: opts.CustomTimeouts.Create,
			Update: opts.CustomTimeouts.Update,
			Delete: opts.CustomTimeouts.Delete,
		},
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
		return nil, errors.Wrap(err, "marshalling properties")
	}

	return &pulumirpc.AnalyzerProviderResource{
		Urn:        string(provider.URN),
		Type:       string(provider.Type),
		Name:       string(provider.Name),
		Properties: props,
	}, nil
}

func marshalEnforcementLevel(el apitype.EnforcementLevel) pulumirpc.EnforcementLevel {
	switch el {
	case apitype.Advisory:
		return pulumirpc.EnforcementLevel_ADVISORY
	case apitype.Mandatory:
		return pulumirpc.EnforcementLevel_MANDATORY
	case apitype.Disabled:
		return pulumirpc.EnforcementLevel_DISABLED
	}
	contract.Failf("Unrecognized enforcement level %s", el)
	return 0
}

func marshalMap(m map[string]interface{}) *structpb.Struct {
	fields := make(map[string]*structpb.Value)
	for k, v := range m {
		val := marshalMapValue(v)
		if val != nil {
			fields[k] = val
		}
	}
	return &structpb.Struct{
		Fields: fields,
	}
}

func marshalMapValue(v interface{}) *structpb.Value {
	if v == nil {
		return &structpb.Value{
			Kind: &structpb.Value_NullValue{
				NullValue: structpb.NullValue_NULL_VALUE,
			},
		}
	}

	switch val := v.(type) {
	case bool:
		return &structpb.Value{
			Kind: &structpb.Value_BoolValue{
				BoolValue: val,
			},
		}
	case float64:
		return &structpb.Value{
			Kind: &structpb.Value_NumberValue{
				NumberValue: val,
			},
		}
	case string:
		return &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: val,
			},
		}
	case []interface{}:
		arr := make([]*structpb.Value, len(val))
		for i, e := range val {
			arr[i] = marshalMapValue(e)
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{Values: arr},
			},
		}
	case map[string]interface{}:
		return &structpb.Value{
			Kind: &structpb.Value_StructValue{
				StructValue: marshalMap(val),
			},
		}
	}

	contract.Failf("Unrecognized value: %v (type=%v)", v, reflect.TypeOf(v))
	return nil
}

func unmarshalMap(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return nil
	}
	result := make(map[string]interface{})
	for k, v := range s.Fields {
		result[k] = unmarshalMapValue(v)
	}
	return result
}

func unmarshalMapValue(v *structpb.Value) interface{} {
	switch val := v.Kind.(type) {
	case *structpb.Value_NullValue:
		return nil
	case *structpb.Value_BoolValue:
		return val.BoolValue
	case *structpb.Value_NumberValue:
		return val.NumberValue
	case *structpb.Value_StringValue:
		return val.StringValue
	case *structpb.Value_ListValue:
		arr := make([]interface{}, len(val.ListValue.Values))
		for i, e := range val.ListValue.Values {
			arr[i] = unmarshalMapValue(e)
		}
		return arr
	case *structpb.Value_StructValue:
		return unmarshalMap(val.StructValue)
	}

	contract.Failf("Unrecognized kind: %v (type=%v)", v.Kind, reflect.TypeOf(v.Kind))
	return nil
}

func convertURNs(urns []resource.URN) []string {
	result := make([]string, len(urns))
	for idx := range urns {
		result[idx] = string(urns[idx])
	}
	return result
}

func convertEnforcementLevel(el pulumirpc.EnforcementLevel) (apitype.EnforcementLevel, error) {
	switch el {
	case pulumirpc.EnforcementLevel_ADVISORY:
		return apitype.Advisory, nil
	case pulumirpc.EnforcementLevel_MANDATORY:
		return apitype.Mandatory, nil
	case pulumirpc.EnforcementLevel_DISABLED:
		return apitype.Disabled, nil

	default:
		return "", fmt.Errorf("Invalid enforcement level %d", el)
	}
}

func convertConfigSchema(schema *pulumirpc.PolicyConfigSchema) *AnalyzerPolicyConfigSchema {
	if schema == nil {
		return nil
	}

	props := make(map[string]JSONSchema)
	for k, v := range unmarshalMap(schema.GetProperties()) {
		s := v.(map[string]interface{})
		props[k] = JSONSchema(s)
	}

	return &AnalyzerPolicyConfigSchema{
		Properties: props,
		Required:   schema.GetRequired(),
	}
}

func convertDiagnostics(protoDiagnostics []*pulumirpc.AnalyzeDiagnostic, version string) ([]AnalyzeDiagnostic, error) {
	diagnostics := make([]AnalyzeDiagnostic, len(protoDiagnostics))
	for idx := range protoDiagnostics {
		protoD := protoDiagnostics[idx]

		// The version from PulumiPolicy.yaml is used, if set, over the version from the diagnostic.
		policyPackVersion := protoD.PolicyPackVersion
		if version != "" {
			policyPackVersion = version
		}

		enforcementLevel, err := convertEnforcementLevel(protoD.EnforcementLevel)
		if err != nil {
			return nil, err
		}

		diagnostics[idx] = AnalyzeDiagnostic{
			PolicyName:        protoD.PolicyName,
			PolicyPackName:    protoD.PolicyPackName,
			PolicyPackVersion: policyPackVersion,
			Description:       protoD.Description,
			Message:           protoD.Message,
			Tags:              protoD.Tags,
			EnforcementLevel:  enforcementLevel,
			URN:               resource.URN(protoD.Urn),
		}
	}

	return diagnostics, nil
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
			maybeAppendEnv("PULUMI_NODEJS_PROJECT", opts.Project)
			maybeAppendEnv("PULUMI_NODEJS_STACK", opts.Stack)
			maybeAppendEnv("PULUMI_NODEJS_DRY_RUN", fmt.Sprintf("%v", opts.DryRun))
		}

		maybeAppendEnv("PULUMI_PROJECT", opts.Project)
		maybeAppendEnv("PULUMI_STACK", opts.Stack)
		maybeAppendEnv("PULUMI_DRY_RUN", fmt.Sprintf("%v", opts.DryRun))
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
