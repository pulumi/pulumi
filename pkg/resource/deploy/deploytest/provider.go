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
	CancelF      func() error

	HandshakeF    func(context.Context, plugin.ProviderHandshakeRequest) (*plugin.ProviderHandshakeResponse, error)
	ParameterizeF func(context.Context, plugin.ParameterizeRequest) (plugin.ParameterizeResponse, error)
	GetSchemaF    func(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error)
	CheckConfigF  func(context.Context, plugin.CheckConfigRequest) (plugin.CheckConfigResponse, error)
	DiffConfigF   func(context.Context, plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error)
	ConfigureF    func(context.Context, plugin.ConfigureRequest) (plugin.ConfigureResponse, error)
	CheckF        func(context.Context, plugin.CheckRequest) (plugin.CheckResponse, error)
	DiffF         func(context.Context, plugin.DiffRequest) (plugin.DiffResult, error)
	CreateF       func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error)
	UpdateF       func(context.Context, plugin.UpdateRequest) (plugin.UpdateResponse, error)
	DeleteF       func(context.Context, plugin.DeleteRequest) (plugin.DeleteResponse, error)
	ReadF         func(context.Context, plugin.ReadRequest) (plugin.ReadResponse, error)
	ConstructF    func(context.Context, plugin.ConstructRequest, *ResourceMonitor) (plugin.ConstructResponse, error)
	InvokeF       func(context.Context, plugin.InvokeRequest) (plugin.InvokeResponse, error)
	CallF         func(context.Context, plugin.CallRequest, *ResourceMonitor) (plugin.CallResponse, error)
	GetMappingF   func(context.Context, plugin.GetMappingRequest) (plugin.GetMappingResponse, error)
	GetMappingsF  func(context.Context, plugin.GetMappingsRequest) (plugin.GetMappingsResponse, error)
}

func (prov *Provider) Handshake(
	ctx context.Context, req plugin.ProviderHandshakeRequest,
) (*plugin.ProviderHandshakeResponse, error) {
	if prov.HandshakeF == nil {
		return &plugin.ProviderHandshakeResponse{}, nil
	}
	return prov.HandshakeF(ctx, req)
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

func (prov *Provider) GetSchema(
	ctx context.Context,
	request plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	if prov.GetSchemaF == nil {
		return plugin.GetSchemaResponse{Schema: []byte("{}")}, nil
	}
	return prov.GetSchemaF(ctx, request)
}

func (prov *Provider) CheckConfig(
	ctx context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	if prov.CheckConfigF == nil {
		return plugin.CheckConfigResponse{Properties: req.News}, nil
	}
	return prov.CheckConfigF(ctx, req)
}

func (prov *Provider) DiffConfig(ctx context.Context, req plugin.DiffConfigRequest) (plugin.DiffConfigResponse, error) {
	if prov.DiffConfigF == nil {
		return plugin.DiffResult{}, nil
	}
	return prov.DiffConfigF(ctx, req)
}

func (prov *Provider) Configure(ctx context.Context, req plugin.ConfigureRequest) (plugin.ConfigureResponse, error) {
	contract.Assertf(!prov.configured, "provider %v was already configured", prov.Name)
	prov.configured = true

	if prov.ConfigureF == nil {
		prov.Config = req.Inputs
		return plugin.ConfigureResponse{}, nil
	}
	return prov.ConfigureF(ctx, req)
}

func (prov *Provider) Check(ctx context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
	contract.Requiref(req.RandomSeed != nil, "randomSeed", "must not be nil")
	if prov.CheckF == nil {
		return plugin.CheckResponse{Properties: req.News}, nil
	}
	return prov.CheckF(ctx, req)
}

func (prov *Provider) Create(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
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
	return prov.CreateF(ctx, req)
}

func (prov *Provider) Diff(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
	if prov.DiffF == nil {
		return plugin.DiffResponse{}, nil
	}
	return prov.DiffF(ctx, req)
}

func (prov *Provider) Update(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
	if prov.UpdateF == nil {
		return plugin.UpdateResponse{Properties: req.NewInputs, Status: resource.StatusOK}, nil
	}
	return prov.UpdateF(ctx, req)
}

func (prov *Provider) Delete(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
	if prov.DeleteF == nil {
		return plugin.DeleteResponse{Status: resource.StatusOK}, nil
	}
	return prov.DeleteF(ctx, req)
}

func (prov *Provider) Read(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
	contract.Assertf(req.URN != "", "Read URN was empty")
	contract.Assertf(req.ID != "", "Read ID was empty")
	if prov.ReadF == nil {
		state := req.State
		if state == nil {
			state = resource.PropertyMap{}
		}
		inputs := req.Inputs
		if inputs == nil {
			inputs = resource.PropertyMap{}
		}

		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{
				ID:      req.ID,
				Outputs: state,
				Inputs:  inputs,
			},
			Status: resource.StatusUnknown,
		}, nil
	}

	return prov.ReadF(ctx, req)
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
	return prov.ConstructF(ctx, req, monitor)
}

func (prov *Provider) Invoke(ctx context.Context, req plugin.InvokeRequest) (plugin.InvokeResponse, error) {
	if prov.InvokeF == nil {
		return plugin.InvokeResponse{
			Properties: resource.PropertyMap{},
		}, nil
	}
	return prov.InvokeF(ctx, req)
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
	return prov.CallF(ctx, req, monitor)
}

func (prov *Provider) GetMapping(
	ctx context.Context,
	req plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	if prov.GetMappingF == nil {
		return plugin.GetMappingResponse{}, nil
	}
	return prov.GetMappingF(ctx, req)
}

func (prov *Provider) GetMappings(
	ctx context.Context,
	req plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	if prov.GetMappingsF == nil {
		return plugin.GetMappingsResponse{}, nil
	}
	return prov.GetMappingsF(ctx, req)
}
