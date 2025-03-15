// Copyright 2018-2024, Pulumi Corporation.
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

package deploy

import (
	"context"
	"errors"
	"fmt"
	"sort"

	uuid "github.com/gofrs/uuid"

	"github.com/pulumi/pulumi/pkg/v3/util/gsync"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// The built-in provider provides resources and functions in the `pulumi` package, such as stack references and the
// `getResource` invoke that powers resource reference hydration.
type builtinProvider struct {
	plugin.NotForwardCompatibleProvider

	context context.Context
	cancel  context.CancelFunc
	diag    diag.Sink

	backendClient BackendClient

	// news is a map of URNs to new resource states that have been produced by the current deployment.
	news *gsync.Map[resource.URN, *resource.State]
	// reads is a map of URNs to resource states that have been read during the current deployment.
	reads *gsync.Map[resource.URN, *resource.State]
}

func newBuiltinProvider(
	backendClient BackendClient,
	news *gsync.Map[resource.URN, *resource.State],
	reads *gsync.Map[resource.URN, *resource.State],
	d diag.Sink,
) *builtinProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &builtinProvider{
		context:       ctx,
		cancel:        cancel,
		backendClient: backendClient,
		news:          news,
		reads:         reads,
		diag:          d,
	}
}

func (p *builtinProvider) Close() error {
	return nil
}

func (p *builtinProvider) Pkg() tokens.Package {
	return "pulumi"
}

func (p *builtinProvider) Handshake(
	context.Context,
	plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	return &plugin.ProviderHandshakeResponse{}, nil
}

func (p *builtinProvider) Parameterize(
	context.Context, plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	return plugin.ParameterizeResponse{}, errors.New("the builtin provider has no parameters")
}

func (p *builtinProvider) Migrate(_ context.Context, req plugin.MigrateRequest) (plugin.MigrateResponse, error) {
	return plugin.MigrateResponse{
		NewID:      req.ID,
		NewInputs:  req.OldInputs,
		NewOutputs: req.OldOutputs,
	}, nil
}

// GetSchema returns the JSON-serialized schema for the provider.
func (p *builtinProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	return plugin.GetSchemaResponse{Schema: []byte("{}")}, nil
}

func (p *builtinProvider) GetMapping(context.Context, plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *builtinProvider) GetMappings(context.Context, plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

// CheckConfig validates the configuration for this resource provider.
func (p *builtinProvider) CheckConfig(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{}, nil
}

// DiffConfig checks what impacts a hypothetical change to this provider's configuration will have on the provider.
func (p *builtinProvider) DiffConfig(context.Context, plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Configure(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

const stackReferenceType = "pulumi:pulumi:StackReference"

func (p *builtinProvider) Check(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	typ := req.URN.Type()
	if typ != stackReferenceType {
		return plugin.CheckResponse{}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}

	// We only need to warn about this in Check. This won't be called for Reads but Creates or Updates will
	// call Check first.
	msg := "The \"pulumi:pulumi:StackReference\" resource type is deprecated. " +
		"Update your SDK or if already up to date raise an issue at https://github.com/pulumi/pulumi/issues."
	p.diag.Warningf(diag.Message(req.URN, msg))

	for k := range req.News {
		if k != "name" {
			return plugin.CheckResponse{
				Failures: []plugin.CheckFailure{{Property: k, Reason: fmt.Sprintf("unknown property \"%v\"", k)}},
			}, nil
		}
	}

	name, ok := req.News["name"]
	if !ok {
		return plugin.CheckResponse{
			Failures: []plugin.CheckFailure{{Property: "name", Reason: `missing required property "name"`}},
		}, nil
	}
	if !name.IsString() && !name.IsComputed() {
		return plugin.CheckResponse{
			Failures: []plugin.CheckFailure{{Property: "name", Reason: `property "name" must be a string`}},
		}, nil
	}
	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *builtinProvider) Diff(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	contract.Assertf(req.URN.Type() == stackReferenceType,
		"expected resource type %v, got %v", stackReferenceType, req.URN.Type())

	if !req.NewInputs["name"].DeepEquals(req.OldInputs["name"]) {
		return plugin.DiffResult{
			Changes:     plugin.DiffSome,
			ReplaceKeys: []resource.PropertyKey{"name"},
		}, nil
	}

	return plugin.DiffResult{Changes: plugin.DiffNone}, nil
}

func (p *builtinProvider) Create(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	contract.Assertf(req.URN.Type() == stackReferenceType,
		"expected resource type %v, got %v", stackReferenceType, req.URN.Type())

	state, err := p.readStackReference(req.Properties)
	if err != nil {
		return plugin.CreateResponse{Status: resource.StatusUnknown}, err
	}

	var id resource.ID
	if !req.Preview {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return plugin.CreateResponse{Status: resource.StatusOK}, err
		}
		id = resource.ID(uuid.String())
	}

	return plugin.CreateResponse{
		ID:         id,
		Properties: state,
		Status:     resource.StatusOK,
	}, nil
}

func (p *builtinProvider) Update(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	contract.Failf("unexpected update for builtin resource %v", req.URN)
	return plugin.UpdateResponse{}, nil
}

func (p *builtinProvider) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	contract.Assertf(req.URN.Type() == stackReferenceType,
		"expected resource type %v, got %v", stackReferenceType, req.URN.Type())

	return plugin.DeleteResponse{Status: resource.StatusOK}, nil
}

func (p *builtinProvider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	contract.Requiref(req.URN != "", "urn", "must not be empty")
	contract.Requiref(req.ID != "", "id", "must not be empty")
	contract.Assertf(req.URN.Type() == stackReferenceType,
		"expected resource type %v, got %v", stackReferenceType, req.URN.Type())

	for k := range req.Inputs {
		if k != "name" {
			return plugin.ReadResponse{Status: resource.StatusUnknown}, fmt.Errorf("unknown property \"%v\"", k)
		}
	}
	// If the name is not provided, we should return an error. This is probably due to a user trying to import
	// this stack reference.
	if _, ok := req.Inputs["name"]; !ok {
		return plugin.ReadResponse{Status: resource.StatusUnknown}, errors.New("stack reference can not be imported")
	}

	outputs, err := p.readStackReference(req.State)
	if err != nil {
		return plugin.ReadResponse{Status: resource.StatusUnknown}, err
	}

	return plugin.ReadResponse{
		ReadResult: plugin.ReadResult{
			Inputs:  req.Inputs,
			Outputs: outputs,
		},
		Status: resource.StatusOK,
	}, nil
}

func (p *builtinProvider) Construct(context.Context, plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	return plugin.ConstructResponse{}, errors.New("builtin resources may not be constructed")
}

const (
	readStackOutputs         = "pulumi:pulumi:readStackOutputs"
	readStackResourceOutputs = "pulumi:pulumi:readStackResourceOutputs" //nolint:gosec // not a credential
	getResource              = "pulumi:pulumi:getResource"
)

func (p *builtinProvider) Invoke(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	var outs resource.PropertyMap
	var err error
	switch req.Tok {
	case readStackOutputs:
		outs, err = p.readStackReference(req.Args)
	case readStackResourceOutputs:
		outs, err = p.readStackResourceOutputs(req.Args)
	case getResource:
		outs, err = p.getResource(req.Args)
	default:
		err = fmt.Errorf("unrecognized function name: '%v'", req.Tok)
	}
	if err != nil {
		return plugin.InvokeResponse{}, err
	}
	return plugin.InvokeResponse{Properties: outs}, nil
}

func (p *builtinProvider) StreamInvoke(
	context.Context, plugin.StreamInvokeRequest,
) (plugin.StreamInvokeResponse, error) {
	return plugin.StreamInvokeResponse{}, errors.New("the builtin provider does not implement streaming invokes")
}

func (p *builtinProvider) Call(context.Context, plugin.CallRequest) (plugin.CallResponse, error) {
	return plugin.CallResult{}, errors.New("the builtin provider does not implement call")
}

func (p *builtinProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	// return an error: this should not be called for the builtin provider
	return workspace.PluginInfo{}, errors.New("the builtin provider does not report plugin info")
}

func (p *builtinProvider) SignalCancellation(context.Context) error {
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
	urnInput, ok := inputs["urn"]
	contract.Assertf(ok, "missing required property 'urn'")
	contract.Assertf(urnInput.IsString(), "expected 'urn' to be a string")

	// When looking up a resource to hydrate it, we'll first check for new states produced by resource registrations. If
	// we fail to find a match there, we'll look for states that have been read.
	urn := resource.URN(urnInput.StringValue())
	state, ok := p.news.Load(urn)
	if !ok {
		state, ok = p.reads.Load(urn)
		if !ok {
			return nil, fmt.Errorf("unknown resource %v", urnInput.StringValue())
		}
	}

	// Take the state lock so we can safely read the Outputs.
	state.Lock.Lock()
	defer state.Lock.Unlock()

	return resource.PropertyMap{
		"urn":      urnInput,
		"id":       resource.NewStringProperty(string(state.ID)),
		"provider": resource.NewStringProperty(state.Provider),
		"state":    resource.NewObjectProperty(state.Outputs),
	}, nil
}
