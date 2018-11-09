package deploy

import (
	"context"

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

func (p *builtinProvider) label() string {
	return "Builtins"
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

const stackReferenceType = "pulumi:service:StackReference"

func (p *builtinProvider) Check(urn resource.URN, state, inputs resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	typ := urn.Type()
	if typ != stackReferenceType {
		return nil, nil, errors.Errorf("unrecognized resource type '%v'", urn.Type())
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
		// We delete stack references before replacing them because the reference to the old stack is promptly stale.
		return plugin.DiffResult{
			Changes:             plugin.DiffSome,
			ReplaceKeys:         []resource.PropertyKey{"name"},
			DeleteBeforeReplace: true,
		}, nil
	}

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Create(urn resource.URN,
	inputs resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	if p.backendClient == nil {
		return "", nil, resource.StatusUnknown, errors.New("no backend client is available")
	}

	name, ok := inputs["name"]
	contract.Assert(ok)
	contract.Assert(name.IsString())

	outputs, err := p.backendClient.GetStackOutputs(p.context, name.StringValue())
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	id := resource.ID(uuid.NewV4().String())
	state := resource.PropertyMap{
		"name":    name,
		"outputs": resource.NewObjectProperty(outputs),
	}

	return id, state, resource.StatusOK, nil
}

func (p *builtinProvider) Update(urn resource.URN, id resource.ID, state,
	inputs resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {

	contract.Assert(urn.Type() == stackReferenceType)

	// If "name" changed, this should have been a replace.
	contract.Assert(inputs["name"].DeepEquals(state["name"]))

	return state, resource.StatusOK, nil
}

func (p *builtinProvider) Delete(urn resource.URN, id resource.ID, state resource.PropertyMap) (resource.Status, error) {
	contract.Assert(urn.Type() == stackReferenceType)

	return resource.StatusOK, nil
}

func (p *builtinProvider) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	return nil, resource.StatusUnknown, errors.Errorf("the builtin provider has no resources that can be read")
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
