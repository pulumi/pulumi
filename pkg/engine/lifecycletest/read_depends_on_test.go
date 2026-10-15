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
	"sync"
	"testing"
	"time"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadDependsOnOrdering verifies that a ReadResource with an explicit
// dependsOn (dependencies) waits for its dependencies to be created before
// the provider's Read is invoked. This covers https://github.com/pulumi/pulumi/issues/16395,
// where `.get()` with `dependsOn` was observed to invoke Read against a resource
// that had not been created yet (notably during preview, where the engine executed
// the Read as soon as the ID was known).
//
// Runs in both update and preview modes: in preview the Create step is a no-op
// at the provider level but still produces an unknown ID, and the Read must
// still be ordered after it.
func TestReadDependsOnOrdering(t *testing.T) {
	t.Parallel()

	for _, dryRun := range []bool{false, true} {
		mode := "update"
		if dryRun {
			mode = "preview"
		}
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var mu sync.Mutex
			events := []string{}
			record := func(s string) {
				mu.Lock()
				defer mu.Unlock()
				events = append(events, s)
			}

			loaders := []*deploytest.ProviderLoader{
				deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
					return &deploytest.Provider{
						CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
							// Hold the Create briefly so that a buggy, unordered engine would
							// race the Read ahead of it.
							time.Sleep(100 * time.Millisecond)
							record("create:" + req.URN.Name())
							id := resource.ID("src-id")
							if req.Preview {
								id = ""
							}
							return plugin.CreateResponse{
								ID:         id,
								Properties: req.Properties,
								Status:     resource.StatusOK,
							}, nil
						},
						ReadF: func(_ context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
							record("read:" + req.URN.Name())
							return plugin.ReadResponse{
								ReadResult: plugin.ReadResult{
									ID:      req.ID,
									Outputs: new(property.Map{}),
								},
								Status: resource.StatusOK,
							}, nil
						},
					}, nil
				}),
			}

			programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
				src, err := monitor.RegisterResource("pkgA:m:typA", "src", true)
				require.NoError(t, err)

				// Call ReadResource directly on the raw monitor so we can set the
				// `dependencies` field (deploytest's ReadResource helper does not expose it).
				_, err = monitor.Client().ReadResource(context.Background(), &pulumirpc.ReadResourceRequest{
					Type:         "pkgA:m:typA",
					Name:         "res",
					Id:           "existing-id",
					Dependencies: []string{string(src.URN)},
				})
				require.NoError(t, err)
				return nil
			})

			hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)
			p := &lt.TestPlan{
				Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
			}

			project := p.GetProject()
			_, err := lt.TestOp(Update).Run(project, p.GetTarget(t, nil), p.Options, dryRun, p.BackendClient, nil)
			require.NoError(t, err)

			mu.Lock()
			defer mu.Unlock()
			if dryRun {
				// In preview, src has not actually been created yet (its Create ran with
				// Preview=true and returned no real ID). Per issue #16395 the engine should
				// not invoke the provider's Read against a resource whose dependsOn target
				// does not yet exist — otherwise the real-world Read will error with
				// "resource does not exist". The outputs of res should instead be unknown.
				assert.NotContains(t, events, "read:res",
					"Read should be skipped in preview when dependsOn target is being newly created (issue #16395)")
			} else {
				if assert.Len(t, events, 2, "expected one Create and one Read") {
					assert.Equal(t, "create:src", events[0],
						"Create of src must run before Read of res (dependsOn ordering, issue #16395)")
					assert.Equal(t, "read:res", events[1])
				}
			}
		})
	}
}
