// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type badProvider struct{}

var _ plugin.Provider = (*badProvider)(nil)

func (p *badProvider) Close() error {
	return nil
}

func (p *badProvider) Pkg() tokens.Package {
	return "bad"
}

func (p *badProvider) GetSchema(version int) ([]byte, error) {
	// The whole point of this provider is to return an invalid schema, so just make up a type for the
	// property value.
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "not a type",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "simple",
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			"simple:index:Resource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: resourceProperties,
					Required:   resourceRequired,
				},
				InputProperties: resourceProperties,
				RequiredInputs:  resourceRequired,
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}

	return jsonBytes, nil
}

func (p *badProvider) CheckConfig(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
	allowUnknowns bool,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	// Expect just the version
	version, ok := newInputs["version"]
	if !ok {
		return nil, makeCheckFailure("version", "missing version"), nil
	}
	if !version.IsString() {
		return nil, makeCheckFailure("version", "version is not a string"), nil
	}
	if version.StringValue() != "1.0.0" {
		return nil, makeCheckFailure("version", "version is not 1.0.0"), nil
	}

	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *badProvider) DiffConfig(urn resource.URN, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, fmt.Errorf("DiffConfig not implemented")
}

func (p *badProvider) Configure(inputs resource.PropertyMap) error {
	return nil
}

func (p *badProvider) Check(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, fmt.Errorf("Check not implemented")
}

func (p *badProvider) Diff(urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, fmt.Errorf("Diff not implemented")
}

func (p *badProvider) Create(
	urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	return "", nil, resource.StatusOK, fmt.Errorf("Create not implemented")
}

func (p *badProvider) Read(urn resource.URN, id resource.ID,
	inputs, state resource.PropertyMap,
) (plugin.ReadResult, resource.Status, error) {
	return plugin.ReadResult{}, resource.StatusOK, fmt.Errorf("Read not implemented")
}

func (p *badProvider) Update(urn resource.URN, id resource.ID,
	oldInputs, oldOutputs, newInputs resource.PropertyMap, timeout float64,
	ignoreChanges []string, preview bool,
) (resource.PropertyMap, resource.Status, error) {
	return nil, resource.StatusOK, fmt.Errorf("Update not implemented")
}

func (p *badProvider) Delete(urn resource.URN, id resource.ID, props resource.PropertyMap,
	timeout float64,
) (resource.Status, error) {
	return resource.StatusOK, fmt.Errorf("Delete not implemented")
}

func (p *badProvider) Construct(info plugin.ConstructInfo, typ tokens.Type, name tokens.QName, parent resource.URN,
	inputs resource.PropertyMap, options plugin.ConstructOptions,
) (plugin.ConstructResult, error) {
	return plugin.ConstructResult{}, fmt.Errorf("Construct not implemented")
}

func (p *badProvider) Invoke(tok tokens.ModuleMember,
	args resource.PropertyMap,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	return nil, nil, fmt.Errorf("Invoke not implemented")
}

func (p *badProvider) StreamInvoke(tok tokens.ModuleMember, args resource.PropertyMap,
	onNext func(resource.PropertyMap) error,
) ([]plugin.CheckFailure, error) {
	return nil, fmt.Errorf("StreamInvoke not implemented")
}

func (p *badProvider) Call(tok tokens.ModuleMember, args resource.PropertyMap, info plugin.CallInfo,
	options plugin.CallOptions,
) (plugin.CallResult, error) {
	return plugin.CallResult{}, fmt.Errorf("Call not implemented")
}

func (p *badProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	ver := semver.MustParse("1.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *badProvider) SignalCancellation() error {
	return nil
}

func (p *badProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (p *badProvider) GetMappings(key string) ([]string, error) {
	return nil, nil
}
