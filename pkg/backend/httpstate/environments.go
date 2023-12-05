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

package httpstate

import (
	"context"
	"time"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var _ = backend.EnvironmentsBackend((*cloudBackend)(nil))

func convertESCDiags(diags []client.EnvironmentDiagnostic) apitype.EnvironmentDiagnostics {
	if len(diags) == 0 {
		return nil
	}
	apiDiags := make(apitype.EnvironmentDiagnostics, len(diags))
	for i, d := range diags {
		apiDiags[i] = apitype.EnvironmentDiagnostic{
			Range:   d.Range,
			Summary: d.Summary,
			Detail:  d.Detail,
		}
	}
	return apiDiags
}

func (b *cloudBackend) CreateEnvironment(
	ctx context.Context,
	org string,
	name string,
	yaml []byte,
) (apitype.EnvironmentDiagnostics, error) {
	if err := b.escClient.CreateEnvironment(ctx, org, name); err != nil {
		return nil, err
	}
	diags, err := b.escClient.UpdateEnvironment(ctx, org, name, yaml, "")
	return convertESCDiags(diags), err
}

func (b *cloudBackend) CheckYAMLEnvironment(
	ctx context.Context,
	org string,
	yaml []byte,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	env, diags, err := b.escClient.CheckYAMLEnvironment(ctx, org, yaml)
	return env, convertESCDiags(diags), err
}

func (b *cloudBackend) OpenYAMLEnvironment(
	ctx context.Context,
	org string,
	yaml []byte,
	duration time.Duration,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	id, diags, err := b.escClient.OpenYAMLEnvironment(ctx, org, yaml, duration)
	if err != nil || len(diags) != 0 {
		return nil, convertESCDiags(diags), err
	}
	env, err := b.escClient.GetOpenEnvironment(ctx, org, "yaml", id)
	return env, nil, err
}
