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
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// An exception in a local policy pack's config file allows a mandatory violation, on a resource or on the stack, so
// that it doesn't fail the update. Exceptions apply to packs that don't support config, too.
func TestLocalPolicyPackExceptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		supportsConfig bool
		policies       string
		err            string
	}{
		{name: "pack with config support", supportsConfig: true, policies: `{"p": "mandatory"}`},
		{name: "pack without config support", policies: `{}`},
		{
			name:     "policy config for a pack without config support",
			policies: `{"p": "mandatory"}`,
			err:      "does not support config",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			analyzerPath, err := filepath.Abs("test")
			require.NoError(t, err)
			violation := plugin.AnalyzeDiagnostic{
				PolicyName:       "p",
				PolicyPackName:   "pack",
				Message:          "a policy failed",
				EnforcementLevel: apitype.Mandatory,
			}
			loaders := []*deploytest.PluginLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{}, nil
				}),
				deploytest.NewAnalyzerLoader(analyzerPath, func(*plugin.PolicyAnalyzerOptions) (plugin.Analyzer, error) {
					return &deploytest.Analyzer{
						Info: plugin.AnalyzerInfo{
							Name:           "pack",
							Policies:       []plugin.AnalyzerPolicyInfo{{Name: "p", EnforcementLevel: apitype.Mandatory}},
							SupportsConfig: tt.supportsConfig,
						},
						AnalyzeF: func(r plugin.AnalyzerResource) (plugin.AnalyzeResponse, error) {
							if r.Type != "pkgA:m:typA" {
								return plugin.AnalyzeResponse{}, nil
							}
							return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
						},
						// The stack violation has no URN, so the engine attributes it to the root stack.
						AnalyzeStackF: func([]plugin.AnalyzerStackResource) (plugin.AnalyzeResponse, error) {
							return plugin.AnalyzeResponse{Diagnostics: []plugin.AnalyzeDiagnostic{violation}}, nil
						},
					}, nil
				}, deploytest.WithGrpc),
			}
			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true)
				return err
			})

			p := &lt.TestPlan{}
			resURN := p.NewURN("pkgA:m:typA", "resA", "")
			rootURN := resource.DefaultRootStackURN(p.GetTarget(t, nil).Name.Q(), p.GetProject().Name)
			configPath := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(configPath, []byte(`{"version": 2, "policies": `+tt.policies+`,
				"exceptions": {"EXC-1": {"reason": "r", "match": [
					{"policies": ["p"], "targets": ["`+string(resURN)+`", "`+string(rootURN)+`"]}
				]}}}`), 0o600))
			p.Options = lt.TestUpdateOptions{
				T:                t,
				SkipDisplayTests: true,
				HostF:            deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...),
				UpdateOptions: UpdateOptions{
					LocalPolicyPacks: []LocalPolicyPack{{Path: "test", Config: configPath}},
				},
			}

			validate := func(_ workspace.Project, _ deploy.Target, _ JournalEntries, events []Event, err error) error {
				if tt.err != "" {
					return err
				}
				var urns []resource.URN
				for _, e := range events {
					if v, ok := e.Payload().(PolicyViolationEventPayload); ok {
						urns = append(urns, v.ResourceURN)
						assert.Equal(t, []apitype.PolicyEventException{{ID: "EXC-1", Reason: "r"}}, v.Exceptions)
					}
				}
				assert.ElementsMatch(t, []resource.URN{resURN, rootURN}, urns)
				return err
			}
			_, err = lt.TestOp(Update).Run(p.GetProject(), p.GetTarget(t, nil), p.Options, false, p.BackendClient, validate)
			if tt.err != "" {
				assert.ErrorContains(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
