// Copyright 2025, Pulumi Corporation.
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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

type SyncProvider struct {
	plugin.UnimplementedProvider

	CreateLimit *sync.WaitGroup
}

var syncVersion = semver.MustParse("3.0.0-alpha.1.internal+exp.sha.2143768")

var _ plugin.Provider = (*SyncProvider)(nil)

func (p *SyncProvider) Close() error {
	return nil
}

func (p *SyncProvider) Configure(
	_ context.Context, req plugin.ConfigureRequest,
) (plugin.ConfigureResponse, error) {
	return plugin.ConfigureResponse{}, nil
}

func (p *SyncProvider) GetSchema(
	context.Context, plugin.GetSchemaRequest,
) (plugin.GetSchemaResponse, error) {
	pkg := schema.PackageSpec{
		Name:    p.Pkg().String(),
		Version: syncVersion.String(),
		Resources: map[string]schema.ResourceSpec{
			"sync:index:Block": {
				ObjectTypeSpec: schema.ObjectTypeSpec{
					Type: "object",
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(pkg)
	return plugin.GetSchemaResponse{Schema: jsonBytes}, err
}

func (p *SyncProvider) CheckConfig(
	_ context.Context, req plugin.CheckConfigRequest,
) (plugin.CheckConfigResponse, error) {
	return plugin.CheckConfigResponse{Properties: req.News}, nil
}

func (p *SyncProvider) Check(
	_ context.Context, req plugin.CheckRequest,
) (plugin.CheckResponse, error) {
	// URN should be of the form "sync:index:Resource"
	if req.URN.Type() != "sync:index:Block" {
		return plugin.CheckResponse{
			Failures: makeCheckFailure("", fmt.Sprintf("invalid URN type: %s", req.URN.Type())),
		}, nil
	}

	return plugin.CheckResponse{Properties: req.News}, nil
}

func (p *SyncProvider) Create(
	_ context.Context, req plugin.CreateRequest,
) (plugin.CreateResponse, error) {
	// URN should be of the form "sync:index:Resource"
	if req.URN.Type() != "sync:index:Block" {
		return plugin.CreateResponse{
			Status: resource.StatusUnknown,
		}, fmt.Errorf("invalid URN type: %s", req.URN.Type())
	}

	id := "id"
	if req.Preview {
		id = ""
	}

	// Block until p.CreateLimit is done or we time out.
	//
	// This is how we ensure that there are at least N parallel creates at once.
	if err := p.waitWithTimeout(p.CreateLimit); err != nil {
		return plugin.CreateResponse{}, err
	}

	return plugin.CreateResponse{
		ID:         resource.ID(id),
		Properties: resource.PropertyMap{},
		Status:     resource.StatusOK,
	}, nil
}

func (p *SyncProvider) waitWithTimeout(wg *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() {
		wg.Done()
		wg.Wait()
		done <- struct{}{}
	}()
	timeout := time.Tick(time.Second * 3)
	select {
	case <-timeout:
		return errors.New("timeout")
	case <-done:
		return nil
	}
}

func (p *SyncProvider) Pkg() tokens.Package {
	return "sync"
}

func (p *SyncProvider) GetPluginInfo(context.Context) (plugin.PluginInfo, error) {
	return plugin.PluginInfo{
		Version: &syncVersion,
	}, nil
}

func (p *SyncProvider) SignalCancellation(context.Context) error {
	return nil
}

func (p *SyncProvider) GetMapping(
	context.Context, plugin.GetMappingRequest,
) (plugin.GetMappingResponse, error) {
	return plugin.GetMappingResponse{}, nil
}

func (p *SyncProvider) GetMappings(
	context.Context, plugin.GetMappingsRequest,
) (plugin.GetMappingsResponse, error) {
	return plugin.GetMappingsResponse{}, nil
}

func (p *SyncProvider) DiffConfig(
	context.Context, plugin.DiffConfigRequest,
) (plugin.DiffConfigResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SyncProvider) Diff(
	context.Context, plugin.DiffRequest,
) (plugin.DiffResponse, error) {
	return plugin.DiffResult{}, nil
}

func (p *SyncProvider) Delete(
	context.Context, plugin.DeleteRequest,
) (plugin.DeleteResponse, error) {
	return plugin.DeleteResponse{}, nil
}
