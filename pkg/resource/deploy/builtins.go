package deploy

import (
	"context"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"

	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/joincontext"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

type builtinProvider struct {
	backendClient BackendClient
	context       context.Context
	cancel        context.CancelFunc
}

func newBuiltinProvider(backendClient BackendClient) *builtinProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &builtinProvider{
		backendClient: backendClient,
		context:       ctx,
		cancel:        cancel,
	}
}

func (p *builtinProvider) requestContext(ctx context.Context) context.Context {
	return joincontext.Join(p.context, ctx)
}

func (p *builtinProvider) Close() error {
	return nil
}

func (p *builtinProvider) Pkg() tokens.Package {
	return "pulumi"
}

// GetSchema returns the JSON-serialized schema for the provider.
func (p *builtinProvider) GetSchema(ctx context.Context, version int) ([]byte, error) {
	return []byte("{}"), nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *builtinProvider) CheckConfig(ctx context.Context, urn resource.URN, olds,
	news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	return nil, nil, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *builtinProvider) DiffConfig(ctx context.Context, urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {
	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Configure(ctx context.Context, props resource.PropertyMap) error {
	return nil
}

const stackReferenceType = "pulumi:pulumi:StackReference"

func (p *builtinProvider) Check(ctx context.Context, urn resource.URN, state, inputs resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	typ := urn.Type()
	if typ != stackReferenceType {
		return nil, nil, errors.Errorf("unrecognized resource type '%v'", urn.Type())
	}

	var name resource.PropertyValue
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

func (p *builtinProvider) Diff(ctx context.Context,
	urn resource.URN, id resource.ID, state, inputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string) (plugin.DiffResult, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	if !inputs["name"].DeepEquals(state["name"]) {
		return plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"name"},
		}, nil
	}

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Create(ctx context.Context, urn resource.URN, inputs resource.PropertyMap, timeout float64,
	preview bool) (resource.ID, resource.PropertyMap, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	state, err := p.readStackReference(ctx, inputs)
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	var id resource.ID
	if !preview {
		id = resource.ID(uuid.NewV4().String())
	}

	return id, state, resource.StatusOK, nil
}

func (p *builtinProvider) Update(ctx context.Context,
	urn resource.URN, id resource.ID, state, inputs resource.PropertyMap, timeout float64,
	ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error) {

	contract.Failf("unexpected update for builtin resource %v", urn)
	contract.Assert(urn.Type() == stackReferenceType)

	return state, resource.StatusOK, errors.New("unexpected update for builtin resource")
}

func (p *builtinProvider) Delete(ctx context.Context, urn resource.URN, id resource.ID,
	state resource.PropertyMap, timeout float64) (resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	return resource.StatusOK, nil
}

func (p *builtinProvider) Read(ctx context.Context, urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	outputs, err := p.readStackReference(ctx, state)
	if err != nil {
		return plugin.ReadResult{}, resource.StatusUnknown, err
	}

	return plugin.ReadResult{
		Inputs:  inputs,
		Outputs: outputs,
	}, resource.StatusOK, nil
}

func (p *builtinProvider) Construct(ctx context.Context,
	info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{}, errors.New("builtin resources may not be constructed")
}

const readStackOutputs = "pulumi:pulumi:readStackOutputs"
const readStackResourceOutputs = "pulumi:pulumi:readStackResourceOutputs"

func (p *builtinProvider) Invoke(ctx context.Context, tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {

	switch tok {
	case readStackOutputs:
		outs, err := p.readStackReference(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return outs, nil, nil
	case readStackResourceOutputs:
		outs, err := p.readStackResourceOutputs(ctx, args)
		if err != nil {
			return nil, nil, err
		}
		return outs, nil, nil
	default:
		return nil, nil, errors.Errorf("unrecognized function name: '%v'", tok)
	}
}

func (p *builtinProvider) StreamInvoke(ctx context.Context,
	tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(context.Context, resource.PropertyMap) error) ([]plugin.CheckFailure, error) {

	return nil, fmt.Errorf("the builtin provider does not implement streaming invokes")
}

func (p *builtinProvider) GetPluginInfo(ctx context.Context) (workspace.PluginInfo, error) {
	// return an error: this should not be called for the builtin provider
	return workspace.PluginInfo{}, errors.New("the builtin provider does not report plugin info")
}

func (p *builtinProvider) SignalCancellation(ctx context.Context) error {
	p.cancel()
	return nil
}

func (p *builtinProvider) readStackReference(ctx context.Context,
	inputs resource.PropertyMap) (resource.PropertyMap, error) {

	name, ok := inputs["name"]
	contract.Assert(ok)
	contract.Assert(name.IsString())

	if p.backendClient == nil {
		return nil, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackOutputs(p.requestContext(ctx), name.StringValue())
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

func (p *builtinProvider) readStackResourceOutputs(ctx context.Context,
	inputs resource.PropertyMap) (resource.PropertyMap, error) {

	name, ok := inputs["stackName"]
	contract.Assert(ok)
	contract.Assert(name.IsString())

	if p.backendClient == nil {
		return nil, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackResourceOutputs(p.requestContext(ctx), name.StringValue())
	if err != nil {
		return nil, err
	}

	return resource.PropertyMap{
		"name":    name,
		"outputs": resource.NewObjectProperty(outputs),
	}, nil
}
