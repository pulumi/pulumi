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

// Package backend encapsulates all extensibility points required to fully implement a new cloud provider.
package backend

import (
	"context"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/v2/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v2/resource/stack"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource"
)

// NewBackendClient returns a deploy.BackendClient that wraps the given Backend.
func NewBackendClient(client Client) deploy.BackendClient {
	return &backendClient{client: client}
}

type backendClient struct {
	client Client
}

// GetStackOutputs returns the outputs of the stack with the given name.
func (c *backendClient) GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error) {
	id, err := ParseStackIdentifierWithClient(ctx, name, c.client)
	if err != nil {
		return nil, err
	}
	deployment, err := c.client.ExportStackDeployment(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	snapshot, err := stack.DeserializeUntypedDeployment(&deployment, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, err
	}
	res, err := stack.GetRootStackResource(snapshot)
	if err != nil {
		return nil, errors.Wrap(err, "getting root stack resources")
	}
	if res == nil {
		return resource.PropertyMap{}, nil
	}
	return res.Outputs, nil
}

func (c *backendClient) GetStackResourceOutputs(ctx context.Context, name string) (resource.PropertyMap, error) {
	id, err := ParseStackIdentifierWithClient(ctx, name, c.client)
	if err != nil {
		return nil, err
	}
	deployment, err := c.client.ExportStackDeployment(ctx, id, nil)
	if err != nil {
		return nil, err
	}
	snapshot, err := stack.DeserializeUntypedDeployment(&deployment, stack.DefaultSecretsProvider)
	if err != nil {
		return nil, err
	}
	pm := resource.PropertyMap{}
	for _, r := range snapshot.Resources {
		if r.Delete {
			continue
		}

		resc := resource.PropertyMap{
			resource.PropertyKey("type"):    resource.NewStringProperty(string(r.Type)),
			resource.PropertyKey("outputs"): resource.NewObjectProperty(r.Outputs)}
		pm[resource.PropertyKey(r.URN)] = resource.NewObjectProperty(resc)
	}
	return pm, nil
}
