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
	"io"

	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pulumi/pulumi/pkg/resource"
)

// BackendClient provides a simple implementation of deploy.BackendClient that defers to a function value.
type BackendClient struct {
	GetStackOutputsF func(ctx context.Context, name string) (resource.PropertyMap, error)
}

// GetStackOutputs returns the outputs (if any) for the named stack or an error if the stack cannot be found.
func (b *BackendClient) GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error) {
	return b.GetStackOutputsF(ctx, name)
}

// DownloadPlugin o ptionally downloads a plugin corresponding to the requested plugin info.
func (b *BackendClient) DownloadPlugin(ctx context.Context, plugin workspace.PluginInfo) (io.ReadCloser, error) {
	return nil, nil
}
