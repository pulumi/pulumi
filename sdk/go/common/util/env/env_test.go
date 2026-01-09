// Copyright 2022-2024, Pulumi Corporation.
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

package env_test

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/assert"
)

func init() {
	env.Global = env.MapStore{
		"PULUMI_FOO": "1",
		// "PULUMI_NOT_SET": explicitly not set
		"FOO":                "bar",
		"PULUMI_MY_INT":      "3",
		"PULUMI_SECRET":      "hidden",
		"PULUMI_SET":         "SET",
		"UNSET":              "SET",
		"PULUMI_ALTERNATIVE": "SET",
	}
}

var (
	SomeBool    = env.Bool("FOO", "A bool used for testing")
	SomeFalse   = env.Bool("NOT_SET", "a falsy value")
	SomeString  = env.String("FOO", "A bool used for testing", env.NoPrefix)
	SomeSecret  = env.String("SECRET", "A secret that shouldn't be displayed", env.Secret)
	UnsetString = env.String("PULUMI_UNSET", "Should be unset", env.Needs(SomeFalse))
	SetString   = env.String("SET", "Should be set", env.Needs(SomeBool))
	AnInt       = env.Int("MY_INT", "Should be 3")
	Alternative = env.String("NOT_ALTERNATIVE", "Should be set with alt name", env.Alternative("ALTERNATIVE"))
)

func TestInt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 3, AnInt.Value())
	assert.Equal(t, 3, env.NewEnv(env.Global).GetInt(AnInt))
	assert.Equal(t, 6, env.NewEnv(
		env.MapStore{"PULUMI_MY_INT": "6"},
	).GetInt(AnInt))
}

func TestBool(t *testing.T) {
	t.Parallel()
	assert.Equal(t, true, SomeBool.Value())
}

func TestString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "bar", SomeString.Value())
}

func TestSecret(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hidden", SomeSecret.Value())
	assert.Equal(t, "[secret]", SomeSecret.String())
}

func TestNeeds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", UnsetString.Value())
	assert.Equal(t, "SET", SetString.Value())
}

func TestAlternative(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "SET", Alternative.Value())
}

func TestMapStoreValues(t *testing.T) {
	t.Parallel()
	store := env.MapStore{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}

	collected := make(map[string]string)
	for k, v := range store.Values() {
		collected[k] = v
	}

	assert.Equal(t, "value1", collected["KEY1"])
	assert.Equal(t, "value2", collected["KEY2"])
	assert.Equal(t, "value3", collected["KEY3"])
	nonexistentValue, ok := collected["KEY_NOT_EXIST"]
	assert.Equal(t, "", nonexistentValue)
	assert.False(t, ok)
	assert.Len(t, collected, 3)
}

func TestMapStoreValuesEmpty(t *testing.T) {
	t.Parallel()
	store := env.MapStore{}

	collected := make(map[string]string)
	for k, v := range store.Values() {
		collected[k] = v
	}

	assert.Empty(t, collected)
}

func TestGetStoreForEnv(t *testing.T) {
	t.Parallel()
	store := env.MapStore{
		"TEST_KEY": "test_value",
	}
	e := env.NewEnv(store)

	retrievedStore := e.GetStore()
	assert.Equal(t, store, retrievedStore)

	val, ok := retrievedStore.Raw("TEST_KEY")
	assert.True(t, ok)
	assert.Equal(t, "test_value", val)
}

func TestJoinStoreValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		stores        []env.Store
		wantCollected map[string]string
		wantLen       int
	}{
		{
			name: "multiple stores with duplicate keys use value of first key",
			stores: []env.Store{
				env.MapStore{"KEY1": "value1", "KEY2": "value2"},
				env.MapStore{"KEY2": "ignored_due_to_key_exists", "KEY3": "value3"},
				env.MapStore{"KEY4": "value4"},
			},
			wantCollected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
				"KEY4": "value4",
			},
			wantLen: 4,
		},
		{
			name: "empty stores in chain do not affect the result",
			stores: []env.Store{
				env.MapStore{"KEY1": "value1"},
				env.MapStore{},
				env.MapStore{"KEY2": "value2"},
			},
			wantCollected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantLen: 2,
		},
		{
			name: "single store returns all its values",
			stores: []env.Store{
				env.MapStore{"KEY1": "value1", "KEY2": "value2"},
			},
			wantCollected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
			},
			wantLen: 2,
		},
		{
			name:          "no stores returns an empty map",
			stores:        []env.Store{},
			wantCollected: map[string]string{},
			wantLen:       0,
		},
		{
			name: "returns early from iteration when a key is found",
			stores: []env.Store{
				env.MapStore{"KEY1": "value1", "KEY2": "value2", "KEY3": "value3"},
				env.MapStore{"KEY4": "value4"},
			},
			wantCollected: nil,
			wantLen:       0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			joined := env.JoinStore(tt.stores...)

			if tt.name == "returns early from iteration when a key is found" {
				count := 0
				for k := range joined.Values() {
					count++
					if k == "KEY2" {
						break
					}
				}
				assert.GreaterOrEqual(t, count, 1)
				assert.LessOrEqual(t, count, 2)
			} else {
				collected := make(map[string]string)
				for k, v := range joined.Values() {
					collected[k] = v
				}
				assert.Equal(t, tt.wantLen, len(collected))
				for key, wantValue := range tt.wantCollected {
					assert.Equal(t, wantValue, collected[key], "key %s", key)
				}
			}
		})
	}
}
