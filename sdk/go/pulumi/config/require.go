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

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

// Require loads a configuration value by its key, or panics if it doesn't exist.
func Require(ctx *pulumi.Context, key string) string {
	v, ok := ctx.GetConfig(key)
	if !ok {
		contract.Failf("missing required configuration variable '%s'; run `pulumi config` to set", key)
	}
	return v
}

// RequireBool loads an optional configuration value by its key, as a bool, or panics if it doesn't exist.
func RequireBool(ctx *pulumi.Context, key string) bool {
	v := Require(ctx, key)
	return cast.ToBool(v)
}

// RequireFloat32 loads an optional configuration value by its key, as a float32, or panics if it doesn't exist.
func RequireFloat32(ctx *pulumi.Context, key string) float32 {
	v := Require(ctx, key)
	return cast.ToFloat32(v)
}

// RequireFloat64 loads an optional configuration value by its key, as a float64, or panics if it doesn't exist.
func RequireFloat64(ctx *pulumi.Context, key string) float64 {
	v := Require(ctx, key)
	return cast.ToFloat64(v)
}

// RequireInt loads an optional configuration value by its key, as a int, or panics if it doesn't exist.
func RequireInt(ctx *pulumi.Context, key string) int {
	v := Require(ctx, key)
	return cast.ToInt(v)
}

// RequireInt8 loads an optional configuration value by its key, as a int8, or panics if it doesn't exist.
func RequireInt8(ctx *pulumi.Context, key string) int8 {
	v := Require(ctx, key)
	return cast.ToInt8(v)
}

// RequireInt16 loads an optional configuration value by its key, as a int16, or panics if it doesn't exist.
func RequireInt16(ctx *pulumi.Context, key string) int16 {
	v := Require(ctx, key)
	return cast.ToInt16(v)
}

// RequireInt32 loads an optional configuration value by its key, as a int32, or panics if it doesn't exist.
func RequireInt32(ctx *pulumi.Context, key string) int32 {
	v := Require(ctx, key)
	return cast.ToInt32(v)
}

// RequireInt64 loads an optional configuration value by its key, as a int64, or panics if it doesn't exist.
func RequireInt64(ctx *pulumi.Context, key string) int64 {
	v := Require(ctx, key)
	return cast.ToInt64(v)
}

// RequireObject loads an optional configuration value by its key into the output variable,
// or panics if unable to do so.
func RequireObject(ctx *pulumi.Context, key string, output interface{}) {
	v := Require(ctx, key)
	if err := json.Unmarshal([]byte(v), output); err != nil {
		contract.Failf("unable to unmarshall required configuration variable '%s'; %s", key, err.Error())
	}
}

// RequireUint loads an optional configuration value by its key, as a uint, or panics if it doesn't exist.
func RequireUint(ctx *pulumi.Context, key string) uint {
	v := Require(ctx, key)
	return cast.ToUint(v)
}

// RequireUint8 loads an optional configuration value by its key, as a uint8, or panics if it doesn't exist.
func RequireUint8(ctx *pulumi.Context, key string) uint8 {
	v := Require(ctx, key)
	return cast.ToUint8(v)
}

// RequireUint16 loads an optional configuration value by its key, as a uint16, or panics if it doesn't exist.
func RequireUint16(ctx *pulumi.Context, key string) uint16 {
	v := Require(ctx, key)
	return cast.ToUint16(v)
}

// RequireUint32 loads an optional configuration value by its key, as a uint32, or panics if it doesn't exist.
func RequireUint32(ctx *pulumi.Context, key string) uint32 {
	v := Require(ctx, key)
	return cast.ToUint32(v)
}

// RequireUint64 loads an optional configuration value by its key, as a uint64, or panics if it doesn't exist.
func RequireUint64(ctx *pulumi.Context, key string) uint64 {
	v := Require(ctx, key)
	return cast.ToUint64(v)
}
