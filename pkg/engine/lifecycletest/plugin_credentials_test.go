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

package lifecycletest

import (
	"sync/atomic"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/auth"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/stretchr/testify/assert"
)

type credentialCheckingHost struct {
	plugin.Host
	t           *testing.T
	calls       *atomic.Int32
	credentials *auth.Credentials
}

func (h *credentialCheckingHost) Loader(ctx *plugin.Context) (*plugin.GrpcServer, error) {
	h.calls.Add(1)
	assert.Same(h.t, h.credentials, ctx.Credentials())
	return h.Host.Loader(ctx)
}

func TestDeploymentPluginCredentials(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	credentials := &auth.Credentials{
		BackendURL: "https://opened.example.com",
		HTTPAuth:   auth.NewHTTPAuth("https://opened.example.com", "effective-token", nil),
	}
	host := deploytest.NewPluginHostF(nil, nil, deploytest.NewLanguageRuntimeF(
		func(plugin.RunInfo, *deploytest.ResourceMonitor) error { return nil }), nil, nil)
	plan := &lt.TestPlan{
		Options: lt.TestUpdateOptions{
			T: t, SkipDisplayTests: true,
			HostF: func() plugin.Host {
				return &credentialCheckingHost{Host: host(), t: t, calls: &calls, credentials: credentials}
			},
		},
		Steps: []lt.TestStep{{Op: func(
			info engine.UpdateInfo, ctx *engine.Context, opts engine.UpdateOptions, dryRun bool,
		) (*deploy.Plan, display.ResourceChanges, error) {
			ctx.Credentials = credentials
			return engine.Update(info, ctx, opts, dryRun)
		}}},
	}
	plan.Run(t, nil)
	assert.Positive(t, calls.Load())
}
