package deploy

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
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

func (p *builtinProvider) Close() error {
	return nil
}

func (p *builtinProvider) Pkg() tokens.Package {
	return "pulumi"
}

// CheckConfig validates the configuration for this resource provider.
func (p *builtinProvider) CheckConfig(olds,
	news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {

	return nil, nil, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *builtinProvider) DiffConfig(olds, news resource.PropertyMap) (plugin.DiffResult, error) {

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Configure(props resource.PropertyMap) error {
	return nil
}

const stackReferenceType = "pulumi:pulumi:StackReference"

func (p *builtinProvider) Check(urn resource.URN, state, inputs resource.PropertyMap,
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

func (p *builtinProvider) Diff(urn resource.URN, id resource.ID, state, inputs resource.PropertyMap,
	allowUnknowns bool) (plugin.DiffResult, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	if !inputs["name"].DeepEquals(state["name"]) {
		return plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"name"},
		}, nil
	}

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Create(urn resource.URN,
	inputs resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	state, err := p.readStackReference(inputs)
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}
	id := resource.ID(uuid.NewV4().String())
	return id, state, resource.StatusOK, nil
}

func (p *builtinProvider) Update(urn resource.URN, id resource.ID, state,
	inputs resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {

	contract.Failf("unexpected update for builtin resource %v", urn)
	contract.Assert(urn.Type() == stackReferenceType)

	return state, resource.StatusOK, errors.New("unexpected update for builtin resource")
}

func (p *builtinProvider) Delete(urn resource.URN, id resource.ID,
	state resource.PropertyMap) (resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	return resource.StatusOK, nil
}

func (p *builtinProvider) Read(urn resource.URN, id resource.ID,
	state resource.PropertyMap) (plugin.ReadResult, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	outputs, err := p.readStackReference(state)
	if err != nil {
		return plugin.ReadResult{}, resource.StatusUnknown, err
	}

	return plugin.ReadResult{
		Inputs:  state,
		Outputs: outputs,
	}, resource.StatusOK, nil
}

func (p *builtinProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {

	return nil, nil, errors.Errorf("unrecognized function name: '%v'", tok)
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
	contract.Assert(ok)
	contract.Assert(name.IsString())

	if p.backendClient == nil {
		return nil, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackOutputs(p.context, name.StringValue())
	if err != nil {
		return nil, err
	}

	return resource.PropertyMap{
		"name":    name,
		"outputs": resource.NewObjectProperty(outputs),
	}, nil
}
