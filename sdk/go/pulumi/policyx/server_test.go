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
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// newTestAnalyzerServer returns a server whose policy pack factory records the stack of each context it receives.
func newTestAnalyzerServer(stacks *[]string) *analyzerServer {
	return &analyzerServer{
		policyPackFactory: func(ctx *pulumi.Context) (PolicyPack, error) {
			*stacks = append(*stacks, ctx.Stack())
			return NewPolicyPack("test-pack", semver.MustParse("1.2.3"), EnforcementLevelAdvisory, []Policy{
				NewResourceValidationPolicy("test-policy", ResourceValidationPolicyArgs{
					Description:      "A test policy",
					EnforcementLevel: EnforcementLevelMandatory,
				}),
			})
		},
	}
}

// `pulumi policy publish` loads the policy pack without calling ConfigureStack, so GetPluginInfo and GetAnalyzerInfo
// must work without it.
func TestGetAnalyzerInfoWithoutConfigureStack(t *testing.T) {
	t.Parallel()

	var stacks []string
	srv := newTestAnalyzerServer(&stacks)
	_, err := srv.Handshake(t.Context(), &pulumirpc.AnalyzerHandshakeRequest{EngineAddress: "127.0.0.1:1"})
	require.NoError(t, err)

	pluginInfo, err := srv.GetPluginInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pluginInfo.GetVersion())

	info, err := srv.GetAnalyzerInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "test-pack", info.GetName())
	assert.Equal(t, "1.2.3", info.GetVersion())
	require.Len(t, info.GetPolicies(), 1)
	assert.Equal(t, "test-policy", info.GetPolicies()[0].GetName())
	assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, info.GetPolicies()[0].GetEnforcementLevel())

	assert.Equal(t, []string{""}, stacks, "expected the policy pack to be created once, without a stack")
}

func TestConfigureStackRecreatesPolicyPackWithStack(t *testing.T) {
	t.Parallel()

	var stacks []string
	srv := newTestAnalyzerServer(&stacks)
	_, err := srv.Handshake(t.Context(), &pulumirpc.AnalyzerHandshakeRequest{EngineAddress: "127.0.0.1:1"})
	require.NoError(t, err)

	_, err = srv.GetAnalyzerInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)
	_, err = srv.ConfigureStack(t.Context(), &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:   "dev",
		Project: "test-project",
	})
	require.NoError(t, err)
	info, err := srv.GetAnalyzerInfo(t.Context(), &emptypb.Empty{})
	require.NoError(t, err)

	assert.Equal(t, "test-pack", info.GetName())
	assert.Equal(t, []string{"", "dev"}, stacks)
}
