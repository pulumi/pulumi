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

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func ensureKey(ctx *pulumi.Context, key string) string {
	if !strings.Contains(key, ":") {
		key = ctx.Project() + ":" + key
	}
	return key
}

func get(ctx *pulumi.Context, key, use, insteadOf string) (string, bool) {
	key = ensureKey(ctx, key)
	v, ok := ctx.GetConfig(key)
	// TODO[pulumi/pulumi#7127]: Re-enabled the warning.
	// if use != "" && ctx.IsConfigSecret(key) {
	// 	contract.Assert(insteadOf != "")
	// 	warning := fmt.Sprintf("Configuration '%s' value is a secret; use `%s` instead of `%s`", key, use, insteadOf)
	// 	err := ctx.Log.Warn(warning, nil)
	// 	contract.IgnoreError(err)
	// }
	return v, ok
}

// Get loads an optional configuration value by its key, or returns "" if it doesn't exist.
func Get(ctx *pulumi.Context, key string) string {
	v, _ := get(ctx, key, "GetSecret", "Get")
	return v
}

func getObject(ctx *pulumi.Context, key string, output interface{}, use, insteadOf string) error {
	if v, ok := get(ctx, key, use, insteadOf); ok {
		return json.Unmarshal([]byte(v), output)
	}

	return nil
}

// GetObject attempts to load an optional configuration value by its key into the specified output variable.
func GetObject(ctx *pulumi.Context, key string, output interface{}) error {
	return getObject(ctx, key, output, "GetSecretObject", "GetObject")
}

func getBool(ctx *pulumi.Context, key, use, insteadOf string) bool {
	if v, ok := get(ctx, key, use, insteadOf); ok {
		return cast.ToBool(v)
	}
	return false
}

// GetBool loads an optional configuration value by its key, as a bool, or returns false if it doesn't exist.
func GetBool(ctx *pulumi.Context, key string) bool {
	return getBool(ctx, key, "GetSecretBool", "GetBool")
}

func getFloat64(ctx *pulumi.Context, key, use, insteadOf string) float64 {
	if v, ok := get(ctx, key, use, insteadOf); ok {
		return cast.ToFloat64(v)
	}
	return 0
}

// GetFloat64 loads an optional configuration value by its key, as a float64, or returns 0 if it doesn't exist.
func GetFloat64(ctx *pulumi.Context, key string) float64 {
	return getFloat64(ctx, key, "GetSecretFloat64", "GetFloat64")
}

func getInt(ctx *pulumi.Context, key, use, insteadOf string) int {
	if v, ok := get(ctx, key, use, insteadOf); ok {
		return cast.ToInt(v)
	}
	return 0
}

// GetInt loads an optional configuration value by its key, as a int, or returns 0 if it doesn't exist.
func GetInt(ctx *pulumi.Context, key string) int {
	return getInt(ctx, key, "GetSecretInt", "GetInt")
}

// GetSecret loads an optional configuration value by its key, or "" if it does not exist, into a secret Output.
func GetSecret(ctx *pulumi.Context, key string) pulumi.StringOutput {
	v, _ := get(ctx, key, "", "")
	return pulumi.ToSecret(pulumi.String(v)).(pulumi.StringOutput)
}

// GetSecretObject attempts to load an optional configuration value by its key into the specified output variable.
func GetSecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	if err := getObject(ctx, key, output, "", ""); err != nil {
		return nil, err
	}

	return pulumi.ToSecret(output), nil
}

// GetSecretBool loads an optional bool configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretBool(ctx *pulumi.Context, key string) pulumi.BoolOutput {
	return pulumi.ToSecret(getBool(ctx, key, "", "")).(pulumi.BoolOutput)
}

// GetSecretFloat64 loads an optional float64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretFloat64(ctx *pulumi.Context, key string) pulumi.Float64Output {
	return pulumi.ToSecret(getFloat64(ctx, key, "", "")).(pulumi.Float64Output)
}

// GetSecretInt loads an optional int configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt(ctx *pulumi.Context, key string) pulumi.IntOutput {
	return pulumi.ToSecret(getInt(ctx, key, "", "")).(pulumi.IntOutput)
}
