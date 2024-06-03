package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	uuid "github.com/gofrs/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type builtinProvider struct {
	plugin.NotForwardCompatibleProvider

	context context.Context
	cancel  context.CancelFunc
	diag    diag.Sink

	backendClient BackendClient
	resources     *gsync.Map[resource.URN, *resource.State]
	plugctx       *plugin.Context
	organization  tokens.Name
}

func newBuiltinProvider(
	backendClient BackendClient, resources *gsync.Map[resource.URN, *resource.State],
	d diag.Sink, plugctx *plugin.Context, organization tokens.Name,
) *builtinProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &builtinProvider{
		context:       ctx,
		cancel:        cancel,
		backendClient: backendClient,
		resources:     resources,
		diag:          d,
		plugctx:       plugctx,
		organization:  organization,
	}
}

func (p *builtinProvider) Close() error {
	return nil
}

func (p *builtinProvider) Pkg() tokens.Package {
	return "pulumi"
}

func (p *builtinProvider) Parameterize(
	context.Context, plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	return plugin.ParameterizeResponse{}, errors.New("the builtin provider has no parameters")
}

// GetSchema returns the JSON-serialized schema for the provider.
func (p *builtinProvider) GetSchema(plugin.GetSchemaRequest) ([]byte, error) {
	return []byte("{}"), nil
}

func (p *builtinProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (p *builtinProvider) GetMappings(key string) ([]string, error) {
	return []string{}, nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *builtinProvider) CheckConfig(urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *builtinProvider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Configure(props resource.PropertyMap) error {
	return nil
}

const stackReferenceType = "pulumi:pulumi:StackReference"

func (p *builtinProvider) Check(urn resource.URN, state, inputs resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	typ := urn.Type()
	if typ != stackReferenceType {
		return nil, nil, fmt.Errorf("unrecognized resource type '%v'", urn.Type())
	}

	// We only need to warn about this in Check. This won't be called for Reads but Creates or Updates will
	// call Check first.
	msg := "The \"pulumi:pulumi:StackReference\" resource type is deprecated. " +
		"Update your SDK or if already up to date raise an issue at https://github.com/pulumi/pulumi/issues."
	p.diag.Warningf(diag.Message(urn, msg))

	for k := range inputs {
		if k != "name" {
			return nil, []plugin.CheckFailure{{Property: k, Reason: fmt.Sprintf("unknown property \"%v\"", k)}}, nil
		}
	}

	name, ok := inputs["name"]
	if !ok {
		return nil, []plugin.CheckFailure{{Property: "name", Reason: `missing required property "name"`}}, nil
	}
	if !name.IsString() && !name.IsComputed() {
		return nil, []plugin.CheckFailure{{Property: "name", Reason: `property "name" must be a string`}}, nil
	}
	return inputs, nil, nil
}

func (p *builtinProvider) Diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	contract.Assertf(urn.Type() == stackReferenceType, "expected resource type %v, got %v", stackReferenceType, urn.Type())

	if !newInputs["name"].DeepEquals(oldOutputs["name"]) {
		return plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"name"},
		}, nil
	}

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Create(urn resource.URN, inputs resource.PropertyMap, timeout float64,
	preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	contract.Assertf(urn.Type() == stackReferenceType, "expected resource type %v, got %v", stackReferenceType, urn.Type())

	state, err := p.readStackReference(inputs)
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	var id resource.ID
	if !preview {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return "", nil, resource.StatusOK, err
		}
		id = resource.ID(uuid.String())
	}

	return id, state, resource.StatusOK, nil
}

func (p *builtinProvider) Update(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs, newInputs resource.PropertyMap,
	timeout float64, ignoreChanges []string, preview bool,
) (resource.PropertyMap, resource.Status, error) {
	contract.Failf("unexpected update for builtin resource %v", urn)
	contract.Assertf(urn.Type() == stackReferenceType, "expected resource type %v, got %v", stackReferenceType, urn.Type())

	return oldOutputs, resource.StatusOK, errors.New("unexpected update for builtin resource")
}

func (p *builtinProvider) Delete(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs resource.PropertyMap, timeout float64,
) (resource.Status, error) {
	contract.Assertf(urn.Type() == stackReferenceType, "expected resource type %v, got %v", stackReferenceType, urn.Type())

	return resource.StatusOK, nil
}

func (p *builtinProvider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	contract.Requiref(urn != "", "urn", "must not be empty")
	contract.Requiref(id != "", "id", "must not be empty")
	contract.Assertf(urn.Type() == stackReferenceType, "expected resource type %v, got %v", stackReferenceType, urn.Type())

	for k := range inputs {
		if k != "name" {
			return plugin.ReadResult{}, resource.StatusUnknown, fmt.Errorf("unknown property \"%v\"", k)
		}
	}

	outputs, err := p.readStackReference(state)
	if err != nil {
		return plugin.ReadResult{}, resource.StatusUnknown, err
	}

	return plugin.ReadResult{
		Inputs:  inputs,
		Outputs: outputs,
	}, resource.StatusOK, nil
}

func isPrimitiveValue(value interface{}) bool {
	switch value.(type) {
	case string, int, bool:
		return true
	default:
		return false
	}
}

func createConfigValue(rawValue interface{}) (string, error) {
	if isPrimitiveValue(rawValue) {
		configValueContent := fmt.Sprintf("%v", rawValue)
		return configValueContent, nil
	}
	value, err := workspace.SimplifyMarshalledValue(rawValue)
	if err != nil {
		return "", err
	}
	configValueJSON, jsonError := json.Marshal(value)
	if jsonError != nil {
		return "", jsonError
	}
	return string(configValueJSON), nil
}

func (p *builtinProvider) Construct(info plugin.ConstructInfo, typ tokens.Type, name string, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions,
) (plugin.ConstructResult, error) {
	if typ == "pulumi:pulumi:Stack" {

		// Because we embed stack names into the qualified type, we need to ensure that the stack name is valid to be
		// embedded in that part of the URN. We limit it to the same subset as StackNames
		_, err := tokens.ParseStackName(name)
		if err != nil {
			return plugin.ConstructResult{}, err
		}

		source := inputs["source"].StringValue()

		_, err = os.Stat(source)
		if os.IsNotExist(err) {
			template, err := workspace.RetrieveTemplates(source, false, workspace.TemplateKindPulumiProject)
			if err != nil {
				return plugin.ConstructResult{}, err
			}
			defer func() {
				contract.IgnoreError(template.Delete())
			}()
			source = template.SubDirectory
		} else if err != nil {
			return plugin.ConstructResult{}, err
		}

		configSource := inputs["config"]
		if configSource.IsNull() {
			configSource = resource.NewObjectProperty(resource.PropertyMap{})
		}
		configInputs := configSource.ObjectValue()

		// grpc channel -> client for resource monitor
		var monitorConn *grpc.ClientConn
		var monitor pulumirpc.ResourceMonitorClient
		conn, err := grpc.Dial(
			info.MonitorAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			rpcutil.GrpcChannelOptions(),
		)
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("connecting to resource monitor over RPC: %w", err)
		}
		monitorConn = conn
		monitor = pulumirpc.NewResourceMonitorClient(monitorConn)

		registerSubStackResourceResponse, err := monitor.RegisterResource(p.context, &pulumirpc.RegisterResourceRequest{
			Type: string(typ),
			// Top-level stacks are named as PROJECT-STACK but in sub-stacks we use the name specified in the Program resource.
			Name:   name,
			Parent: string(parent),
		})
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("registering sub-stack resource: %w", err)
		}

		subStackResourceUrn := registerSubStackResourceResponse.GetUrn()

		// TODO: Do we need an interrupt handler?
		cancelChannel := make(chan bool)
		go func() {
			<-p.context.Done()
			close(cancelChannel)
		}()

		// Create new monitor server (with facade)
		// Fire up a gRPC server and start listening for incomings.
		monitorProxy := subStackMonitorProxy{
			monitor:      monitor,
			subStackUrn:  resource.URN(subStackResourceUrn),
			dependencies: options.Dependencies,
		}
		monitorServer, err := rpcutil.ServeWithOptions(rpcutil.ServeOptions{
			Cancel: cancelChannel,
			Init: func(srv *grpc.Server) error {
				pulumirpc.RegisterResourceMonitorServer(srv, &monitorProxy)
				return nil
			},
			// Options: sourceEvalServeOptions(src.plugctx, tracingSpan),
		})
		if err != nil {
			return plugin.ConstructResult{}, err
		}

		resolvedSource, err := filepath.Abs(source)
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("resolving source: %w", err)
		}
		projectPath, err := workspace.DetectProjectPathFrom(resolvedSource)
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("detecting project path: %w", err)
		}
		// Sub-programs require that the project file must be directly in the source path and not in a parent directory.
		if filepath.Dir(projectPath) != resolvedSource {
			return plugin.ConstructResult{}, fmt.Errorf("project path %s is not a parent of %s", projectPath, resolvedSource)
		}
		project, err := workspace.LoadProject(projectPath)
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("loading project: %w", err)
		}

		// We can't yet handle unknowns during a preview as unknowns can't be represented in a config object yet.
		// Therefore we just mark the whole output as unknown during preview for now.
		// In the future we should be able to handle unknowns in config objects.
		if configInputs.ContainsUnknowns() {
			contract.Assertf(info.DryRun, "config inputs must be known in non-dry-run mode")
			return plugin.ConstructResult{
				URN:     resource.URN(subStackResourceUrn),
				Outputs: resource.PropertyMap{"outputs": resource.MakeComputed(resource.NewStringProperty(""))},
			}, nil
		}

		// Execute the program pointing to the new monitor server
		rt := project.Runtime.Name()
		rtopts := project.Runtime.Options()
		programDir, entryPoint := resolvedSource, "."
		if project.Main != "" {
			programDir = filepath.Join(resolvedSource, filepath.Dir(project.Main))
			entryPoint = filepath.Base(project.Main)
		}
		langhost, err := p.plugctx.Host.LanguageRuntime(
			rt, plugin.NewProgramInfo(resolvedSource, programDir, entryPoint, rtopts))
		if err != nil {
			return plugin.ConstructResult{}, fmt.Errorf("failed to launch language host %s: %w", rt, err)
		}
		contract.Assertf(langhost != nil, "expected non-nil language host %s", rt)

		configs := map[config.Key]string{}

		for key, val := range project.Config {
			configVal := val.Value
			if configVal == nil {
				configVal = val.Default
			}

			if configVal != nil {
				configValue, err := createConfigValue(configVal)
				if err != nil {
					return plugin.ConstructResult{}, err
				}
				configKey := config.MustMakeKey(info.Project, key)
				configs[configKey] = configValue
			}
		}
		secretKeys := make([]config.Key, 0)
		for key, val := range configInputs {
			unwrappedVal := val
			if val.IsOutput() {
				unwrappedVal = val.OutputValue().Element
			}

			var encodedValue string
			if unwrappedVal.IsString() {
				encodedValue = unwrappedVal.StringValue()
			} else if unwrappedVal.IsBool() {
				encodedValue = strconv.FormatBool(unwrappedVal.BoolValue())
			} else if unwrappedVal.IsNumber() {
				encodedValue = strconv.FormatFloat(unwrappedVal.NumberValue(), 'f', -1, 64)
			} else {
				marshalled, err := plugin.MarshalPropertyValue(key, unwrappedVal, plugin.MarshalOptions{
					KeepUnknowns: true, KeepSecrets: true, KeepOutputValues: true,
				})
				if err != nil {
					return plugin.ConstructResult{}, err
				}
				jsonValue, err := json.Marshal(marshalled)
				if err != nil {
					return plugin.ConstructResult{}, err
				}
				encodedValue = string(jsonValue)
			}
			configKey := config.MustMakeKey(info.Project, string(key))
			if val.ContainsSecrets() {
				secretKeys = append(secretKeys, configKey)
			}
			configs[configKey] = encodedValue
		}
		// Now run the actual program.
		progerr, bail, err := langhost.Run(plugin.RunInfo{
			MonitorAddress:    fmt.Sprintf("127.0.0.1:%d", monitorServer.Port),
			Stack:             info.Stack,
			Project:           info.Project,
			Pwd:               resolvedSource,
			Args:              []string{}, // TODO: make this an arg
			Config:            configs,
			ConfigSecretKeys:  secretKeys,
			ConfigPropertyMap: configInputs,
			DryRun:            info.DryRun,
			Parallel:          info.Parallel,
			Organization:      string(p.organization),
		})

		// Check if we were asked to Bail.  This a special random constant used for that
		// purpose.
		if err == nil && bail {
			return plugin.ConstructResult{}, result.BailErrorf("run bailed")
		}

		if err == nil && progerr != "" {
			// If the program had an unhandled error; propagate it to the caller.
			err = fmt.Errorf("an unhandled error occurred: %v", progerr)
		}
		if err != nil {
			return plugin.ConstructResult{}, err
		}

		outPropMap, err := plugin.UnmarshalProperties(monitorProxy.outputs,
			plugin.MarshalOptions{KeepUnknowns: true, KeepSecrets: true, SkipInternalKeys: true})
		if err != nil {
			return plugin.ConstructResult{}, err
		}

		return plugin.ConstructResult{
			URN:     resource.URN(subStackResourceUrn),
			Outputs: outPropMap,
		}, nil
	}
	return plugin.ConstructResult{}, errors.New("builtin resources may not be constructed")
}

var _ pulumirpc.ResourceMonitorServer = (*subStackMonitorProxy)(nil)

type subStackMonitorProxy struct {
	pulumirpc.UnimplementedResourceMonitorServer
	monitor      pulumirpc.ResourceMonitorClient
	subStackUrn  resource.URN
	outputs      *structpb.Struct
	dependencies []resource.URN
}

func (p *subStackMonitorProxy) Invoke(
	ctx context.Context, req *pulumirpc.ResourceInvokeRequest,
) (*pulumirpc.InvokeResponse, error) {
	return p.monitor.Invoke(ctx, req)
}

func (p *subStackMonitorProxy) StreamInvoke(
	req *pulumirpc.ResourceInvokeRequest, server pulumirpc.ResourceMonitor_StreamInvokeServer,
) error {
	return errors.New("not supported")
}

func (p *subStackMonitorProxy) Call(
	ctx context.Context, in *pulumirpc.ResourceCallRequest,
) (*pulumirpc.CallResponse, error) {
	return p.monitor.Call(ctx, in)
}

func (p *subStackMonitorProxy) ReadResource(
	ctx context.Context, req *pulumirpc.ReadResourceRequest,
) (*pulumirpc.ReadResourceResponse, error) {
	// TODO: Adjust URN
	return p.monitor.ReadResource(ctx, req)
}

func (p *subStackMonitorProxy) RegisterResource(
	ctx context.Context, req *pulumirpc.RegisterResourceRequest,
) (*pulumirpc.RegisterResourceResponse, error) {
	if req.Type == "pulumi:pulumi:Stack" {
		return &pulumirpc.RegisterResourceResponse{
			Urn: string(p.subStackUrn),
		}, nil
	}
	for _, dep := range p.dependencies {
		req.Dependencies = append(req.Dependencies, string(dep))
	}
	if req.Parent == "" {
		req.Parent = string(p.subStackUrn)
	}
	return p.monitor.RegisterResource(ctx, req)
}

func (p *subStackMonitorProxy) RegisterResourceOutputs(
	ctx context.Context, req *pulumirpc.RegisterResourceOutputsRequest,
) (*emptypb.Empty, error) {
	if req.Urn == string(p.subStackUrn) {
		outputs := structpb.Struct{
			Fields: map[string]*structpb.Value{
				"outputs": structpb.NewStructValue(req.Outputs),
			},
		}
		p.outputs = &outputs
		req.Outputs = &outputs
	}
	return p.monitor.RegisterResourceOutputs(ctx, req)
}

func (p *subStackMonitorProxy) SupportsFeature(
	ctx context.Context, req *pulumirpc.SupportsFeatureRequest,
) (*pulumirpc.SupportsFeatureResponse, error) {
	return p.monitor.SupportsFeature(ctx, req)
}

const (
	readStackOutputs         = "pulumi:pulumi:readStackOutputs"
	readStackResourceOutputs = "pulumi:pulumi:readStackResourceOutputs" //nolint:gosec // not a credential
	getResource              = "pulumi:pulumi:getResource"
)

func (p *builtinProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	switch tok {
	case readStackOutputs:
		outs, err := p.readStackReference(args)
		if err != nil {
			return nil, nil, err
		}
		return outs, nil, nil
	case readStackResourceOutputs:
		outs, err := p.readStackResourceOutputs(args)
		if err != nil {
			return nil, nil, err
		}
		return outs, nil, nil
	case getResource:
		outs, err := p.getResource(args)
		if err != nil {
			return nil, nil, err
		}
		return outs, nil, nil
	default:
		return nil, nil, fmt.Errorf("unrecognized function name: '%v'", tok)
	}
}

func (p *builtinProvider) StreamInvoke(
	tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]plugin.CheckFailure, error) {
	return nil, errors.New("the builtin provider does not implement streaming invokes")
}

func (p *builtinProvider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions,
) (plugin.CallResult, error) {
	return plugin.CallResult{}, errors.New("the builtin provider does not implement call")
}

func (p *builtinProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	// return an error: this should not be called for the builtin provider
	return workspace.PluginInfo{}, errors.New("the builtin provider does not report plugin info")
}

func (p *builtinProvider) SignalCancellation() error {
	p.cancel()
	return nil
}

func (p *builtinProvider) readStackReference(inputs resource.PropertyMap) (resource.PropertyMap, error) {
	name, ok := inputs["name"]
	contract.Assertf(ok, "missing required property 'name'")
	contract.Assertf(name.IsString(), "expected 'name' to be a string")

	if p.backendClient == nil {
		return nil, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackOutputs(p.context, name.StringValue())
	if err != nil {
		return nil, err
	}

	secretOutputs := make([]resource.PropertyValue, 0)
	for k, v := range outputs {
		if v.ContainsSecrets() {
			secretOutputs = append(secretOutputs, resource.NewStringProperty(string(k)))
		}
	}

	// Sort the secret outputs so the order is deterministic, to avoid spurious diffs during updates.
	sort.Slice(secretOutputs, func(i, j int) bool {
		return secretOutputs[i].String() < secretOutputs[j].String()
	})

	return resource.PropertyMap{
		"name":              name,
		"outputs":           resource.NewObjectProperty(outputs),
		"secretOutputNames": resource.NewArrayProperty(secretOutputs),
	}, nil
}

func (p *builtinProvider) readStackResourceOutputs(inputs resource.PropertyMap) (resource.PropertyMap, error) {
	name, ok := inputs["stackName"]
	contract.Assertf(ok, "missing required property 'stackName'")
	contract.Assertf(name.IsString(), "expected 'stackName' to be a string")

	if p.backendClient == nil {
		return nil, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackResourceOutputs(p.context, name.StringValue())
	if err != nil {
		return nil, err
	}

	return resource.PropertyMap{
		"name":    name,
		"outputs": resource.NewObjectProperty(outputs),
	}, nil
}

func (p *builtinProvider) getResource(inputs resource.PropertyMap) (resource.PropertyMap, error) {
	urn, ok := inputs["urn"]
	contract.Assertf(ok, "missing required property 'urn'")
	contract.Assertf(urn.IsString(), "expected 'urn' to be a string")

	state, ok := p.resources.Load(resource.URN(urn.StringValue()))
	if !ok {
		return nil, fmt.Errorf("unknown resource %v", urn.StringValue())
	}

	// Take the state lock so we can safely read the Outputs.
	state.Lock.Lock()
	defer state.Lock.Unlock()

	return resource.PropertyMap{
		"urn":   urn,
		"id":    resource.NewStringProperty(string(state.ID)),
		"state": resource.NewObjectProperty(state.Outputs),
	}, nil
}
