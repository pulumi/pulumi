// Copyright 2016-2024, Pulumi Corporation.
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
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// LargeProvider is a test provider that exercises the provider protocol by returning really large strings, lists, and
// maps.
type LargeProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*LargeProvider)(nil)

func (p *LargeProvider) Close() error {
	return nil
}

func (p *LargeProvider) Configure(inputs resource.PropertyMap) error {
	return nil
}

func (p *LargeProvider) Pkg() tokens.Package {
	return "large"
}

func (p *LargeProvider) GetSchema(request plugin.GetSchemaRequest) ([]byte, error) {
	resourceProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Type: "string",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "large",
		Version: "4.3.2",
		Resources: map[string]schema.ResourceSpec{
			"large:index:String": {
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

func (p *LargeProvider) CheckConfig(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
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
	if version.StringValue() != "4.3.2" {
		return nil, makeCheckFailure("version", "version is not 4.3.2"), nil
	}

	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *LargeProvider) Check(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	if urn.Type() != "large:index:String" {
		return nil, makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", urn.Type())), nil
	}

	// Expect just the boolean value
	value, ok := newInputs["value"]
	if !ok {
		return nil, makeCheckFailure("value", "missing value"), nil
	}
	if !value.IsString() {
		return nil, makeCheckFailure("value", "value is not a string"), nil
	}
	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *LargeProvider) Create(
	urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	if urn.Type() != "large:index:String" {
		return "", nil, resource.StatusUnknown, fmt.Errorf("invalid URN type: %s", urn.Type())
	}

	id := "id"
	if preview {
		id = ""
	}

	// Take the input value and _massively_ expand it.
	value, ok := news["value"]
	if !ok {
		return "", nil, resource.StatusUnknown, errors.New("missing value")
	}
	if !value.IsString() {
		return "", nil, resource.StatusUnknown, errors.New("value is not a string")
	}

	// aim for 100mb of data (400mb is the size limit we normally set, but nodejs is far more limited)
	repeat := (100 * 1024 * 1024) / len(value.StringValue())
	result := resource.PropertyMap{
		"value": resource.NewStringProperty(
			strings.Repeat(value.StringValue(), repeat)),
	}
	return resource.ID(id), result, resource.StatusOK, nil
}

func (p *LargeProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	ver := semver.MustParse("4.3.2")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *LargeProvider) SignalCancellation() error {
	return nil
}

func (p *LargeProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (p *LargeProvider) GetMappings(key string) ([]string, error) {
	return nil, nil
}

func (p *LargeProvider) DiffConfig(
	urn resource.URN, oldInputs, ouldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *LargeProvider) Diff(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *LargeProvider) Delete(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap, timeout float64,
) (resource.Status, error) {
	return resource.StatusOK, nil
}
