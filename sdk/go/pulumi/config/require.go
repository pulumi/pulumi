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

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
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

// RequireFloat32 loads an optional configuration value by its key, as a float32, or panics if it doesn't exist.
func RequireFloat32(ctx *pulumi.Context, key string) float32 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToFloat32(v)
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

// RequireInt16 loads an optional configuration value by its key, as a int16, or panics if it doesn't exist.
func RequireInt16(ctx *pulumi.Context, key string) int16 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToInt16(v)
}

// RequireInt32 loads an optional configuration value by its key, as a int32, or panics if it doesn't exist.
func RequireInt32(ctx *pulumi.Context, key string) int32 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToInt32(v)
}

// RequireInt64 loads an optional configuration value by its key, as a int64, or panics if it doesn't exist.
func RequireInt64(ctx *pulumi.Context, key string) int64 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToInt64(v)
}

// RequireInt8 loads an optional configuration value by its key, as a int8, or panics if it doesn't exist.
func RequireInt8(ctx *pulumi.Context, key string) int8 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToInt8(v)
}

// RequireUint loads an optional configuration value by its key, as a uint, or panics if it doesn't exist.
func RequireUint(ctx *pulumi.Context, key string) uint {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToUint(v)
}

// RequireUint16 loads an optional configuration value by its key, as a uint16, or panics if it doesn't exist.
func RequireUint16(ctx *pulumi.Context, key string) uint16 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToUint16(v)
}

// RequireUint32 loads an optional configuration value by its key, as a uint32, or panics if it doesn't exist.
func RequireUint32(ctx *pulumi.Context, key string) uint32 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToUint32(v)
}

// RequireUint64 loads an optional configuration value by its key, as a uint64, or panics if it doesn't exist.
func RequireUint64(ctx *pulumi.Context, key string) uint64 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToUint64(v)
}

// RequireUint8 loads an optional configuration value by its key, as a uint8, or panics if it doesn't exist.
func RequireUint8(ctx *pulumi.Context, key string) uint8 {
	key = ensureKey(ctx, key)
	v := Require(ctx, key)
	return cast.ToUint8(v)
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

// RequireSecretFloat32 loads an optional configuration value by its key,
// as a float32 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretFloat32(ctx *pulumi.Context, key string) pulumi.Float32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireFloat32(ctx, key)).(pulumi.Float32Output)
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

// RequireSecretInt16 loads an optional configuration value by its key,
// as a int16 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretInt16(ctx *pulumi.Context, key string) pulumi.Int16Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireInt16(ctx, key)).(pulumi.Int16Output)
}

// RequireSecretInt32 loads an optional configuration value by its key,
// as a int32 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretInt32(ctx *pulumi.Context, key string) pulumi.Int32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireInt32(ctx, key)).(pulumi.Int32Output)
}

// RequireSecretInt64 loads an optional configuration value by its key,
// as a int64 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretInt64(ctx *pulumi.Context, key string) pulumi.Int64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireInt64(ctx, key)).(pulumi.Int64Output)
}

// RequireSecretInt8 loads an optional configuration value by its key,
// as a int8 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretInt8(ctx *pulumi.Context, key string) pulumi.Int8Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireInt8(ctx, key)).(pulumi.Int8Output)
}

// RequireSecretUint loads an optional configuration value by its key,
// as a uint wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretUint(ctx *pulumi.Context, key string) pulumi.UintOutput {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireUint(ctx, key)).(pulumi.UintOutput)
}

// RequireSecretUint16 loads an optional configuration value by its key,
// as a uint16 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretUint16(ctx *pulumi.Context, key string) pulumi.Uint16Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireUint16(ctx, key)).(pulumi.Uint16Output)
}

// RequireSecretUint32 loads an optional configuration value by its key,
// as a uint32 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretUint32(ctx *pulumi.Context, key string) pulumi.Uint32Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireUint32(ctx, key)).(pulumi.Uint32Output)
}

// RequireSecretUint64 loads an optional configuration value by its key,
// as a uint64 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretUint64(ctx *pulumi.Context, key string) pulumi.Uint64Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireUint64(ctx, key)).(pulumi.Uint64Output)
}

// RequireSecretUint8 loads an optional configuration value by its key,
// as a uint8 wrapped in a secret Output, or panics if it doesn't exist.
func RequireSecretUint8(ctx *pulumi.Context, key string) pulumi.Uint8Output {
	key = ensureKey(ctx, key)
	return pulumi.ToSecret(RequireUint8(ctx, key)).(pulumi.Uint8Output)
}
