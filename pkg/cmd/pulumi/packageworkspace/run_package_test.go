// Copyright 2026, Pulumi Corporation.
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

package packageworkspace_test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Regression test for https://github.com/pulumi/pulumi/issues/25637. The host owns providers started by RunPackage
// and stops them when it closes, so cancelling the caller's context must not cancel the provider's plugin context.
// Otherwise a provider run by a language host is killed early, and the host reports it as having exited prematurely
// when it later shuts it down.
func TestRunPackageProviderOutlivesCallerContext(t *testing.T) {
	t.Parallel()

	var providerCtx context.Context
	host := &plugin.MockHost{
		ProviderFromPathF: func(ctx *plugin.Context, _ string, _ env.Env) (plugin.Provider, error) {
			providerCtx = ctx.Request()
			return &plugin.MockProvider{}, nil
		},
	}
	pctx, err := plugin.NewContextWithHost(t.Context(), nil, nil, host, "", "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pctx.Close()) })

	ws := packageworkspace.New(nil, nil, pctx, io.Discard, io.Discard, nil, packageworkspace.Options{})

	ctx, cancel := context.WithCancel(t.Context())
	_, err = ws.RunPackage(ctx, t.TempDir(), "plugin-path", nil, workspace.PackageSpec{})
	require.NoError(t, err)
	cancel()

	require.NotNil(t, providerCtx)
	require.NoError(t, providerCtx.Err())
}
