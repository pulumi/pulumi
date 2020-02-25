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

	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

// Get loads an optional configuration value by its key, or returns "" if it doesn't exist.
func Get(ctx *pulumi.Context, key string) string {
	v, _ := ctx.GetConfig(key)
	return v
}

// GetObject attempts to load an optional configuration value by its key into the specified output variable.
func GetObject(ctx *pulumi.Context, key string, output interface{}) error {
	if v, ok := ctx.GetConfig(key); ok {
		return json.Unmarshal([]byte(v), output)
	}

	return nil
}

// GetBool loads an optional configuration value by its key, as a bool, or returns false if it doesn't exist.
func GetBool(ctx *pulumi.Context, key string) bool {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToBool(v)
	}
	return false
}

// GetFloat32 loads an optional configuration value by its key, as a float32, or returns 0 if it doesn't exist.
func GetFloat32(ctx *pulumi.Context, key string) float32 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToFloat32(v)
	}
	return 0
}

// GetFloat64 loads an optional configuration value by its key, as a float64, or returns 0 if it doesn't exist.
func GetFloat64(ctx *pulumi.Context, key string) float64 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToFloat64(v)
	}
	return 0
}

// GetInt loads an optional configuration value by its key, as a int, or returns 0 if it doesn't exist.
func GetInt(ctx *pulumi.Context, key string) int {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt(v)
	}
	return 0
}

// GetInt16 loads an optional configuration value by its key, as a int16, or returns 0 if it doesn't exist.
func GetInt16(ctx *pulumi.Context, key string) int16 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt16(v)
	}
	return 0
}

// GetInt32 loads an optional configuration value by its key, as a int32, or returns 0 if it doesn't exist.
func GetInt32(ctx *pulumi.Context, key string) int32 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt32(v)
	}
	return 0
}

// GetInt64 loads an optional configuration value by its key, as a int64, or returns 0 if it doesn't exist.
func GetInt64(ctx *pulumi.Context, key string) int64 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt64(v)
	}
	return 0
}

// GetInt8 loads an optional configuration value by its key, as a int8, or returns 0 if it doesn't exist.
func GetInt8(ctx *pulumi.Context, key string) int8 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToInt8(v)
	}
	return 0
}

// GetUint loads an optional configuration value by its key, as a uint, or returns 0 if it doesn't exist.
func GetUint(ctx *pulumi.Context, key string) uint {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint(v)
	}
	return 0
}

// GetUint16 loads an optional configuration value by its key, as a uint16, or returns 0 if it doesn't exist.
func GetUint16(ctx *pulumi.Context, key string) uint16 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint16(v)
	}
	return 0
}

// GetUint32 loads an optional configuration value by its key, as a uint32, or returns 0 if it doesn't exist.
func GetUint32(ctx *pulumi.Context, key string) uint32 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint32(v)
	}
	return 0
}

// GetUint64 loads an optional configuration value by its key, as a uint64, or returns 0 if it doesn't exist.
func GetUint64(ctx *pulumi.Context, key string) uint64 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint64(v)
	}
	return 0
}

// GetUint8 loads an optional configuration value by its key, as a uint8, or returns 0 if it doesn't exist.
func GetUint8(ctx *pulumi.Context, key string) uint8 {
	if v, ok := ctx.GetConfig(key); ok {
		return cast.ToUint8(v)
	}
	return 0
}

// GetSecret loads an optional configuration value by its key, or "" if it does not exist, into a secret Output.
func GetSecret(ctx *pulumi.Context, key string) pulumi.Output {
	v, _ := ctx.GetConfig(key)
	return pulumi.SecretT(pulumi.String(v))
}

// GetSecretObject attempts to load an optional configuration value by its key into the specified output variable.
func GetSecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	if err := GetObject(ctx, key, output); err != nil {
		return nil, err
	}

	return pulumi.SecretT(output), nil
}

// GetSecretBool loads an optional bool configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretBool(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetBool(ctx, key))
}

// GetSecretFloat32 loads an optional float32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretFloat32(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetFloat32(ctx, key))
}

// GetSecretFloat64 loads an optional float64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretFloat64(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetFloat64(ctx, key))
}

// GetSecretInt loads an optional int configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetInt(ctx, key))
}

// GetSecretInt16 loads an optional int16 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt16(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetInt16(ctx, key))
}

// GetSecretInt32 loads an optional int32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt32(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetInt32(ctx, key))
}

// GetSecretInt64 loads an optional int64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt64(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetInt64(ctx, key))
}

// GetSecretInt8 loads an optional int8 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretInt8(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetInt8(ctx, key))
}

// GetSecretUint loads an optional uint configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetUint(ctx, key))
}

// GetSecretUint16 loads an optional uint16 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint16(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetUint16(ctx, key))
}

// GetSecretUint32 loads an optional uint32 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint32(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetUint32(ctx, key))
}

// GetSecretUint64 loads an optional uint64 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint64(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetUint64(ctx, key))
}

// GetSecretUint8 loads an optional uint8 configuration value by its key,
// or false if it does not exist, into a secret Output.
func GetSecretUint8(ctx *pulumi.Context, key string) pulumi.Output {
	return pulumi.SecretT(GetUint8(ctx, key))
}
