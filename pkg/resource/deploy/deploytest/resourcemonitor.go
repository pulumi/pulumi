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

	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/plugin"
	"github.com/pulumi/pulumi/pkg/tokens"
	pulumirpc "github.com/pulumi/pulumi/sdk/proto/go"
)

type ResourceMonitor struct {
	resmon pulumirpc.ResourceMonitorClient
}

func (rm *ResourceMonitor) RegisterResource(t tokens.Type, name string, custom bool, parent resource.URN, protect bool,
	dependencies []resource.URN, provider string, inputs resource.PropertyMap,
	propertyDeps map[resource.PropertyKey][]resource.URN, deleteBeforeReplace bool,
	version string, ignoreChanges []string,
	aliases []resource.URN, importID resource.ID, customTimeouts *resource.CustomTimeouts) (resource.URN, resource.ID, resource.PropertyMap, error) {

	// marshal inputs
	ins, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return "", "", nil, err
	}

	// marshal dependencies
	deps := []string{}
	for _, d := range dependencies {
		deps = append(deps, string(d))
	}

	// marshal aliases
	aliasStrings := []string{}
	for _, a := range aliases {
		aliasStrings = append(aliasStrings, string(a))
	}

	inputDeps := make(map[string]*pulumirpc.RegisterResourceRequest_PropertyDependencies)
	for pk, pd := range propertyDeps {
		pdeps := []string{}
		for _, d := range pd {
			pdeps = append(pdeps, string(d))
		}
		inputDeps[string(pk)] = &pulumirpc.RegisterResourceRequest_PropertyDependencies{
			Urns: pdeps,
		}
	}

	var timeouts pulumirpc.RegisterResourceRequest_CustomTimeouts
	if customTimeouts != nil {
		timeouts.Create = customTimeouts.Create
		timeouts.Update = customTimeouts.Update
		timeouts.Delete = customTimeouts.Delete
	}

	// submit request
	resp, err := rm.resmon.RegisterResource(context.Background(), &pulumirpc.RegisterResourceRequest{
		Type:                 string(t),
		Name:                 name,
		Custom:               custom,
		Parent:               string(parent),
		Protect:              protect,
		Dependencies:         deps,
		Provider:             provider,
		Object:               ins,
		PropertyDependencies: inputDeps,
		DeleteBeforeReplace:  deleteBeforeReplace,
		IgnoreChanges:        ignoreChanges,
		Version:              version,
		Aliases:              aliasStrings,
		ImportId:             string(importID),
		CustomTimeouts:		  &timeouts,
	})
	if err != nil {
		return "", "", nil, err
	}

	// unmarshal outputs
	outs, err := plugin.UnmarshalProperties(resp.Object, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return "", "", nil, err
	}

	return resource.URN(resp.Urn), resource.ID(resp.Id), outs, nil
}

func (rm *ResourceMonitor) ReadResource(t tokens.Type, name string, id resource.ID, parent resource.URN,
	inputs resource.PropertyMap, provider string, version string) (resource.URN, resource.PropertyMap, error) {

	// marshal inputs
	ins, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return "", nil, err
	}

	// submit request
	resp, err := rm.resmon.ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
		Type:       string(t),
		Name:       name,
		Id:         string(id),
		Parent:     string(parent),
		Provider:   provider,
		Properties: ins,
		Version:    version,
	})
	if err != nil {
		return "", nil, err
	}

	// unmarshal outputs
	outs, err := plugin.UnmarshalProperties(resp.Properties, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return "", nil, err
	}

	return resource.URN(resp.Urn), outs, nil
}

func (rm *ResourceMonitor) Invoke(tok tokens.ModuleMember, inputs resource.PropertyMap,
	provider string, version string) (resource.PropertyMap, []*pulumirpc.CheckFailure, error) {

	// marshal inputs
	ins, err := plugin.MarshalProperties(inputs, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	// submit request
	resp, err := rm.resmon.Invoke(context.Background(), &pulumirpc.InvokeRequest{
		Tok:      string(tok),
		Provider: provider,
		Args:     ins,
		Version:  version,
	})
	if err != nil {
		return nil, nil, err
	}

	// handle failures
	if len(resp.Failures) != 0 {
		return nil, resp.Failures, nil
	}

	// unmarshal outputs
	outs, err := plugin.UnmarshalProperties(resp.Return, plugin.MarshalOptions{KeepUnknowns: true})
	if err != nil {
		return nil, nil, err
	}

	return outs, nil, nil
}
