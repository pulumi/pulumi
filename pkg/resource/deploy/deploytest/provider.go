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

package deploytest

import (
	"context"
	"errors"

	"github.com/blang/semver"
	uuid "github.com/gofrs/uuid"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type Provider struct {
	plugin.NotForwardCompatibleProvider

	Name    string
	Package tokens.Package
	Version semver.Version

	Config     resource.PropertyMap
	configured bool

	DialMonitorF func(ctx context.Context, endpoint string) (*ResourceMonitor, error)

	ParameterizeF func(ctx context.Context, request plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error)

	GetSchemaF func(request plugin.GetSchemaRequest) ([]byte, error)

	CheckConfigF func(urn resource.URN, olds,
		news resource.PropertyMap, allowUnknowns bool) (resource.PropertyMap, []plugin.CheckFailure, error)
	DiffConfigF func(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
		ignoreChanges []string) (plugin.DiffResult, error)
	ConfigureF func(news resource.PropertyMap) error

	CheckF func(urn resource.URN,
		olds, news resource.PropertyMap, randomSeed []byte) (resource.PropertyMap, []plugin.CheckFailure, error)
	DiffF func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
		ignoreChanges []string) (plugin.DiffResult, error)
	CreateF func(urn resource.URN, inputs resource.PropertyMap, timeout float64,
		preview bool) (resource.ID, resource.PropertyMap, resource.Status, error)
	UpdateF func(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap, timeout float64,
		ignoreChanges []string, preview bool) (resource.PropertyMap, resource.Status, error)
	DeleteF func(urn resource.URN, id resource.ID,
		oldInputs, oldOutputs resource.PropertyMap, timeout float64) (resource.Status, error)
	ReadF func(urn resource.URN, id resource.ID,
		inputs, state resource.PropertyMap) (plugin.ReadResult, resource.Status, error)

	ConstructF func(monitor *ResourceMonitor, typ, name string, parent resource.URN, inputs resource.PropertyMap,
		info plugin.ConstructInfo, options plugin.ConstructOptions) (plugin.ConstructResult, error)

	InvokeF func(tok tokens.ModuleMember,
		inputs resource.PropertyMap) (resource.PropertyMap, []plugin.CheckFailure, error)
	StreamInvokeF func(tok tokens.ModuleMember, args resource.PropertyMap,
		onNext func(resource.PropertyMap) error) ([]plugin.CheckFailure, error)

	CallF func(monitor *ResourceMonitor, tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
		options plugin.CallOptions) (plugin.CallResult, error)

	CancelF func() error

	GetMappingF  func(key, provider string) ([]byte, string, error)
	GetMappingsF func(key string) ([]string, error)
}

func (prov *Provider) SignalCancellation(context.Context) error {
	if prov.CancelF == nil {
		return nil
	}
	return prov.CancelF()
}

func (prov *Provider) Close() error {
	return nil
}

func (prov *Provider) Pkg() tokens.Package {
	return prov.Package
}

func (prov *Provider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	return workspace.PluginInfo{
		Name:    prov.Name,
		Version: &prov.Version,
	}, nil
}

func (prov *Provider) Parameterize(
	ctx context.Context, params plugin.ParameterizeRequest,
) (plugin.ParameterizeResponse, error) {
	if prov.ParameterizeF == nil {
		return plugin.ParameterizeResponse{}, errors.New("no parameters")
	}
	return prov.ParameterizeF(ctx, params)
}

func (prov *Provider) GetSchema(_ context.Context, request plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
	if prov.GetSchemaF == nil {
		return plugin.GetSchemaResponse{Schema: []byte("{}")}, nil
	}
	bytes, err := prov.GetSchemaF(request)
	return plugin.GetSchemaResponse{Schema: bytes}, err
}

func (prov *Provider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	if prov.CheckConfigF == nil {
		return plugin.CheckConfigResponse{Properties: req.News}, nil
	}
	props, failures, err := prov.CheckConfigF(req.URN, req.Olds, req.News, req.AllowUnknowns)
	return plugin.CheckConfigResponse{
		Properties: props,
		Failures:   failures,
	}, err
}

func (prov *Provider) DiffConfig(_ context.Context, req plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	if prov.DiffConfigF == nil {
		return plugin.DiffResult{}, nil
	}
	return prov.DiffConfigF(req.URN, req.OldInputs, req.OldOutputs, req.NewInputs, req.IgnoreChanges)
}

func (prov *Provider) Configure(_ context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	contract.Assertf(!prov.configured, "provider %v was already configured", prov.Name)
	prov.configured = true

	if prov.ConfigureF == nil {
		prov.Config = req.Inputs
		return plugin.ConfigureResponse{}, nil
	}
	return plugin.ConfigureResponse{}, prov.ConfigureF(req.Inputs)
}

func (prov *Provider) Check(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	contract.Requiref(req.RandomSeed != nil, "randomSeed", "must not be nil")
	if prov.CheckF == nil {
		return plugin.CheckResponse{Properties: req.News}, nil
	}
	props, failures, err := prov.CheckF(req.URN, req.Olds, req.News, req.RandomSeed)
	return plugin.CheckResponse{Properties: props, Failures: failures}, err
}

func (prov *Provider) Create(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
	if prov.CreateF == nil {
		// generate a new uuid
		uuid, err := uuid.NewV4()
		if err != nil {
			return plugin.CreateResponse{}, err
		}
		return plugin.CreateResponse{
			ID:         resource.ID(uuid.String()),
			Properties: resource.PropertyMap{},
		}, nil
	}
	id, properties, status, err := prov.CreateF(req.URN, req.Properties, req.Timeout, req.Preview)
	return plugin.CreateResponse{
		ID:         id,
		Properties: properties,
		Status:     status,
	}, err
}

func (prov *Provider) Diff(_ context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	if prov.DiffF == nil {
		return plugin.DiffResponse{}, nil
	}
	return prov.DiffF(req.URN, req.ID, req.OldInputs, req.OldOutputs, req.NewInputs, req.IgnoreChanges)
}

func (prov *Provider) Update(_ context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	if prov.UpdateF == nil {
		return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
	}
	properties, status, err := prov.UpdateF(req.URN, req.ID,
		req.OldInputs, req.OldOutputs,
		req.NewInputs,
		req.Timeout, req.IgnoreChanges, req.Preview)
	return plugin.UpdateResponse{Properties: properties, Status: status}, err
}

func (prov *Provider) Delete(_ context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	if prov.DeleteF == nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}
	status, err := prov.DeleteF(req.URN, req.ID, req.Inputs, req.Outputs, req.Timeout)
	return plugin.DeleteResponse{Status: status}, err
}

func (prov *Provider) Read(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	contract.Assertf(req.URN != "", "Read URN was empty")
	contract.Assertf(req.ID != "", "Read ID was empty")
	if prov.ReadF == nil {
		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{
				Outputs: resource.PropertyMap{},
				Inputs:  resource.PropertyMap{},
			},
			Status: resource.StatusUnknown,
		}, nil
	}

	result, status, err := prov.ReadF(req.URN, req.ID, req.Inputs, req.State)
	return plugin.ReadResponse{
		ReadResult: result,
		Status:     status,
	}, err
}

func (prov *Provider) Construct(ctx context.Context, req plugin.ConstructRequest) (plugin.ConstructResult, error) {
	if prov.ConstructF == nil {
		return plugin.ConstructResult{}, nil
	}
	dialMonitorImpl := dialMonitor
	if prov.DialMonitorF != nil {
		dialMonitorImpl = prov.DialMonitorF
	}
	monitor, err := dialMonitorImpl(ctx, req.Info.MonitorAddress)
	if err != nil {
		return plugin.ConstructResult{}, err
	}
	return prov.ConstructF(monitor, string(req.Type), req.Name, req.Parent, req.Inputs, req.Info, req.Options)
}

func (prov *Provider) Invoke(_ context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	if prov.InvokeF == nil {
		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{},
		}, nil
	}
	result, failures, err := prov.InvokeF(req.Tok, req.Args)
	return plugin.InvokeResponse{Properties: result, Failures: failures}, err
}

func (prov *Provider) StreamInvoke(
	_ context.Context, req plugin.StreamInvokeRequest,
) (plugin.StreamInvokeResponse, error) {
	if prov.StreamInvokeF == nil {
		return plugin.StreamInvokeResponse{}, errors.New("StreamInvoke unimplemented")
	}
	failures, err := prov.StreamInvokeF(req.Tok, req.Args, req.OnNext)
	return plugin.StreamInvokeResponse{Failures: failures}, err
}

func (prov *Provider) Call(ctx context.Context, req plugin.CallRequest) (plugin.CallResponse, error) {
	if prov.CallF == nil {
		return plugin.CallResult{}, nil
	}
	dialMonitorImpl := dialMonitor
	if prov.DialMonitorF != nil {
		dialMonitorImpl = prov.DialMonitorF
	}
	monitor, err := dialMonitorImpl(ctx, req.Info.MonitorAddress)
	if err != nil {
		return plugin.CallResult{}, err
	}
	return prov.CallF(monitor, req.Tok, req.Args, req.Info, req.Options)
}

func (prov *Provider) GetMapping(_ context.Context, req plugin.GetMappingRequest) (plugin.GetMappingResponse, error) {
	if prov.GetMappingF == nil {
		return plugin.GetMappingResponse{}, nil
	}
	data, provider, err := prov.GetMappingF(req.Key, req.Provider)
	return plugin.GetMappingResponse{
		Data:     data,
		Provider: provider,
	}, err
}

func (prov *Provider) GetMappings(
	_ context.Context, req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	if prov.GetMappingsF == nil {
		return plugin.GetMappingsResponse{}, nil
	}
	keys, err := prov.GetMappingsF(req.Key)
	return plugin.GetMappingsResponse{Keys: keys}, err
}
