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
	"maps"
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

	assert.Equal(t, map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}, maps.Collect(store.Values()))
}

func TestMapStoreValuesEmpty(t *testing.T) {
	t.Parallel()
	store := env.MapStore{}

	assert.Empty(t, maps.Collect(store.Values()))
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
		},
		{
			name:          "no stores returns an empty map",
			stores:        []env.Store{},
			wantCollected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			joined := env.JoinStore(tt.stores...)
			assert.Equal(t, tt.wantCollected, maps.Collect(joined.Values()))
		})
	}
}

func TestJoinStoreValuesReturnsEarlyWhenKeyFound(t *testing.T) {
	t.Parallel()
	stores := []env.Store{
		env.MapStore{"KEY1": "value1", "KEY2": "value2", "KEY3": "value3"},
		env.MapStore{"KEY4": "value4"},
	}
	joined := env.JoinStore(stores...)
	count := 0
	for k := range joined.Values() {
		count++
		if k == "KEY2" {
			break
		}
	}
	assert.GreaterOrEqual(t, count, 1)
	assert.LessOrEqual(t, count, 2)
}

func TestJoinStoreRaw(t *testing.T) {
	t.Parallel()

	store1 := env.MapStore{"KEY1": "value1_from_store1", "KEY2": "value2_from_store1"}
	store2 := env.MapStore{"KEY2": "value2_from_store2", "KEY3": "value3_from_store2"}
	store3 := env.MapStore{"KEY4": "value4_from_store3"}

	tests := []struct {
		name      string
		stores    []env.Store
		key       string
		wantValue string
		wantOk    bool
	}{
		{
			name:      "gets value from first store",
			stores:    []env.Store{store1, store2, store3},
			key:       "KEY1",
			wantValue: "value1_from_store1",
			wantOk:    true,
		},
		{
			name:      "gets value from first store when key exists in multiple stores",
			stores:    []env.Store{store1, store2, store3},
			key:       "KEY2",
			wantValue: "value2_from_store1",
			wantOk:    true,
		},
		{
			name:      "gets value from later store when not in first",
			stores:    []env.Store{store1, store2, store3},
			key:       "KEY3",
			wantValue: "value3_from_store2",
			wantOk:    true,
		},
		{
			name:      "gets value from last store",
			stores:    []env.Store{store1, store2, store3},
			key:       "KEY4",
			wantValue: "value4_from_store3",
			wantOk:    true,
		},
		{
			name:      "returns false for non-existent key",
			stores:    []env.Store{store1, store2, store3},
			key:       "NONEXISTENT",
			wantValue: "",
			wantOk:    false,
		},
		{
			name:      "returns false when stores are empty",
			stores:    []env.Store{},
			key:       "ANY_KEY",
			wantValue: "",
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			joined := env.JoinStore(tt.stores...)
			val, ok := joined.Raw(tt.key)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantValue, val)
		})
	}
}

func TestEnvStoreValues(t *testing.T) {
	t.Parallel()

	testKey1 := "PULUMI_TEST_ENV_STORE_KEY1"
	testKey2 := "PULUMI_TEST_ENV_STORE_KEY2"
	testValue1 := "test_value_1"
	testValue2 := "test_value_2"

	// Set test environment variables
	t.Setenv(testKey1, testValue1)
	t.Setenv(testKey2, testValue2)

	store := env.NewEnvStore()

	// Collect all values from the store
	allValues := maps.Collect(store.Values())

	// Verify our test keys are present
	assert.Equal(t, testValue1, allValues[testKey1])
	assert.Equal(t, testValue2, allValues[testKey2])

	// Verify that the store can retrieve values via Raw
	val, ok := store.Raw(testKey1)
	assert.True(t, ok)
	assert.Equal(t, testValue1, val)

	val, ok = store.Raw(testKey2)
	assert.True(t, ok)
	assert.Equal(t, testValue2, val)
}
