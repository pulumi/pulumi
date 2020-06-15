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

// GetFloat32 loads an optional configuration value by its key, as a float32, or returns 0 if it doesn't exist.
func GetFloat32(ctx *pulumi.Context, key string) float32 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToFloat32(v)
	}
	return 0
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

// GetInt16 loads an optional configuration value by its key, as a int16, or returns 0 if it doesn't exist.
func GetInt16(ctx *pulumi.Context, key string) int16 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt16(v)
	}
	return 0
}

// GetInt32 loads an optional configuration value by its key, as a int32, or returns 0 if it doesn't exist.
func GetInt32(ctx *pulumi.Context, key string) int32 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt32(v)
	}
	return 0
}

// GetInt64 loads an optional configuration value by its key, as a int64, or returns 0 if it doesn't exist.
func GetInt64(ctx *pulumi.Context, key string) int64 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt64(v)
	}
	return 0
}

// GetInt8 loads an optional configuration value by its key, as a int8, or returns 0 if it doesn't exist.
func GetInt8(ctx *pulumi.Context, key string) int8 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt8(v)
	}
	return 0
}

// GetUint loads an optional configuration value by its key, as a uint, or returns 0 if it doesn't exist.
func GetUint(ctx *pulumi.Context, key string) uint {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint(v)
	}
	return 0
}

// GetUint16 loads an optional configuration value by its key, as a uint16, or returns 0 if it doesn't exist.
func GetUint16(ctx *pulumi.Context, key string) uint16 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint16(v)
	}
	return 0
}

// GetUint32 loads an optional configuration value by its key, as a uint32, or returns 0 if it doesn't exist.
func GetUint32(ctx *pulumi.Context, key string) uint32 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint32(v)
	}
	return 0
}

// GetUint64 loads an optional configuration value by its key, as a uint64, or returns 0 if it doesn't exist.
func GetUint64(ctx *pulumi.Context, key string) uint64 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint64(v)
	}
	return 0
}

// GetUint8 loads an optional configuration value by its key, as a uint8, or returns 0 if it doesn't exist.
func GetUint8(ctx *pulumi.Context, key string) uint8 {
	key = ensureKey(ctx, key)
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint8(v)
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

// GetSecretFloat32 loads an optional float32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretFloat32(ctx *pulumi.Context, key string) pulumi.Float32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetFloat32(ctx, key)).(pulumi.Float32Output)
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

// GetSecretInt16 loads an optional int16 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt16(ctx *pulumi.Context, key string) pulumi.Int16Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetInt16(ctx, key)).(pulumi.Int16Output)
}

// GetSecretInt32 loads an optional int32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt32(ctx *pulumi.Context, key string) pulumi.Int32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetInt32(ctx, key)).(pulumi.Int32Output)
}

// GetSecretInt64 loads an optional int64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt64(ctx *pulumi.Context, key string) pulumi.Int64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetInt64(ctx, key)).(pulumi.Int64Output)
}

// GetSecretInt8 loads an optional int8 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt8(ctx *pulumi.Context, key string) pulumi.Int8Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetInt8(ctx, key)).(pulumi.Int8Output)
}

// GetSecretUint loads an optional uint configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint(ctx *pulumi.Context, key string) pulumi.UintOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetUint(ctx, key)).(pulumi.UintOutput)
}

// GetSecretUint16 loads an optional uint16 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint16(ctx *pulumi.Context, key string) pulumi.Uint16Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetUint16(ctx, key)).(pulumi.Uint16Output)
}

// GetSecretUint32 loads an optional uint32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint32(ctx *pulumi.Context, key string) pulumi.Uint32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetUint32(ctx, key)).(pulumi.Uint32Output)
}

// GetSecretUint64 loads an optional uint64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint64(ctx *pulumi.Context, key string) pulumi.Uint64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetUint64(ctx, key)).(pulumi.Uint64Output)
}

// GetSecretUint8 loads an optional uint8 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint8(ctx *pulumi.Context, key string) pulumi.Uint8Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(GetUint8(ctx, key)).(pulumi.Uint8Output)
}
