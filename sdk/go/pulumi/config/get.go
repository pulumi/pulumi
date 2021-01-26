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
	"strings"

	"github.com/spf13/cast"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

func ensureKey(ctx *pulumi.Context, key string) string {
	if !strings.Contains(key, ":") {
		key = ctx.Project() + ":" + key
	}
	return key
}

// Get loads an optional configuration value by its key, or returns "" if it doesn't exist.
func Get(ctx *pulumi.Context, key string) string {
	key = ensureKey(ctx, key)
	v, _ := ctx.GetConfig(key)
	return v
}

// GetObject attempts to load an optional configuration value by its key into the specified output variable.
func GetObject(ctx *pulumi.Context, key string, output interface{}) error {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return json.Unmarshal([]byte(v), output)
	}

	return nil
}

// GetBool loads an optional configuration value by its key, as a bool, or returns false if it doesn't exist.
func GetBool(ctx *pulumi.Context, key string) bool {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToBool(v)
	}
	return false
}

// GetFloat64 loads an optional configuration value by its key, as a float64, or returns 0 if it doesn't exist.
func GetFloat64(ctx *pulumi.Context, key string) float64 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToFloat64(v)
	}
	return 0
}

// GetInt loads an optional configuration value by its key, as a int, or returns 0 if it doesn't exist.
func GetInt(ctx *pulumi.Context, key string) int {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt(v)
	}
	return 0
}

// GetSecret loads an optional configuration value by its key, or "" if it does not exist, into a secret Output.
func GetSecret(ctx *pulumi.Context, key string) pulumi.StringOutput {
	key = ensureKey(ctx, key)
	v, _ := ctx.GetConfig(key)
	return pulumi.ToSecret(pulumi.String(v)).(pulumi.StringOutput)
}

// GetSecretObject attempts to load an optional configuration value by its key into the specified output variable.
func GetSecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	key = ensureKey(ctx, key)
	if err := GetObject(ctx, key, output); err != nil {
		return nil, err
	}

	return pulumi.ToSecret(output), nil
}

// GetSecretBool loads an optional bool configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretBool(ctx *pulumi.Context, key string) pulumi.BoolOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetBool(ctx, key)).(pulumi.BoolOutput)
}

// GetSecretFloat64 loads an optional float64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretFloat64(ctx *pulumi.Context, key string) pulumi.Float64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetFloat64(ctx, key)).(pulumi.Float64Output)
}

// GetSecretInt loads an optional int configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt(ctx *pulumi.Context, key string) pulumi.IntOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetInt(ctx, key)).(pulumi.IntOutput)
}
