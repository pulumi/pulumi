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
	"net/http"
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
	// Classify the errors so that callers creating an environment that already exists see
	// backend.ErrEnvironmentConflict and can reuse it rather than failing.
	if err := b.escClient.CreateEnvironment(ctx, org, projectName, envName); err != nil {
		return nil, classifyEnvironmentError(err)
	}
	diags, _, err := b.escClient.UpdateEnvironment(ctx, org, projectName, envName, yaml, "")
	return convertESCDiags(diags), classifyEnvironmentError(err)
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

var _ = backend.EnvironmentDefinitionsBackend((*cloudBackend)(nil))

// classifyEnvironmentError maps the HTTP status codes the ESC API uses for optimistic-concurrency and
// missing-environment failures onto the sentinel errors the command layer matches on.
func classifyEnvironmentError(err error) error {
	if err == nil {
		return nil
	}
	var code int
	switch e := err.(type) { //nolint:errorlint // the client returns these error values directly
	case *apitype.ErrorResponse:
		code = e.Code
	case *client.EnvironmentErrorResponse:
		code = e.Code
	}
	switch code {
	case http.StatusPreconditionFailed, http.StatusConflict:
		return fmt.Errorf("%w: %w", backend.ErrEnvironmentConflict, err)
	case http.StatusNotFound:
		return fmt.Errorf("%w: %w", backend.ErrEnvironmentNotFound, err)
	}
	return err
}

func (b *cloudBackend) GetEnvironmentDefinition(
	ctx context.Context,
	org string,
	envProject string,
	envName string,
	version string,
) ([]byte, string, int, error) {
	// decrypt is false: existing secrets round-trip through read-modify-write as opaque ciphertext, so
	// no other value's plaintext ever enters this process's memory.
	yaml, etag, revision, err := b.escClient.GetEnvironment(ctx, org, envProject, envName, version, false)
	if err != nil {
		return nil, "", 0, classifyEnvironmentError(err)
	}
	return yaml, etag, revision, nil
}

func (b *cloudBackend) UpdateEnvironmentDefinition(
	ctx context.Context,
	org string,
	envProject string,
	envName string,
	yaml []byte,
	etag string,
) (apitype.EnvironmentDiagnostics, int, error) {
	diags, revision, err := b.escClient.UpdateEnvironment(ctx, org, envProject, envName, yaml, etag)
	if err != nil {
		return nil, 0, classifyEnvironmentError(err)
	}
	return convertESCDiags(diags), revision, nil
}

func (b *cloudBackend) GetEnvironmentRevision(
	ctx context.Context,
	org string,
	envProject string,
	envName string,
	version string,
) (int, error) {
	revision, err := b.escClient.GetRevisionNumber(ctx, org, envProject, envName, version)
	if err != nil {
		return 0, classifyEnvironmentError(err)
	}
	return revision, nil
}
