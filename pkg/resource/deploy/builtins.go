// Copyright 2018, Pulumi Corporation.
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

	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"

	uuid "github.com/gofrs/uuid"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/proto"

	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi-internal/gsync"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
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
	news *gsync.Map[resource.URN, *pkgresource.State]
	// reads is a map of URNs to resource states that have been read during the current deployment.
	reads *gsync.Map[resource.URN, *pkgresource.State]

	// callbacks is the shared cache of gRPC clients for language-side callback servers, used to
	// invoke callbacks such as the pulumi:index:Stash reducer.
	callbacks *plugin.CallbacksClientCache
}

func newBuiltinProvider(
	backendClient BackendClient,
	news *gsync.Map[resource.URN, *pkgresource.State],
	reads *gsync.Map[resource.URN, *pkgresource.State],
	d diag.Sink,
	callbacks *plugin.CallbacksClientCache,
) *builtinProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &builtinProvider{
		context:       ctx,
		cancel:        cancel,
		backendClient: backendClient,
		news:          news,
		reads:         reads,
		diag:          d,
		callbacks:     callbacks,
	}
}

func (p *builtinProvider) Close() error {
	return nil
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

const (
	stackReferenceType = "pulumi:pulumi:StackReference"
	stashType          = "pulumi:index:Stash"
)

// Names of the input properties used to configure a stash reducer callback. The "reducer"
// input, when present, is an object shaped like a Callback ({ target, token }) that identifies
// a language-side callback the builtin provider will invoke during Check. It is consumed by
// builtinProvider.Check and is never persisted in the resource's state.
const (
	stashReducerKey resource.PropertyKey = "reducer"
	// stashReducedKey is the private, checked input property produced by builtinProvider.Check
	// that carries the result of the reducer. It is only visible in the checked-inputs flow
	// (Diff/Create/Update receive it via News/NewInputs); it does not surface as a public output.
	stashReducedKey resource.PropertyKey = "__reduced"
)

// invokeStashReducer invokes the user-supplied reducer callback with (oldInput, oldOutput,
// newInput) and returns the reduced value. Callers must have already checked for
// computed/unknown values.
func (p *builtinProvider) invokeStashReducer(
	ctx context.Context, target, token string, oldInput, oldOutput, newInput resource.PropertyValue,
) (resource.PropertyValue, error) {
	if p.callbacks == nil {
		return resource.PropertyValue{}, errors.New("no callbacks client cache configured for builtin provider")
	}
	client, err := p.callbacks.Get(target)
	if err != nil {
		return resource.PropertyValue{}, err
	}

	marshalOpts := plugin.MarshalOptions{
		Label:        "stash-reducer",
		KeepUnknowns: true,
		KeepSecrets:  true,
	}
	oldInputPB, err := plugin.MarshalPropertyValue("old_input", oldInput, marshalOpts)
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("marshaling stash reducer old_input: %w", err)
	}
	oldOutputPB, err := plugin.MarshalPropertyValue("old_output", oldOutput, marshalOpts)
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("marshaling stash reducer old_output: %w", err)
	}
	newInputPB, err := plugin.MarshalPropertyValue("new_input", newInput, marshalOpts)
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("marshaling stash reducer new_input: %w", err)
	}
	reqBytes, err := proto.Marshal(&pulumirpc.StashReduceRequest{
		OldInput:  oldInputPB,
		OldOutput: oldOutputPB,
		NewInput:  newInputPB,
	})
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("marshaling StashReduceRequest: %w", err)
	}

	resp, err := client.Invoke(ctx, &pulumirpc.CallbackInvokeRequest{
		Token:   token,
		Request: reqBytes,
	})
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("invoking stash reducer: %w", err)
	}
	var respMsg pulumirpc.StashReduceResponse
	if err := proto.Unmarshal(resp.Response, &respMsg); err != nil {
		return resource.PropertyValue{}, fmt.Errorf("unmarshaling StashReduceResponse: %w", err)
	}
	if respMsg.Reduced == nil {
		return resource.PropertyValue{}, errors.New("stash reducer returned nil reduced value")
	}
	reduced, err := plugin.UnmarshalPropertyValue("reduced", respMsg.Reduced, marshalOpts)
	if err != nil {
		return resource.PropertyValue{}, fmt.Errorf("unmarshaling reducer response: %w", err)
	}
	if reduced == nil {
		return resource.PropertyValue{}, errors.New("stash reducer returned nil reduced value")
	}
	return *reduced, nil
}

// checkStash validates a pulumi:index:Stash resource's inputs. If a reducer callback has been
// supplied via the "reducer" input (an object of the shape { target, token }), it is invoked
// here to produce a __reduced private checked input consumed by Diff/Create/Update. The reducer
// object is stripped from the returned properties so it never leaks into persisted state.
func (p *builtinProvider) checkStash(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	inputVal, ok := req.News["input"]
	if !ok {
		return plugin.CheckResponse{
			Failures: []plugin.CheckFailure{{Property: "input", Reason: `missing required property "input"`}},
		}, nil
	}

	// Any input keys other than "input" and "reducer" are unknown.
	for k := range req.News {
		if k != "input" && k != stashReducerKey {
			return plugin.CheckResponse{
				Failures: []plugin.CheckFailure{{Property: k, Reason: fmt.Sprintf("unknown property \"%v\"", k)}},
			}, nil
		}
	}

	out := resource.PropertyMap{
		"input": inputVal,
	}

	// Determine the reduced value.
	if reducerVal, hasReducer := req.News[stashReducerKey]; hasReducer {
		if !reducerVal.IsObject() {
			return plugin.CheckResponse{
				Failures: []plugin.CheckFailure{{
					Property: stashReducerKey,
					Reason:   `"reducer" must be an object with "target" and "token" string properties`,
				}},
			}, nil
		}
		reducerMap := reducerVal.ObjectValue()
		targetVal, hasTarget := reducerMap["target"]
		tokenVal, hasToken := reducerMap["token"]
		if !hasTarget || !hasToken || !targetVal.IsString() || !tokenVal.IsString() {
			return plugin.CheckResponse{
				Failures: []plugin.CheckFailure{{
					Property: stashReducerKey,
					Reason:   `"reducer" must have string "target" and "token" properties`,
				}},
			}, nil
		}
		target, token := targetVal.StringValue(), tokenVal.StringValue()

		// On create (no prior outputs) the reducer is not invoked; the initial reduced value
		// is just the current input.
		oldOutput, hasOld := req.OldOutputs["output"]
		oldInput := req.Olds["input"]
		if !hasOld {
			out[stashReducedKey] = inputVal
		} else if inputVal.ContainsUnknowns() || oldOutput.ContainsUnknowns() || oldInput.ContainsUnknowns() {
			// Can't run a user function against an unknown; propagate unknown.
			out[stashReducedKey] = resource.MakeComputed(resource.NewProperty(""))
		} else {
			// Unwrap secrets for the callback; re-wrap the result if any side was secret.
			oldInputPlain, oldInputSecret := unwrapSecret(oldInput)
			oldOutputPlain, oldOutputSecret := unwrapSecret(oldOutput)
			newInputPlain, newInputSecret := unwrapSecret(inputVal)
			reduced, err := p.invokeStashReducer(
				ctx, target, token, oldInputPlain, oldOutputPlain, newInputPlain)
			if err != nil {
				return plugin.CheckResponse{}, err
			}
			if oldInputSecret || oldOutputSecret || newInputSecret {
				reduced = resource.MakeSecret(reduced)
			}
			out[stashReducedKey] = reduced
		}
	}
	// No reducer: don't set __reduced. Diff, Create, and Update fall back to legacy behavior
	// (diff on input change, output = input on create, output preserved on update).

	return plugin.CheckResponse{Properties: out}, nil
}

// unwrapSecret strips secret wrappers from v at every nesting level, returning the plaintext
// value along with a boolean indicating whether any secret was found. Objects and arrays are
// walked recursively so a secret buried inside a nested field is unwrapped too.
func unwrapSecret(v resource.PropertyValue) (resource.PropertyValue, bool) {
	if v.IsSecret() {
		inner, _ := unwrapSecret(v.SecretValue().Element)
		return inner, true
	}
	if v.IsObject() {
		obj := v.ObjectValue()
		out := make(resource.PropertyMap, len(obj))
		anySecret := false
		for k, elem := range obj {
			plain, hadSecret := unwrapSecret(elem)
			out[k] = plain
			anySecret = anySecret || hadSecret
		}
		return resource.NewProperty(out), anySecret
	}
	if v.IsArray() {
		arr := v.ArrayValue()
		out := make([]resource.PropertyValue, len(arr))
		anySecret := false
		for i, elem := range arr {
			plain, hadSecret := unwrapSecret(elem)
			out[i] = plain
			anySecret = anySecret || hadSecret
		}
		return resource.NewProperty(out), anySecret
	}
	return v, false
}

func (p *builtinProvider) Check(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	typ := req.URN.Type()
	switch typ { //nolint:exhaustive
	case stackReferenceType:
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
	case stashType:
		return p.checkStash(ctx, req)
	default:
		return plugin.CheckResponse{}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}
}

func (p *builtinProvider) Diff(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	typ := req.URN.Type()
	switch typ { //nolint:exhaustive
	case stackReferenceType:
		if !req.NewInputs["name"].DeepEquals(req.OldInputs["name"]) {
			return plugin.DiffResult{
				Changes:     plugin.DiffSome,
				ReplaceKeys: []resource.PropertyKey{"name"},
			}, nil
		}

		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	case stashType:
		// Diff against the reduced value produced by Check (which for the no-reducer case
		// preserves the old output, so behavior matches the pre-reducer semantics of updating
		// on input change).
		reduced, hasReduced := req.NewInputs[stashReducedKey]
		if !hasReduced {
			// Should not happen; Check always produces __reduced. Fall back to the old input diff.
			if !req.NewInputs["input"].DeepEquals(req.OldInputs["input"]) {
				return plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"input"}}, nil
			}
			return plugin.DiffResult{Changes: plugin.DiffNone}, nil
		}
		if !reduced.DeepEquals(req.OldOutputs["output"]) {
			return plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"output"}}, nil
		}
		return plugin.DiffResult{Changes: plugin.DiffNone}, nil
	default:
		return plugin.DiffResult{}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}
}

func (p *builtinProvider) Create(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	typ := req.URN.Type()
	switch typ { //nolint:exhaustive
	case stackReferenceType:

		state, err := p.readStackReference(ctx, resource.FromResourcePropertyMap(req.Properties))
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
			Properties: resource.ToResourcePropertyMap(state),
			Status:     resource.StatusOK,
		}, nil
	case stashType:
		var id resource.ID
		if !req.Preview {
			// generate a new uuid
			uuid, err := uuid.NewV4()
			if err != nil {
				return plugin.CreateResponse{Status: resource.StatusOK}, err
			}
			id = resource.ID(uuid.String())
		}

		output := req.Properties["input"]
		if r, ok := req.Properties[stashReducedKey]; ok {
			output = r
		}
		return plugin.CreateResponse{
			ID: id,
			Properties: resource.PropertyMap{
				"input":  req.Properties["input"],
				"output": output,
			},
			Status: resource.StatusOK,
		}, nil
	default:
		return plugin.CreateResponse{}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}
}

func (p *builtinProvider) Update(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	typ := req.URN.Type()
	switch typ { //nolint:exhaustive
	case stashType:
		output := req.OldOutputs["output"]
		if r, ok := req.NewInputs[stashReducedKey]; ok {
			output = r
		}
		return plugin.UpdateResponse{
			Properties: resource.PropertyMap{
				"input":  req.NewInputs["input"],
				"output": output,
			},
			Status: resource.StatusOK,
		}, nil
	default:
		return plugin.UpdateResponse{}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}
}

func (p *builtinProvider) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	contract.Assertf(req.URN.Type() == stackReferenceType || req.URN.Type() == stashType,
		"expected resource type %v or %v, got %v", stackReferenceType, stashType, req.URN.Type())

	return plugin.DeleteResponse{Status: resource.StatusOK}, nil
}

func (p *builtinProvider) List(context.Context, plugin.ListRequest) (*plugin.ListStream, error) {
	return nil, errors.New("the builtin provider does not support List")
}

func (p *builtinProvider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	contract.Requiref(req.URN != "", "urn", "must not be empty")
	contract.Requiref(req.ID != "", "id", "must not be empty")

	typ := req.URN.Type()
	switch typ { //nolint:exhaustive
	case stackReferenceType:
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

		outputs, err := p.readStackReference(ctx, resource.FromResourcePropertyMap(req.State))
		if err != nil {
			return plugin.ReadResponse{Status: resource.StatusUnknown}, err
		}

		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{
				ID:      req.ID,
				Inputs:  req.Inputs,
				Outputs: resource.ToResourcePropertyMap(outputs),
			},
			Status: resource.StatusOK,
		}, nil

	case stashType:
		if len(req.Inputs) == 0 {
			return plugin.ReadResponse{Status: resource.StatusUnknown}, errors.New("stash can not be imported")
		}

		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{
				ID:      req.ID,
				Inputs:  req.Inputs,
				Outputs: req.State,
			},
			Status: resource.StatusOK,
		}, nil
	default:
		return plugin.ReadResponse{Status: resource.StatusUnknown}, fmt.Errorf("unrecognized resource type '%v'", typ)
	}
}

func (p *builtinProvider) Construct(context.Context, plugin.ConstructRequest) (plugin.ConstructResponse, error) {
	return plugin.ConstructResponse{}, errors.New("builtin resources may not be constructed")
}

const (
	readStackOutputs         = "pulumi:pulumi:readStackOutputs"
	readStackResourceOutputs = "pulumi:pulumi:readStackResourceOutputs"
	getResource              = "pulumi:pulumi:getResource"
)

func (p *builtinProvider) Invoke(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	var outs property.Map
	var err error
	switch req.Tok {
	case readStackOutputs:
		outs, err = p.readStackReference(ctx, req.Args)
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

func (p *builtinProvider) Call(context.Context, plugin.CallRequest) (plugin.CallResponse, error) {
	return plugin.CallResult{}, errors.New("the builtin provider does not implement call")
}

func (p *builtinProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	// return an error: this should not be called for the builtin provider
	return plugin.PluginInfo{}, errors.New("the builtin provider does not report plugin info")
}

func (p *builtinProvider) SignalCancellation(context.Context) error {
	p.cancel()
	return nil
}

func (p *builtinProvider) readStackReference(
	ctx context.Context, inputs property.Map,
) (property.Map, error) {
	tracer := otel.Tracer("pulumi-cli")
	ctx, span := cmdutil.StartSpan(ctx, tracer, "builtinProvider.readStackReference")
	defer span.End()

	name, ok := inputs.GetOk("name")
	contract.Assertf(ok, "missing required property 'name'")
	contract.Assertf(name.IsString(), "expected 'name' to be a string")

	if p.backendClient == nil {
		return property.Map{}, errors.New("no backend client is available")
	}

	// If we fail to decrypt secrets when fetching the stack's outputs, we'll catch the error and log diagnostics rather
	// than failing outright.
	var decryptionErr error
	outputs, err := p.backendClient.GetStackOutputs(
		ctx,
		name.AsString(),
		func(e error) error { decryptionErr = e; return nil },
	)
	if err != nil {
		return property.Map{}, err
	}

	if decryptionErr != nil {
		p.diag.Infof(diag.Message("", "eliding undecryptable secrets for stack reference '%s'"), name.AsString())
		p.diag.Debugf(diag.Message("", "stack reference '%s' decryption error: %v"), name.AsString(), decryptionErr)
	}

	secretOutputs := make([]property.Value, 0)
	for k, v := range outputs.All {
		if v.HasSecrets() {
			secretOutputs = append(secretOutputs, property.New(k))
		}
	}

	// Sort the secret outputs so the order is deterministic, to avoid spurious diffs during updates.
	sort.Slice(secretOutputs, func(i, j int) bool {
		return secretOutputs[i].AsString() < secretOutputs[j].AsString()
	})

	return property.NewMap(map[string]property.Value{
		"name":              name,
		"outputs":           property.New(outputs),
		"secretOutputNames": property.New(secretOutputs),
	}), nil
}

func (p *builtinProvider) readStackResourceOutputs(inputs property.Map) (property.Map, error) {
	name, ok := inputs.GetOk("stackName")
	contract.Assertf(ok, "missing required property 'stackName'")
	contract.Assertf(name.IsString(), "expected 'stackName' to be a string")

	if p.backendClient == nil {
		return property.Map{}, errors.New("no backend client is available")
	}

	outputs, err := p.backendClient.GetStackResourceOutputs(p.context, name.AsString())
	if err != nil {
		return property.Map{}, err
	}

	return property.NewMap(map[string]property.Value{
		"name":    name,
		"outputs": property.New(outputs),
	}), nil
}

func (p *builtinProvider) getResource(inputs property.Map) (property.Map, error) {
	urnInput, ok := inputs.GetOk("urn")
	contract.Assertf(ok, "missing required property 'urn'")
	contract.Assertf(urnInput.IsString(), "expected 'urn' to be a string")

	// When looking up a resource to hydrate it, we'll first check for new states produced by resource registrations. If
	// we fail to find a match there, we'll look for states that have been read.
	urn := resource.URN(urnInput.AsString())
	state, ok := p.news.Load(urn)
	if !ok {
		state, ok = p.reads.Load(urn)
		if !ok {
			return property.Map{}, fmt.Errorf("unknown resource %v", urnInput.AsString())
		}
	}

	// Take the state lock so we can safely read the Outputs.
	state.Lock.Lock()
	defer state.Lock.Unlock()

	return property.NewMap(map[string]property.Value{
		"urn":      urnInput,
		"id":       property.New(string(state.ID)),
		"provider": property.New(state.Provider),
		"state":    property.New(resource.FromResourcePropertyMap(state.Outputs)),
	}), nil
}
