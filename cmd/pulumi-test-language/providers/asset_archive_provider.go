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
	"context"
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

func (p *AssetArchiveProvider) Configure(
	context.Context, plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *AssetArchiveProvider) Pkg() tokens.Package {
	return "asset-archive"
}

func (p *AssetArchiveProvider) GetSchema(context.Context, plugin.GetSchemaRequest) (plugin.GetSchemaResponse, error) {
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
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
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

func (p *AssetArchiveProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	// Expect just the version
	version, ok := req.News["version"]
	if !ok {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "missing version")}, nil
	}
	if !version.IsString() {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not a string")}, nil
	}
	if version.StringValue() != "5.0.0" {
		return plugin.CheckConfigResponse{Failures: makeCheckFailure("version", "version is not 5.0.0")}, nil
	}

	if len(req.News) != 1 {
		return plugin.CheckConfigResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News)),
		}, nil
	}

	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *AssetArchiveProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	isAsset, err := p.checkType(req.URN)
	if err != nil {
		return plugin.CheckResponse{Failures: makeCheckFailure("", err.Error())}, nil
	}

	value, ok := req.News["value"]
	if !ok {
		return plugin.CheckResponse{Failures: makeCheckFailure("value", "missing value")}, nil
	}
	if isAsset {
		if !value.IsAsset() {
			return plugin.CheckResponse{Failures: makeCheckFailure("value", "value is not an asset")}, nil
		}
	} else {
		if !value.IsArchive() {
			return plugin.CheckResponse{Failures: makeCheckFailure("value", "value is not an archive")}, nil
		}
	}

	if len(req.News) != 1 {
		return plugin.CheckResponse{Failures: makeCheckFailure("", fmt.Sprintf("too many properties: %v", req.News))}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *AssetArchiveProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	_, err := p.checkType(req.URN)
	if err != nil {
		return plugin.CreateResponse{Status: resource.StatusUnknown}, err
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: req.Properties,
		Status:     resource.StatusOK,
	}, nil
}

func (p *AssetArchiveProvider) GetPluginInfo(context.Context) (workspace.PluginInfo, error) {
	ver := semver.MustParse("5.0.0")
	return workspace.PluginInfo{
		Version: &ver,
	}, nil
}

func (p *AssetArchiveProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *AssetArchiveProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *AssetArchiveProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *AssetArchiveProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *AssetArchiveProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *AssetArchiveProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{Status: resource.StatusOK}, nil
}
