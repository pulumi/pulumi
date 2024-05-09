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

type AssetArchiveProvider struct {
	plugin.UnimplementedProvider
}

var _ plugin.Provider = (*AssetArchiveProvider)(nil)

func (p *AssetArchiveProvider) Close() error {
	return nil
}

func (p *AssetArchiveProvider) Configure(inputs resource.PropertyMap) error {
	return nil
}

func (p *AssetArchiveProvider) Pkg() tokens.Package {
	return "asset-archive"
}

func (p *AssetArchiveProvider) GetSchema(request plugin.GetSchemaRequest) ([]byte, error) {
	assetProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Ref: "pulumi.json#/Asset",
			},
		},
	}
	archiveProperties := map[string]schema.PropertySpec{
		"value": {
			TypeSpec: schema.TypeSpec{
				Ref: "pulumi.json#/Archive",
			},
		},
	}
	resourceRequired := []string{"value"}

	pkg := schema.PackageSpec{
		Name:    "asset-archive",
		Version: "5.0.0",
		Resources: map[string]schema.ResourceSpec{
			"asset-archive:index:AssetResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: assetProperties,
					Required:   resourceRequired,
				},
				InputProperties: assetProperties,
				RequiredInputs:  resourceRequired,
			},
			"asset-archive:index:ArchiveResource": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type:       "object",
					Properties: archiveProperties,
					Required:   resourceRequired,
				},
				InputProperties: archiveProperties,
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

func (p *AssetArchiveProvider) checkType(urn resource.URN) (bool, error) {
	isAsset := false
	if urn.Type() == "asset-archive:index:AssetResource" {
		isAsset = true
	} else if urn.Type() != "asset-archive:index:ArchiveResource" {
		return false, fmt.Errorf("invalid URN type: %s", urn.Type())
	}
	return isAsset, nil
}

func (p *AssetArchiveProvider) CheckConfig(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
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
	if version.StringValue() != "5.0.0" {
		return nil, makeCheckFailure("version", "version is not 5.0.0"), nil
	}

	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *AssetArchiveProvider) Check(urn resource.URN, oldInputs, newInputs resource.PropertyMap,
	allowUnknowns bool, randomSeed []byte,
) (resource.PropertyMap, []plugin.CheckFailure, error) {
	isAsset, err := p.checkType(urn)
	if err != nil {
		return nil, makeCheckFailure("", err.Error()), nil
	}

	value, ok := newInputs["value"]
	if !ok {
		return nil, makeCheckFailure("value", "missing value"), nil
	}
	if isAsset {
		if !value.IsAsset() {
			return nil, makeCheckFailure("value", "value is not an asset"), nil
		}
	} else {
		if !value.IsArchive() {
			return nil, makeCheckFailure("value", "value is not an archive"), nil
		}
	}

	if len(newInputs) != 1 {
		return nil, makeCheckFailure("", fmt.Sprintf("too many properties: %v", newInputs)), nil
	}

	return newInputs, nil, nil
}

func (p *AssetArchiveProvider) Create(
	urn resource.URN, news resource.PropertyMap,
	timeout float64, preview bool,
) (resource.ID, resource.PropertyMap, resource.Status, error) {
	_, err := p.checkType(urn)
	if err != nil {
		return "", nil, resource.StatusUnknown, err
	}

	id := "id"
	if preview {
		id = ""
	}

	return resource.ID(id), news, resource.StatusOK, nil
}

func (p *AssetArchiveProvider) GetPluginInfo() (workspace.PluginInfo, error) {
	ver := semver.MustParse("5.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *AssetArchiveProvider) SignalCancellation() error {
	return nil
}

func (p *AssetArchiveProvider) GetMapping(key, provider string) ([]byte, string, error) {
	return nil, "", nil
}

func (p *AssetArchiveProvider) GetMappings(key string) ([]string, error) {
	return nil, nil
}

func (p *AssetArchiveProvider) DiffConfig(
	urn resource.URN, oldInputs, ouldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *AssetArchiveProvider) Diff(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs, newInputs resource.PropertyMap,
	allowUnknowns bool, ignoreChanges []string,
) (plugin.DiffResult, error) {
	return plugin.DiffResult{}, nil
}

func (p *AssetArchiveProvider) Delete(
	urn resource.URN, id resource.ID, oldInputs, oldOutputs resource.PropertyMap, timeout float64,
) (resource.Status, error) {
	return resource.StatusOK, nil
}
