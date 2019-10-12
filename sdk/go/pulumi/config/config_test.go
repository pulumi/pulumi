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

// TestConfig tests the basic config wrapper.
func TestConfig(t *testing.T) {
	t.Parallel()
	ctx, err := pulumi.NewContext(context.Background(), pulumi.RunInfo{
		Config: map[string]string{
			"testpkg:sss":    "a string value",
			"testpkg:bbb":    "true",
			"testpkg:intint": "42",
			"testpkg:fpfpfp": "99.963",
		},
	})
	assert.Nil(t, err)

	cfg := New(ctx, "testpkg")

	// Test basic keys.
	assert.Equal(t, "testpkg:sss", cfg.fullKey("sss"))

	// Test Get, which returns a default value for missing entries rather than failing.
	assert.Equal(t, "a string value", cfg.Get("sss"))
	assert.Equal(t, true, cfg.GetBool("bbb"))
	assert.Equal(t, 42, cfg.GetInt("intint"))
	assert.Equal(t, 99.963, cfg.GetFloat64("fpfpfp"))
	assert.Equal(t, "", cfg.Get("missing"))

	// Test Require, which panics for missing entries.
	assert.Equal(t, "a string value", cfg.Require("sss"))
	assert.Equal(t, true, cfg.RequireBool("bbb"))
	assert.Equal(t, 42, cfg.RequireInt("intint"))
	assert.Equal(t, 99.963, cfg.RequireFloat64("fpfpfp"))
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
	_, err = cfg.Try("missing")
	assert.NotNil(t, err)
}
