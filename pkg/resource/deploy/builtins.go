package deploy

import (
	"context"

	"github.com/pkg/errors"
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
func (p *builtinProvider) CheckConfig(olds, news resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *builtinProvider) DiffConfig(olds, news resource.PropertyMap) (plugin.DiffResult, error) {

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Configure(props resource.PropertyMap) error {
	return nil
}

func (p *builtinProvider) Check(urn resource.URN, olds, news resource.PropertyMap,
	allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error) {

	return nil, nil, errors.Errorf("unrecognized resource type '%v'", urn.Type())
}

func (p *builtinProvider) Diff(urn resource.URN, id resource.ID, olds, news resource.PropertyMap,
	allowUnknowns bool) (plugin.DiffResult, error) {

	contract.Fail()
	return plugin.DiffResult{}, errors.Errorf("the builtin provider has no resources")
}

func (p *builtinProvider) Create(urn resource.URN,
	news resource.PropertyMap) (resource.ID, resource.PropertyMap, resource.Status, error) {

	contract.Fail()
	return "", nil, resource.StatusUnknown, errors.Errorf("the builtin provider has no resources")
}

func (p *builtinProvider) Update(urn resource.URN, id resource.ID, olds,
	news resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {

	contract.Fail()
	return nil, resource.StatusUnknown, errors.Errorf("the builtin provider has no resources")
}

func (p *builtinProvider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap) (resource.Status, error) {
	contract.Fail()
	return resource.StatusUnknown, errors.Errorf("the builtin provider has no resources")
}

func (p *builtinProvider) Read(urn resource.URN, id resource.ID,
	props resource.PropertyMap) (resource.PropertyMap, resource.Status, error) {
	return nil, resource.StatusUnknown, errors.Errorf("the builtin provider has no resources")
}

func (p *builtinProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error) {

	switch tok.Name() {
	case "getStack":
		if p.backendClient == nil {
			return nil, nil, errors.New("no backend client is available")
		}

		name, ok := args["name"]
		if !ok {
			return nil, []plugin.CheckFailure{{Property: "name", Reason: "missing required property 'name'"}}, nil
		}
		if !name.IsString() {
			return nil, []plugin.CheckFailure{{Property: "name", Reason: "'name' must be a string"}}, nil
		}

		result, err := p.backendClient.GetStackOutputs(p.context, name.StringValue())
		if err != nil {
			return nil, nil, err
		}
		return result, nil, nil
	default:
		return nil, nil, errors.Errorf("unrecognized function name: '%v'", tok)
	}
}

func (p *builtinProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	// return an error: this should not be called for the builtin provider
	return workspace.PluginInfo{}, errors.New("the builtin provider does not report plugin info")
}

func (p *builtinProvider) SignalCancellation() error {
	p.cancel()
	return nil
}
