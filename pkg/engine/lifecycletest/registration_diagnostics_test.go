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
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	pkgresource "github.com/pulumi/pulumi/pkg/v3/resource"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestResourceFailureRegistrationDiagnostic(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"check", "create", "construct"} {
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), "main.py")
			require.NoError(t, os.WriteFile(file, []byte("site = Service(port=0)\n"), 0o600))
			position := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file), Fragment: "1"}).String()
			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CheckF: func(_ context.Context, req plugin.CheckRequest) (plugin.CheckResponse, error) {
							response := plugin.CheckResponse{Properties: req.NewInputs}
							if operation == "check" {
								response.Failures = []plugin.CheckFailure{{Property: "port", Reason: "invalid fixture port"}}
							}
							return response, nil
						},
						ConstructF: func(
							context.Context, plugin.ConstructRequest, *deploytest.ResourceMonitor,
						) (plugin.ConstructResponse, error) {
							return plugin.ConstructResponse{}, errors.New("denied fixture construct")
						},
						CreateF: func(context.Context, plugin.CreateRequest) (plugin.CreateResponse, error) {
							return plugin.CreateResponse{}, errors.New("denied fixture create")
						},
					}, nil
				}),
			}
			program := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				options := deploytest.ResourceOptions{
					Remote:         operation == "construct",
					Inputs:         property.NewMap(map[string]property.Value{"port": property.New(0.0)}),
					SourcePosition: position,
					StackTrace:     []pkgresource.StackFrame{{SourcePosition: position}},
				}
				_, err := monitor.RegisterResource("pkgA:index:Service", "site", operation != "construct", options)
				return err
			})
			plan := &lt.TestPlan{
				Options: lt.TestUpdateOptions{
					T: t, HostF: deploytest.NewPluginHostF(nil, nil, program, nil, nil, loaders...),
					SkipDisplayTests: true,
				},
				Steps: []lt.TestStep{{
					Op: engine.Update, SkipPreview: true, ExpectFailure: true,
					Validate: func(
						_ workspace.Project, _ deploy.Target, _ engine.JournalEntries, events []engine.Event, err error,
					) error {
						found := false
						for _, event := range events {
							if event.Type != engine.DiagEvent {
								continue
							}
							diagnostic := event.Payload().(engine.DiagEventPayload)
							message := diagnostic.Message
							if diagnostic.Severity == diag.Error && diagnostic.SourceContext != nil {
								assert.Contains(t, diagnostic.SourceContext.Rendered, "main.py:1")
								assert.Contains(t, colors.Never.Colorize(diagnostic.SourceContext.Rendered), "site = Service(port=0)")
								assert.NotContains(t, message, "site = Service(port=0)")
								assert.Contains(t, diagnostic.SourceContext.StackTrace,
									apitype.StackFrameV1{
										SourcePosition: diagnostic.SourceContext.SourcePosition,
										Boundary: &apitype.StackTraceBoundaryV1{
											Kind: apitype.StackTraceBoundaryKindProgram, CallSite: diagnostic.SourceContext.SourcePosition,
										},
									})
								found = true
							}
						}
						assert.True(t, found, "expected registration context with the resource diagnostic")
						return err
					},
				}},
			}
			snapshot := plan.Run(t, nil)
			if snapshot != nil {
				for _, state := range snapshot.Resources {
					assert.NotEqual(t, "site", state.URN.Name(), "failed resources should not need saved state for their context")
				}
			}
		})
	}
}
