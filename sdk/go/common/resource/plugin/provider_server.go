// Copyright 2016-2020, Pulumi Corporation.
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
	"fmt"

	pbempty "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

type providerServer struct {
	provider      Provider
	keepSecrets   bool
	keepResources bool
}

func NewProviderServer(provider Provider) pulumirpc.ResourceProviderServer {
	return &providerServer{provider: provider}
}

func (p *providerServer) unmarshalOptions(label string) MarshalOptions {
	return MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	}
}

func (p *providerServer) marshalOptions(label string) MarshalOptions {
	return MarshalOptions{
		Label:         label,
		KeepUnknowns:  true,
		KeepSecrets:   p.keepSecrets,
		KeepResources: p.keepResources,
	}
}

func (p *providerServer) checkNYI(method string, err error) error {
	if err == ErrNotYetImplemented {
		return status.Error(codes.Unimplemented, fmt.Sprintf("%v is not yet implemented", method))
	}
	return err
}

func (p *providerServer) marshalDiff(diff DiffResult) (*pulumirpc.DiffResponse, error) {
	changes := pulumirpc.DiffResponse_DIFF_UNKNOWN
	switch diff.Changes {
	case DiffNone:
		changes = pulumirpc.DiffResponse_DIFF_NONE
	case DiffSome:
		changes = pulumirpc.DiffResponse_DIFF_SOME
	}

	// Infer the result from the detailed diff.
	var diffs, replaces []string
	var detailedDiff map[string]*pulumirpc.PropertyDiff
	if len(diff.DetailedDiff) == 0 {
		diffs = make([]string, len(diff.ChangedKeys))
		for i, k := range diff.ChangedKeys {
			diffs[i] = string(k)
		}
		replaces = make([]string, len(diff.ReplaceKeys))
		for i, k := range diff.ReplaceKeys {
			replaces[i] = string(k)
		}
	} else {
		changes = pulumirpc.DiffResponse_DIFF_SOME

		detailedDiff = make(map[string]*pulumirpc.PropertyDiff)
		for path, diff := range diff.DetailedDiff {
			diffs = append(diffs, path)

			var kind pulumirpc.PropertyDiff_Kind
			switch diff.Kind {
			case DiffAdd:
				kind = pulumirpc.PropertyDiff_ADD
			case DiffAddReplace:
				kind, replaces = pulumirpc.PropertyDiff_ADD_REPLACE, append(replaces, path)
			case DiffDelete:
				kind = pulumirpc.PropertyDiff_DELETE
			case DiffDeleteReplace:
				kind, replaces = pulumirpc.PropertyDiff_DELETE, append(replaces, path)
			case DiffUpdate:
				kind = pulumirpc.PropertyDiff_UPDATE
			case DiffUpdateReplace:
				kind, replaces = pulumirpc.PropertyDiff_UPDATE_REPLACE, append(replaces, path)
			}

			detailedDiff[path] = &pulumirpc.PropertyDiff{
				Kind:      kind,
				InputDiff: diff.InputDiff,
			}
		}
	}

	return &pulumirpc.DiffResponse{
		Replaces:            replaces,
		DeleteBeforeReplace: diff.DeleteBeforeReplace,
		Changes:             changes,
		Diffs:               diffs,
		DetailedDiff:        detailedDiff,
	}, nil

}

func (p *providerServer) GetSchema(ctx context.Context,
	req *pulumirpc.GetSchemaRequest) (*pulumirpc.GetSchemaResponse, error) {

	schema, err := p.provider.GetSchema(int(req.GetVersion()))
	if err != nil {
		return nil, err
	}
	return &pulumirpc.GetSchemaResponse{Schema: string(schema)}, nil
}

func (p *providerServer) GetPluginInfo(ctx context.Context, req *pbempty.Empty) (*pulumirpc.PluginInfo, error) {
	info, err := p.provider.GetPluginInfo()
	if err != nil {
		return nil, err
	}
	return &pulumirpc.PluginInfo{Version: info.Version.String()}, nil
}

func (p *providerServer) Cancel(ctx context.Context, req *pbempty.Empty) (*pbempty.Empty, error) {
	if err := p.provider.SignalCancellation(); err != nil {
		return nil, err
	}
	return &pbempty.Empty{}, nil
}

func (p *providerServer) CheckConfig(ctx context.Context,
	req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {

	urn := resource.URN(req.GetUrn())

	state, err := UnmarshalProperties(req.GetOlds(), p.unmarshalOptions("olds"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetNews(), p.unmarshalOptions("news"))
	if err != nil {
		return nil, err
	}

	newInputs, failures, err := p.provider.CheckConfig(urn, state, inputs, true)
	if err != nil {
		return nil, p.checkNYI("CheckConfig", err)
	}

	rpcInputs, err := MarshalProperties(newInputs, p.marshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	rpcFailures := make([]*pulumirpc.CheckFailure, len(failures))
	for i, f := range failures {
		rpcFailures[i] = &pulumirpc.CheckFailure{Property: string(f.Property), Reason: f.Reason}
	}

	return &pulumirpc.CheckResponse{Inputs: rpcInputs, Failures: rpcFailures}, nil
}

func (p *providerServer) DiffConfig(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	urn := resource.URN(req.GetUrn())

	state, err := UnmarshalProperties(req.GetOlds(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetNews(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	diff, err := p.provider.DiffConfig(urn, state, inputs, true, req.GetIgnoreChanges())
	if err != nil {
		return nil, p.checkNYI("DiffConfig", err)
	}
	return p.marshalDiff(diff)
}

func (p *providerServer) Configure(ctx context.Context,
	req *pulumirpc.ConfigureRequest) (*pulumirpc.ConfigureResponse, error) {

	var inputs resource.PropertyMap
	if req.GetArgs() != nil {
		args, err := UnmarshalProperties(req.GetArgs(), p.unmarshalOptions("args"))
		if err != nil {
			return nil, err
		}
		inputs = args
	} else {
		for k, v := range req.GetVariables() {
			key, err := config.ParseKey(k)
			if err != nil {
				return nil, err
			}

			var value interface{}
			if err = json.Unmarshal([]byte(v), &value); err != nil {
				// If we couldn't unmarshal a JSON value, just pass the raw string through.
				value = v
			}

			inputs[resource.PropertyKey(key.Name())] = resource.NewPropertyValue(value)
		}
	}

	if err := p.provider.Configure(inputs); err != nil {
		return nil, err
	}

	p.keepSecrets = req.GetAcceptSecrets()
	p.keepResources = req.GetAcceptResources()
	return &pulumirpc.ConfigureResponse{AcceptSecrets: true, SupportsPreview: true, AcceptResources: true}, nil
}

func (p *providerServer) Check(ctx context.Context, req *pulumirpc.CheckRequest) (*pulumirpc.CheckResponse, error) {
	urn := resource.URN(req.GetUrn())

	state, err := UnmarshalProperties(req.GetOlds(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetNews(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	newInputs, failures, err := p.provider.Check(urn, state, inputs, true)
	if err != nil {
		return nil, err
	}

	rpcInputs, err := MarshalProperties(newInputs, p.marshalOptions("newInputs"))
	if err != nil {
		return nil, err
	}

	rpcFailures := make([]*pulumirpc.CheckFailure, len(failures))
	for i, f := range failures {
		rpcFailures[i] = &pulumirpc.CheckFailure{Property: string(f.Property), Reason: f.Reason}
	}

	return &pulumirpc.CheckResponse{Inputs: rpcInputs, Failures: rpcFailures}, nil
}

func (p *providerServer) Diff(ctx context.Context, req *pulumirpc.DiffRequest) (*pulumirpc.DiffResponse, error) {
	urn, id := resource.URN(req.GetUrn()), resource.ID(req.GetId())

	state, err := UnmarshalProperties(req.GetOlds(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetNews(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	diff, err := p.provider.Diff(urn, id, state, inputs, true, req.GetIgnoreChanges())
	if err != nil {
		return nil, err
	}
	return p.marshalDiff(diff)
}

func (p *providerServer) Create(ctx context.Context, req *pulumirpc.CreateRequest) (*pulumirpc.CreateResponse, error) {
	urn := resource.URN(req.GetUrn())

	inputs, err := UnmarshalProperties(req.GetProperties(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	id, state, _, err := p.provider.Create(urn, inputs, req.GetTimeout(), req.GetPreview())
	if err != nil {
		return nil, err
	}

	rpcState, err := MarshalProperties(state, p.marshalOptions("newState"))
	if err != nil {
		return nil, err
	}

	return &pulumirpc.CreateResponse{
		Id:         string(id),
		Properties: rpcState,
	}, nil
}

func (p *providerServer) Read(ctx context.Context, req *pulumirpc.ReadRequest) (*pulumirpc.ReadResponse, error) {
	urn, id := resource.URN(req.GetUrn()), resource.ID(req.GetId())

	state, err := UnmarshalProperties(req.GetProperties(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetInputs(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	result, _, err := p.provider.Read(urn, id, inputs, state)
	if err != nil {
		return nil, err
	}

	rpcState, err := MarshalProperties(result.Outputs, p.marshalOptions("newState"))
	if err != nil {
		return nil, err
	}

	rpcInputs, err := MarshalProperties(result.Inputs, p.marshalOptions("newInputs"))
	if err != nil {
		return nil, err
	}

	return &pulumirpc.ReadResponse{
		Id:         string(id),
		Properties: rpcState,
		Inputs:     rpcInputs,
	}, nil
}

func (p *providerServer) Update(ctx context.Context, req *pulumirpc.UpdateRequest) (*pulumirpc.UpdateResponse, error) {
	urn, id := resource.URN(req.GetUrn()), resource.ID(req.GetId())

	state, err := UnmarshalProperties(req.GetOlds(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	inputs, err := UnmarshalProperties(req.GetNews(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	newState, _, err := p.provider.Update(urn, id, state, inputs, req.GetTimeout(), req.GetIgnoreChanges(),
		req.GetPreview())
	if err != nil {
		return nil, err
	}

	rpcState, err := MarshalProperties(newState, p.marshalOptions("newState"))
	if err != nil {
		return nil, err
	}

	return &pulumirpc.UpdateResponse{Properties: rpcState}, nil
}

func (p *providerServer) Delete(ctx context.Context, req *pulumirpc.DeleteRequest) (*pbempty.Empty, error) {
	urn, id := resource.URN(req.GetUrn()), resource.ID(req.GetId())

	state, err := UnmarshalProperties(req.GetProperties(), p.unmarshalOptions("state"))
	if err != nil {
		return nil, err
	}

	if _, err = p.provider.Delete(urn, id, state, req.GetTimeout()); err != nil {
		return nil, err
	}

	return &pbempty.Empty{}, nil
}

func (p *providerServer) Construct(ctx context.Context,
	req *pulumirpc.ConstructRequest) (*pulumirpc.ConstructResponse, error) {

	typ, name, parent := tokens.Type(req.GetType()), tokens.QName(req.GetName()), resource.URN(req.GetParent())

	inputs, err := UnmarshalProperties(req.GetInputs(), p.unmarshalOptions("inputs"))
	if err != nil {
		return nil, err
	}

	cfg := map[config.Key]string{}
	for k, v := range req.GetConfig() {
		configKey, err := config.ParseKey(k)
		if err != nil {
			return nil, err
		}
		cfg[configKey] = v
	}

	cfgSecretKeys := []config.Key{}
	for _, k := range req.GetConfigSecretKeys() {
		key, err := config.ParseKey(k)
		if err != nil {
			return nil, err
		}
		cfgSecretKeys = append(cfgSecretKeys, key)
	}

	info := ConstructInfo{
		Project:          req.GetProject(),
		Stack:            req.GetStack(),
		Config:           cfg,
		ConfigSecretKeys: cfgSecretKeys,
		DryRun:           req.GetDryRun(),
		Parallel:         int(req.GetParallel()),
		MonitorAddress:   req.GetMonitorEndpoint(),
	}

	aliases := make([]resource.URN, len(req.GetAliases()))
	for i, urn := range req.GetAliases() {
		aliases[i] = resource.URN(urn)
	}
	dependencies := make([]resource.URN, len(req.GetDependencies()))
	for i, urn := range req.GetDependencies() {
		dependencies[i] = resource.URN(urn)
	}
	propertyDependencies := map[resource.PropertyKey][]resource.URN{}
	for name, deps := range req.GetInputDependencies() {
		urns := make([]resource.URN, len(deps.Urns))
		for i, urn := range deps.Urns {
			urns[i] = resource.URN(urn)
		}
		propertyDependencies[resource.PropertyKey(name)] = urns
	}
	options := ConstructOptions{
		Aliases:              aliases,
		Dependencies:         dependencies,
		Protect:              req.GetProtect(),
		Providers:            req.GetProviders(),
		PropertyDependencies: propertyDependencies,
	}

	result, err := p.provider.Construct(info, typ, name, parent, inputs, options)
	if err != nil {
		return nil, err
	}

	outputs, err := MarshalProperties(result.Outputs, p.marshalOptions("outputs"))
	if err != nil {
		return nil, err
	}

	outputDependencies := map[string]*pulumirpc.ConstructResponse_PropertyDependencies{}
	for name, deps := range result.OutputDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		outputDependencies[string(name)] = &pulumirpc.ConstructResponse_PropertyDependencies{Urns: urns}
	}

	return &pulumirpc.ConstructResponse{
		Urn:               string(result.URN),
		State:             outputs,
		StateDependencies: outputDependencies,
	}, nil
}

func (p *providerServer) Invoke(ctx context.Context, req *pulumirpc.InvokeRequest) (*pulumirpc.InvokeResponse, error) {
	args, err := UnmarshalProperties(req.GetArgs(), p.unmarshalOptions("args"))
	if err != nil {
		return nil, err
	}

	result, failures, err := p.provider.Invoke(tokens.ModuleMember(req.GetTok()), args)
	if err != nil {
		return nil, err
	}

	rpcResult, err := MarshalProperties(result, p.marshalOptions("result"))
	if err != nil {
		return nil, err
	}

	rpcFailures := make([]*pulumirpc.CheckFailure, len(failures))
	for i, f := range failures {
		rpcFailures[i] = &pulumirpc.CheckFailure{Property: string(f.Property), Reason: f.Reason}
	}

	return &pulumirpc.InvokeResponse{
		Return:   rpcResult,
		Failures: rpcFailures,
	}, nil
}

func (p *providerServer) StreamInvoke(req *pulumirpc.InvokeRequest,
	server pulumirpc.ResourceProvider_StreamInvokeServer) error {

	args, err := UnmarshalProperties(req.GetArgs(), p.unmarshalOptions("args"))
	if err != nil {
		return err
	}

	failures, err := p.provider.StreamInvoke(tokens.ModuleMember(req.GetTok()), args,
		func(item resource.PropertyMap) error {
			rpcItem, err := MarshalProperties(item, p.marshalOptions("item"))
			if err != nil {
				return err
			}

			return server.Send(&pulumirpc.InvokeResponse{Return: rpcItem})
		})
	if err != nil {
		return err
	}
	if len(failures) == 0 {
		return nil
	}

	rpcFailures := make([]*pulumirpc.CheckFailure, len(failures))
	for i, f := range failures {
		rpcFailures[i] = &pulumirpc.CheckFailure{Property: string(f.Property), Reason: f.Reason}
	}

	return server.Send(&pulumirpc.InvokeResponse{Failures: rpcFailures})
}

func (p *providerServer) Call(ctx context.Context, req *pulumirpc.CallRequest) (*pulumirpc.CallResponse, error) {
	args, err := UnmarshalProperties(req.GetArgs(), p.unmarshalOptions("args"))
	if err != nil {
		return nil, err
	}

	cfg := map[config.Key]string{}
	for k, v := range req.GetConfig() {
		configKey, err := config.ParseKey(k)
		if err != nil {
			return nil, err
		}
		cfg[configKey] = v
	}
	info := CallInfo{
		Project:        req.GetProject(),
		Stack:          req.GetStack(),
		Config:         cfg,
		DryRun:         req.GetDryRun(),
		Parallel:       int(req.GetParallel()),
		MonitorAddress: req.GetMonitorEndpoint(),
	}
	argDependencies := map[resource.PropertyKey][]resource.URN{}
	for name, deps := range req.GetArgDependencies() {
		urns := make([]resource.URN, len(deps.Urns))
		for i, urn := range deps.Urns {
			urns[i] = resource.URN(urn)
		}
		argDependencies[resource.PropertyKey(name)] = urns
	}
	options := CallOptions{
		ArgDependencies: argDependencies,
	}

	result, err := p.provider.Call(tokens.ModuleMember(req.GetTok()), args, info, options)
	if err != nil {
		return nil, err
	}

	rpcResult, err := MarshalProperties(result.Return, MarshalOptions{
		Label:         "result",
		KeepUnknowns:  true,
		KeepSecrets:   true,
		KeepResources: true,
	})
	if err != nil {
		return nil, err
	}

	returnDependencies := map[string]*pulumirpc.CallResponse_ReturnDependencies{}
	for name, deps := range result.ReturnDependencies {
		urns := make([]string, len(deps))
		for i, urn := range deps {
			urns[i] = string(urn)
		}
		returnDependencies[string(name)] = &pulumirpc.CallResponse_ReturnDependencies{Urns: urns}
	}

	rpcFailures := make([]*pulumirpc.CheckFailure, len(result.Failures))
	for i, f := range result.Failures {
		rpcFailures[i] = &pulumirpc.CheckFailure{Property: string(f.Property), Reason: f.Reason}
	}

	return &pulumirpc.CallResponse{
		Return:             rpcResult,
		ReturnDependencies: returnDependencies,
		Failures:           rpcFailures,
	}, nil
}
