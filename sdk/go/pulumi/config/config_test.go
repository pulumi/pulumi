// Copyright 2016-2018, Pulumi Corporation.
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

package config

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

type TestStruct struct {
	Foo map[string]string
	Bar string
}

// TestConfig tests the basic config wrapper.
func TestConfig(t *testing.T) {
	ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
		Config: map[string]string{
			"testpkg:sss":    "a string value",
			"testpkg:bbb":    "true",
			"testpkg:intint": "42",
			"testpkg:fpfpfp": "99.963",
			"testpkg:obj": `
				{
					"foo": {
						"a": "1",
						"b": "2"
					},
					"bar": "abc"
				}
			`,
			"testpkg:malobj": "not_a_struct",
		},
	})
	assert.Nil(t, err)

	cfg := New(ctx, "testpkg")

	var testStruct TestStruct
	var emptyTestStruct TestStruct

	fooMap := make(map[string]string)
	fooMap["a"] = "1"
	fooMap["b"] = "2"
	expectedTestStruct := TestStruct{
		Foo: fooMap,
		Bar: "abc",
	}
	clearTestStruct := func(ts *TestStruct) {
		*ts = TestStruct{}
	}

	// Test basic keys.
	assert.Equal(t, "testpkg:sss", cfg.fullKey("sss"))

	// Test Get, which returns a default value for missing entries rather than failing.
	assert.Equal(t, "a string value", cfg.Get("sss"))
	assert.Equal(t, true, cfg.GetBool("bbb"))
	assert.Equal(t, 42, cfg.GetInt("intint"))
	assert.Equal(t, 99.963, cfg.GetFloat64("fpfpfp"))
	assert.Equal(t, "", cfg.Get("missing"))
	// missing key GetObj
	err = cfg.GetObject("missing", &testStruct)
	assert.Equal(t, emptyTestStruct, testStruct)
	assert.Nil(t, err)
	clearTestStruct(&testStruct)
	// malformed key GetObj
	err = cfg.GetObject("malobj", &testStruct)
	assert.Equal(t, emptyTestStruct, testStruct)
	assert.NotNil(t, err)
	clearTestStruct(&testStruct)
	// GetObj
	err = cfg.GetObject("obj", &testStruct)
	assert.Equal(t, expectedTestStruct, testStruct)
	assert.Nil(t, err)
	clearTestStruct(&testStruct)

	// Test Require, which panics for missing entries.
	assert.Equal(t, "a string value", cfg.Require("sss"))
	assert.Equal(t, true, cfg.RequireBool("bbb"))
	assert.Equal(t, 42, cfg.RequireInt("intint"))
	assert.Equal(t, 99.963, cfg.RequireFloat64("fpfpfp"))
	cfg.RequireObject("obj", &testStruct)
	assert.Equal(t, expectedTestStruct, testStruct)
	clearTestStruct(&testStruct)
	// GetObj panics if value is malformed
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected malformed value for RequireObject to panic")
			}
		}()
		cfg.RequireObject("malobj", &testStruct)
	}()
	clearTestStruct(&testStruct)
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected missing key for Require to panic")
			}
		}()
		_ = cfg.Require("missing")
	}()

	// Test Try, which returns an error for missing entries.
	k1, err := cfg.Try("sss")
	assert.Nil(t, err)
	assert.Equal(t, "a string value", k1)
	k2, err := cfg.TryBool("bbb")
	assert.Nil(t, err)
	assert.Equal(t, true, k2)
	k3, err := cfg.TryInt("intint")
	assert.Nil(t, err)
	assert.Equal(t, 42, k3)
	k4, err := cfg.TryFloat64("fpfpfp")
	assert.Nil(t, err)
	assert.Equal(t, 99.963, k4)
	// happy path TryObject
	err = cfg.TryObject("obj", &testStruct)
	assert.Nil(t, err)
	assert.Equal(t, expectedTestStruct, testStruct)
	clearTestStruct(&testStruct)
	// missing TryObject
	err = cfg.TryObject("missing", &testStruct)
	assert.NotNil(t, err)
	assert.Equal(t, emptyTestStruct, testStruct)
	clearTestStruct(&testStruct)
	// malformed TryObject
	err = cfg.TryObject("malobj", &testStruct)
	assert.NotNil(t, err)
	assert.Equal(t, emptyTestStruct, testStruct)
	clearTestStruct(&testStruct)
	_, err = cfg.Try("missing")
	assert.NotNil(t, err)
}
