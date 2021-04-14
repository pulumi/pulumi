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
	"encoding/json"

	"github.com/spf13/cast"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Require loads a configuration value by its key, or panics if it doesn't exist.
func Require(ctx *pulumi.Context, key string) string {
	key = ensureKey(ctx, key)
	v, ok := ctx.GetConfig(key)
	if !ok {
		contract.Failf("missing required configuration variable '%s'; run `pulumi config` to set", key)
	}
	return v
}

// RequireObject loads an optional configuration value by its key into the output variable,
// or panics if unable to do so.
func RequireObject(ctx *pulumi.Context, key string, output interface{}) {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	if err := json.Unmarshal([]byte(v), output); err != nil {
		contract.Failf("unable to unmarshall required configuration variable '%s'; %s", key, err.Error())
	}
}

// RequireBool loads an optional configuration value by its key, as a bool, or panics if it doesn't exist.
func RequireBool(ctx *pulumi.Context, key string) bool {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToBool(v)
}

// RequireFloat64 loads an optional configuration value by its key, as a float64, or panics if it doesn't exist.
func RequireFloat64(ctx *pulumi.Context, key string) float64 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToFloat64(v)
}

// RequireInt loads an optional configuration value by its key, as a int, or panics if it doesn't exist.
func RequireInt(ctx *pulumi.Context, key string) int {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToInt(v)
}

// RequireSecret loads a configuration value by its key returning it wrapped in a secret Output,
// or panics if it doesn't exist.
func RequireSecret(ctx *pulumi.Context, key string) pulumi.StringOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(Require(ctx, key)).(pulumi.StringOutput)
}

// RequireSecretObject loads an optional configuration value by its key into the output variable,
// returning it wrapped in a secret Output, or panics if unable to do so.
func RequireSecretObject(ctx *pulumi.Context, key string, output interface{}) pulumi.Output {
	key = ensureKey(ctx, key)
	RequireObject(ctx, key, output)
	return pulumi.ToSecret(output)
}

// RequireSecretBool loads an optional configuration value by its key,
// as a bool wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretBool(ctx *pulumi.Context, key string) pulumi.BoolOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireBool(ctx, key)).(pulumi.BoolOutput)
}

// RequireSecretFloat64 loads an optional configuration value by its key,
// as a float64 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretFloat64(ctx *pulumi.Context, key string) pulumi.Float64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireFloat64(ctx, key)).(pulumi.Float64Output)
}

// RequireSecretInt loads an optional configuration value by its key,
// as a int wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretInt(ctx *pulumi.Context, key string) pulumi.IntOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireInt(ctx, key)).(pulumi.IntOutput)
}
