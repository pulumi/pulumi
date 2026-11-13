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
	"testing"

	"github.com/blang/semver"
	. "github.com/pulumi/pulumi/pkg/v3/engine"
	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemoveSecretFromInputs tests that dropping pulumi.secret from a program input also clears the
// secret bit the engine mirrored onto the matching output, even when the provider reports no diff.
// See https://github.com/pulumi/pulumi/issues/13877.
func TestRemoveSecretFromInputs(t *testing.T) {
	t.Parallel()

	loaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkgA", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			// A bridged-style provider: it does not accept secrets, so it only ever sees, diffs and
			// returns plaintext.
			return &deploytest.Provider{
				DiffF: func(_ context.Context, req plugin.DiffRequest) (plugin.DiffResult, error) {
					if plaintext(req.OldOutputs["bucket"]).DeepEquals(plaintext(req.NewInputs["bucket"])) {
						return plugin.DiffResult{Changes: plugin.DiffNone}, nil
					}
					return plugin.DiffResult{Changes: plugin.DiffSome, ChangedKeys: []resource.PropertyKey{"bucket"}}, nil
				},
				CreateF: func(_ context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
					bucket := plaintext(req.Properties["bucket"])
					return plugin.CreateResponse{
						ID: "id123",
						Properties: resource.PropertyMap{
							"bucket": bucket,
							// An output the provider generates itself, unrelated to any input.
							"arn": resource.NewProperty("arn:" + bucket.StringValue()),
						},
						Status: resource.StatusOK,
					}, nil
				},
			}, nil
		}),
	}

	var inputs resource.PropertyMap
	programF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		_, err := monitor.RegisterResource("pkgA:m:typA", "resA", true, deploytest.ResourceOptions{
			Inputs:                  inputs,
			AdditionalSecretOutputs: []resource.PropertyKey{"arn"},
		})
		require.NoError(t, err)
		return nil
	})
	hostF := deploytest.NewPluginHostF(nil, nil, programF, nil, nil, loaders...)

	p := &lt.TestPlan{
		// Skip display tests because secrets are serialized with the blinding crypter and can't be restored
		Options: lt.TestUpdateOptions{T: t, HostF: hostF, SkipDisplayTests: true},
		Steps:   []lt.TestStep{{Op: Update, SkipPreview: true}},
	}

	// The program marks the input as secret.
	inputs = resource.PropertyMap{"bucket": resource.MakeSecret(resource.NewProperty("b"))}
	snap := p.Run(t, nil)

	resA := snap.Resources[len(snap.Resources)-1]
	require.Equal(t, "resA", resA.URN.Name())
	assert.True(t, resA.Inputs["bucket"].IsSecret())
	assert.True(t, resA.Outputs["bucket"].IsSecret())

	// The program stops marking the input as secret. The provider still reports no diff, so this is a
	// same step, but the state must no longer claim the value is a secret.
	inputs = resource.PropertyMap{"bucket": resource.NewProperty("b")}
	snap = p.Run(t, snap)

	resA = snap.Resources[len(snap.Resources)-1]
	require.Equal(t, "resA", resA.URN.Name())
	assert.False(t, resA.Inputs["bucket"].IsSecret())
	assert.False(t, resA.Outputs["bucket"].IsSecret())
	// Outputs that are secret for reasons other than the input keep their secret bit.
	assert.True(t, resA.Outputs["arn"].IsSecret())
}

// plaintext models a provider that does not accept secrets and so only ever sees unwrapped values.
func plaintext(v resource.PropertyValue) resource.PropertyValue {
	if v.IsSecret() {
		return v.SecretValue().Element
	}
	return v
}
