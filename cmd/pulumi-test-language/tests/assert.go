// Copyright 2016-2024, Pulumi Corporation.
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

package tests

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func AssertStackResource(t TestingT, err error, changes display.ResourceChanges) (ok bool) {
	t.Helper()

	ok = true
	ok = ok && assert.Nil(t, err, "expected no error, got %v", err)
	ok = ok && assert.NotEmpty(t, changes, "expected at least 1 StepOp")
	ok = ok && assert.NotZero(t, changes[deploy.OpCreate], "expected at least 1 Create")
	return ok
}

func RequireStackResource(t TestingT, err error, changes display.ResourceChanges) {
	t.Helper()

	if !AssertStackResource(t, err, changes) {
		t.FailNow()
	}
}

func RequireSingleResource(t TestingT, resources []*resource.State, typ tokens.Type) *resource.State {
	t.Helper()

	var result *resource.State
	for _, res := range resources {
		if res.Type == typ {
			require.Nil(t, result, "expected exactly 1 resource of type %q, got multiple", typ)
			result = res
		}
	}

	require.NotNil(t, result, "expected exactly 1 resource of type %q, got none", typ)
	return result
}

// RequireSingleNamedResource returns the single resource with the given name from the given list of resources. If more
// than one resource has the given name, the test fails. If no resources have the given name, the test fails.
func RequireSingleNamedResource(
	t TestingT,
	resources []*resource.State,
	name string,
) *resource.State {
	t.Helper()

	var result *resource.State
	for _, res := range resources {
		if res.URN.Name() == name {
			require.Nil(t, result, "expected exactly 1 resource named %q, got multiple", name)
			result = res
		}
	}

	require.NotNil(t, result, "expected exactly 1 resource named %q, got none", name)
	return result
}

// AssertPropertyMapMember asserts that the given property map has a member with the given key and value.
func AssertPropertyMapMember(
	t TestingT,
	props resource.PropertyMap,
	key string,
	want resource.PropertyValue,
) (ok bool) {
	t.Helper()

	got, ok := props[resource.PropertyKey(key)]
	if !assert.True(t, ok, "expected property %q", key) {
		return false
	}

	return assert.Equal(t, want, got, "expected property %q to be %v", key, want)
}

// Like assert.Equal but also permits the actual value to be the JSON-serialized string form of the expected value.
func AssertEqualOrJSONEncoded(l *L, expect any, actual any, msg string) {
	if actualS, ok := actual.(string); ok {
		var a any
		err := json.Unmarshal([]byte(actualS), &a)
		if err == nil {
			if reflect.DeepEqual(expect, a) {
				return
			}
		}
	}
	assert.Equal(l, expect, actual, msg)
}

// Like assert.Equal but also permits secreted JSON-encoded values. The assert succeeds if either:
//
//	actual == expect
//	actual == secret(json(expect2))
//
// The second form expect2 usually has secrets stripped. If nil, the code assumes expect2=expect.
func AssertEqualOrJSONEncodedSecret(l *L, expect, expect2, actual any, msg string) {
	if expect2 == nil {
		expect2 = expect
	}
	if actualObj, ok := actual.(map[string]any); ok {
		tagV, ok := actualObj["4dabf18193072939515e22adb298388d"]
		if ok && tagV == "1b47061264138c4ac30d75fd1eb44270" {
			actualS, ok := actualObj["value"].(string)
			if ok {
				var a any
				err := json.Unmarshal([]byte(actualS), &a)
				if err == nil && reflect.DeepEqual(expect2, a) {
					return
				}
			}
		}
	}
	assert.Equal(l, expect, actual, msg)
}

// Wraps a value in secret sentinel as seen on the RPC wire.
func Secret(x any) any {
	return map[string]any{
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"value":                            x,
	}
}

type AssertNoSecretLeaksOpts struct {
	IgnoreResourceTypes []tokens.Type
	Secrets             []string
}

func (opts *AssertNoSecretLeaksOpts) isIgnored(ty tokens.Type) bool {
	for _, t := range opts.IgnoreResourceTypes {
		if ty == t {
			return true
		}
	}
	return false
}

func AssertNoSecretLeaks(l require.TestingT, snap *deploy.Snapshot, opts AssertNoSecretLeaksOpts) {
	// Remove states for resources with types in opts.IgnoreResourceTypes from the snap to exclude these states from
	// the secret leak checks.
	var filteredResourceStates []*resource.State
	for _, r := range snap.Resources {
		if !opts.isIgnored(r.Type) {
			filteredResourceStates = append(filteredResourceStates, r)
		}
	}
	snap.Resources = filteredResourceStates

	// Ensure that secrets do not leak to the state.
	deployment, err := stack.SerializeDeployment(context.Background(), snap, false /*showSecrets*/)
	require.NoError(l, err)
	bytes, err := json.MarshalIndent(deployment, "", "  ")
	require.NoError(l, err)
	for _, s := range opts.Secrets {
		require.NotContainsf(l, string(bytes), s, "Detected a secret leak in state: %s", s)
	}
}
