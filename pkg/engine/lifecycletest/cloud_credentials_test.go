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
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type credentialCheckingHost struct {
	plugin.Host
	t        *testing.T
	expected map[string]string
}

func (h *credentialCheckingHost) LanguageRuntime(ctx *plugin.Context, runtime string) (plugin.LanguageRuntime, error) {
	assert.Equal(h.t, h.expected, ctx.CloudCredentialEnv)
	return h.Host.LanguageRuntime(ctx, runtime)
}

func TestUpdatePassesCloudCredentialsToPluginContext(t *testing.T) {
	t.Setenv("PULUMI_BACKEND_URL", "https://ambient.example.com")
	t.Setenv("PULUMI_ACCESS_TOKEN", "ambient-token")
	//nolint:paralleltest // Subtests share inherited process credentials configured by the parent.
	for _, test := range []struct {
		name        string
		credentials map[string]string
	}{
		{"none", nil},
		{"token", map[string]string{"PULUMI_API": "https://helper.example.com", "PULUMI_ACCESS_TOKEN": "helper-token"}},
		{"headers", map[string]string{"PULUMI_API": "https://helper.example.com", "PULUMI_ACCESS_TOKEN": ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected := test.credentials
			runtime := deploytest.NewLanguageRuntimeF(func(plugin.RunInfo, *deploytest.ResourceMonitor) error { return nil })
			host := deploytest.NewPluginHostF(nil, nil, runtime, nil, nil)
			plan := &lt.TestPlan{Options: lt.TestUpdateOptions{
				T: t, SkipDisplayTests: true,
				HostF:         func() plugin.Host { return &credentialCheckingHost{Host: host(), t: t, expected: expected} },
				UpdateOptions: engine.UpdateOptions{CloudCredentialEnv: expected},
			}}
			_, err := lt.TestOp(engine.Update).RunStep(
				plan.GetProject(), plan.GetTarget(t, nil), plan.Options, false, plan.BackendClient, nil, "0")
			require.NoError(t, err)
		})
	}
}
