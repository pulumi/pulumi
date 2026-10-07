// Copyright 2016, Pulumi Corporation.
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
	"fmt"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
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
	projectName string,
	envName string,
	yaml []byte,
) (apitype.EnvironmentDiagnostics, error) {
	if err := b.escClient.CreateEnvironment(ctx, org, projectName, envName); err != nil {
		return nil, err
	}
	diags, _, err := b.escClient.UpdateEnvironment(ctx, org, projectName, envName, yaml, "")
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
	environmentOverrides map[string]string,
) (*esc.Environment, apitype.EnvironmentDiagnostics, error) {
	id, diags, err := b.escClient.OpenYAMLEnvironment(ctx, org, yaml, duration,
		client.OpenYAMLOption{EnvironmentOverrides: environmentOverrides})
	if err != nil || len(diags) != 0 {
		return nil, convertESCDiags(diags), err
	}
	env, err := b.escClient.GetAnonymousOpenEnvironment(ctx, org, id)
	return env, nil, err
}

var _ = backend.StackEnvironmentsBackend((*cloudBackend)(nil))

func (b *cloudBackend) SyncStackEnvironment(
	ctx context.Context,
	stack backend.Stack,
	definition []byte,
	opts backend.StackEnvironmentSyncOptions,
) (*backend.StackEnvironmentSync, error) {
	if !b.Capabilities(ctx).StackEnvironmentSync {
		return nil, backend.ErrStackEnvironmentSyncUnsupported
	}

	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	req := apitype.StackEnvironmentSyncRequest{
		Yaml:             string(definition),
		ExpectedRevision: opts.ExpectedRevision,
		DryRun:           opts.DryRun,
	}
	if opts.Duration != 0 {
		req.OpenDuration = opts.Duration.String()
	}
	resp, err := b.client.SyncStackEnvironment(ctx, stackID, req)
	if err != nil {
		return nil, err
	}

	res := &backend.StackEnvironmentSync{
		Environment:       resp.Environment,
		PreviousRevision:  resp.PreviousRevision,
		Revision:          resp.Revision,
		Created:           resp.Created,
		Changed:           resp.Changed,
		CurrentDefinition: []byte(resp.CurrentYaml),
		OpenSessionID:     resp.OpenSessionID,
		Diagnostics:       resp.Diagnostics,
	}
	if resp.OpenSessionID == "" || len(resp.Diagnostics) != 0 {
		return res, nil
	}

	projectName, envName, ok := strings.Cut(resp.Environment, "/")
	if !ok {
		return nil, fmt.Errorf("the service named the stack's environment %q, expected project/name", resp.Environment)
	}
	env, err := b.escClient.GetOpenEnvironment(ctx, stackID.Owner, projectName, envName, resp.OpenSessionID)
	if err != nil {
		return nil, fmt.Errorf("reading environment %s@%d: %w", resp.Environment, resp.Revision, err)
	}
	res.Opened = env
	return res, nil
}
