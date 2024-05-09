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

package providers

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

type SimpleProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*SimpleProvider)(nil)

func (p *SimpleProvider) Close() error {
	return nil
}

func (p *SimpleProvider) Configure(inputs resource.PropertyMap) error {
	return nil
}

func (p *SimpleProvider) Pkg() tokens.Package {
	return "simple"
}

func (p *SimpleProvider) GetSchema(request plugin.GetSchemaRequest) ([]byte, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "boolean",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "simple",
		Version: "2.0.0",
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

func makeCheckFailure(property resource.PropertyKey, reason string) []plugin.CheckFailure {
	return []plugin.CheckFailure{{
		Property: property,
		Reason:   reason,
	}}
}

func (p *SimpleProvider) CheckConfig(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
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
	if version.StringValue() != "2.0.0" {
		return nil, makeCheckFailure("version", "version is not 2.0.0"), nil
	}

	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *SimpleProvider) Check(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	// URN should be of the form "simple:index:Resource"
	if urn.Type() != "simple:index:Resource" {
		return nil, makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", urn.Type())), nil
	}

	// Expect just the boolean value
	value, ok := newInputs["value"]
	if !ok {
		return nil, makeCheckFailure("value", "missing value"), nil
	}
	if !value.IsBool() {
		return nil, makeCheckFailure("value", "value is not a boolean"), nil
	}
	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *SimpleProvider) Create(
	urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	// URN should be of the form "simple:index:Resource"
	if urn.Type() != "simple:index:Resource" {
		return "", nil, resource.StatusUnknown, fmt.Errorf("invalid URN type: %s", urn.Type())
	}

	id := "id"
	if preview {
		id = ""
	}

	return resource.ID(id), news, resource.StatusOK, nil
}

func (p *SimpleProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	ver := semver.MustParse("2.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *SimpleProvider) SignalCancellation() error {
	return nil
}

func (p *SimpleProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (p *SimpleProvider) GetMappings(key string) ([]string, error) {
	return nil, nil
}

func (p *SimpleProvider) DiffConfig(
	urn resource.URN, oldInputs, ouldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleProvider) Diff(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *SimpleProvider) Delete(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap, timeout float64,
) (resource.Status, error) {
	return resource.StatusOK, nil
}
