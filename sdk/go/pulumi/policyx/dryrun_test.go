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

package policyx

import (
	"context"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// Policies read whether the operation is a preview from the *pulumi.Context passed to the policy pack factory.
func TestPolicyReadsDryRunFromPolicyPackContext(t *testing.T) {
	t.Parallel()

	for _, dryRun := range []bool{true, false} {
		t.Run(map[bool]string{true: "preview", false: "update"}[dryRun], func(t *testing.T) {
			t.Parallel()

			var seen []bool
			srv := &analyzerServer{
				policyPackFactory: func(pctx *pulumi.Context) (PolicyPack, error) {
					return NewPolicyPack("test-pack", semver.MustParse("1.0.0"), EnforcementLevelAdvisory, []Policy{
						NewResourceValidationPolicy("dry-run", ResourceValidationPolicyArgs{
							Description: "Records whether the operation is a preview",
							ValidateResource: func(context.Context, ResourceValidationArgs) error {
								seen = append(seen, pctx.DryRun())
								return nil
							},
						}),
					})
				},
			}

			_, err := srv.Handshake(t.Context(), &pulumirpc.AnalyzerHandshakeRequest{EngineAddress: "127.0.0.1:1"})
			require.NoError(t, err)
			_, err = srv.ConfigureStack(t.Context(), &pulumirpc.AnalyzerStackConfigureRequest{
				Stack:   "dev",
				Project: "test-project",
				DryRun:  dryRun,
			})
			require.NoError(t, err)
			_, err = srv.Analyze(t.Context(), &pulumirpc.AnalyzeRequest{
				Type: "test:index:Resource",
				Urn:  "urn:pulumi:dev::test-project::test:index:Resource::res",
			})
			require.NoError(t, err)

			assert.Equal(t, []bool{dryRun}, seen)
		})
	}
}
