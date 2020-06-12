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
	"fmt"

	"github.com/spf13/cast"

	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Try loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func Try(ctx *pulumi.Context, key string) (string, error) {
	key = ensureKey(ctx, key)
	v, ok := ctx.GetConfig(key)
	if !ok {
		return "",
			fmt.Errorf("missing required configuration variable '%s'; run `pulumi config` to set", key)
	}
	return v, nil
}

// TryObject loads an optional configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TryObject(ctx *pulumi.Context, key string, output interface{}) error {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(v), output)
}

// TryBool loads an optional configuration value by its key, as a bool, or returns an error if it doesn't exist.
func TryBool(ctx *pulumi.Context, key string) (bool, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return false, err
	}
	return cast.ToBool(v), nil
}

// TryFloat32 loads an optional configuration value by its key, as a float32, or returns an error if it doesn't exist.
func TryFloat32(ctx *pulumi.Context, key string) (float32, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToFloat32(v), nil
}

// TryFloat64 loads an optional configuration value by its key, as a float64, or returns an error if it doesn't exist.
func TryFloat64(ctx *pulumi.Context, key string) (float64, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToFloat64(v), nil
}

// TryInt loads an optional configuration value by its key, as a int, or returns an error if it doesn't exist.
func TryInt(ctx *pulumi.Context, key string) (int, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(v), nil
}

// TryInt16 loads an optional configuration value by its key, as a int16, or returns an error if it doesn't exist.
func TryInt16(ctx *pulumi.Context, key string) (int16, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt16(v), nil
}

// TryInt32 loads an optional configuration value by its key, as a int32, or returns an error if it doesn't exist.
func TryInt32(ctx *pulumi.Context, key string) (int32, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt32(v), nil
}

// TryInt64 loads an optional configuration value by its key, as a int64, or returns an error if it doesn't exist.
func TryInt64(ctx *pulumi.Context, key string) (int64, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt64(v), nil
}

// TryInt8 loads an optional configuration value by its key, as a int8, or returns an error if it doesn't exist.
func TryInt8(ctx *pulumi.Context, key string) (int8, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt8(v), nil
}

// TryUint loads an optional configuration value by its key, as a uint, or returns an error if it doesn't exist.
func TryUint(ctx *pulumi.Context, key string) (uint, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint(v), nil
}

// TryUint16 loads an optional configuration value by its key, as a uint16, or returns an error if it doesn't exist.
func TryUint16(ctx *pulumi.Context, key string) (uint16, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint16(v), nil
}

// TryUint32 loads an optional configuration value by its key, as a uint32, or returns an error if it doesn't exist.
func TryUint32(ctx *pulumi.Context, key string) (uint32, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint32(v), nil
}

// TryUint64 loads an optional configuration value by its key, as a uint64, or returns an error if it doesn't exist.
func TryUint64(ctx *pulumi.Context, key string) (uint64, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint64(v), nil
}

// TryUint8 loads an optional configuration value by its key, as a uint8, or returns an error if it doesn't exist.
func TryUint8(ctx *pulumi.Context, key string) (uint8, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint8(v), nil
}

// TrySecret loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func TrySecret(ctx *pulumi.Context, key string) (pulumi.StringOutput, error) {
	key = ensureKey(ctx, key)
	v, err := Try(ctx, key)
	if err != nil {
		var empty pulumi.StringOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.String(v)).(pulumi.StringOutput), nil
}

// TrySecretObject loads a configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TrySecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	key = ensureKey(ctx, key)
	err := TryObject(ctx, key, output)
	if err != nil {
		return nil, err
	}

	return pulumi.ToSecret(output), nil
}

// TrySecretBool loads an optional configuration value by its key, as a bool,
// or returns an error if it doesn't exist.
func TrySecretBool(ctx *pulumi.Context, key string) (pulumi.BoolOutput, error) {
	key = ensureKey(ctx, key)
	v, err := TryBool(ctx, key)
	if err != nil {
		var empty pulumi.BoolOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Bool(v)).(pulumi.BoolOutput), nil
}

// TrySecretFloat32 loads an optional configuration value by its key, as a float32,
// or returns an error if it doesn't exist.
func TrySecretFloat32(ctx *pulumi.Context, key string) (pulumi.Float32Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryFloat32(ctx, key)
	if err != nil {
		var empty pulumi.Float32Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Float32(v)).(pulumi.Float32Output), nil
}

// TrySecretFloat64 loads an optional configuration value by its key, as a float64,
// or returns an error if it doesn't exist.
func TrySecretFloat64(ctx *pulumi.Context, key string) (pulumi.Float64Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryFloat64(ctx, key)
	if err != nil {
		var empty pulumi.Float64Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Float64(v)).(pulumi.Float64Output), nil
}

// TrySecretInt loads an optional configuration value by its key, as a int,
// or returns an error if it doesn't exist.
func TrySecretInt(ctx *pulumi.Context, key string) (pulumi.IntOutput, error) {
	key = ensureKey(ctx, key)
	v, err := TryInt(ctx, key)
	if err != nil {
		var empty pulumi.IntOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int(v)).(pulumi.IntOutput), nil
}

// TrySecretInt16 loads an optional configuration value by its key, as a int16,
// or returns an error if it doesn't exist.
func TrySecretInt16(ctx *pulumi.Context, key string) (pulumi.Int16Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryInt16(ctx, key)
	if err != nil {
		var empty pulumi.Int16Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int16(v)).(pulumi.Int16Output), nil
}

// TrySecretInt32 loads an optional configuration value by its key, as a int32,
// or returns an error if it doesn't exist.
func TrySecretInt32(ctx *pulumi.Context, key string) (pulumi.Int32Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryInt32(ctx, key)
	if err != nil {
		var empty pulumi.Int32Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int32(v)).(pulumi.Int32Output), nil
}

// TrySecretInt64 loads an optional configuration value by its key, as a int64,
// or returns an error if it doesn't exist.
func TrySecretInt64(ctx *pulumi.Context, key string) (pulumi.Int64Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryInt64(ctx, key)
	if err != nil {
		var empty pulumi.Int64Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int64(v)).(pulumi.Int64Output), nil
}

// TrySecretInt8 loads an optional configuration value by its key, as a int8,
// or returns an error if it doesn't exist.
func TrySecretInt8(ctx *pulumi.Context, key string) (pulumi.Int8Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryInt8(ctx, key)
	if err != nil {
		var empty pulumi.Int8Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int8(v)).(pulumi.Int8Output), nil
}

// TrySecretUint loads an optional configuration value by its key, as a uint,
// or returns an error if it doesn't exist.
func TrySecretUint(ctx *pulumi.Context, key string) (pulumi.UintOutput, error) {
	key = ensureKey(ctx, key)
	v, err := TryUint(ctx, key)
	if err != nil {
		var empty pulumi.UintOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Uint(v)).(pulumi.UintOutput), nil
}

// TrySecretUint16 loads an optional configuration value by its key, as a uint16,
// or returns an error if it doesn't exist.
func TrySecretUint16(ctx *pulumi.Context, key string) (pulumi.Uint16Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryUint16(ctx, key)
	if err != nil {
		var empty pulumi.Uint16Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Uint16(v)).(pulumi.Uint16Output), nil
}

// TrySecretUint32 loads an optional configuration value by its key, as a uint32,
// or returns an error if it doesn't exist.
func TrySecretUint32(ctx *pulumi.Context, key string) (pulumi.Uint32Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryUint32(ctx, key)
	if err != nil {
		var empty pulumi.Uint32Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Uint32(v)).(pulumi.Uint32Output), nil
}

// TrySecretUint64 loads an optional configuration value by its key, as a uint64,
// or returns an error if it doesn't exist.
func TrySecretUint64(ctx *pulumi.Context, key string) (pulumi.Uint64Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryUint64(ctx, key)
	if err != nil {
		var empty pulumi.Uint64Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Uint64(v)).(pulumi.Uint64Output), nil
}

// TrySecretUint8 loads an optional configuration value by its key, as a uint8,
// or returns an error if it doesn't exist.
func TrySecretUint8(ctx *pulumi.Context, key string) (pulumi.Uint8Output, error) {
	key = ensureKey(ctx, key)
	v, err := TryUint8(ctx, key)
	if err != nil {
		var empty pulumi.Uint8Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Uint8(v)).(pulumi.Uint8Output), nil
}
