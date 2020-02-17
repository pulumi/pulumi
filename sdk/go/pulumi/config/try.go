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

	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"github.com/pulumi/pulumi/sdk/go/pulumi"
)

// Try loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func Try(ctx *pulumi.Context, key string) (string, error) {
	v, ok := ctx.GetConfig(key)
	if !ok {
		return "",
			errors.Errorf("missing required configuration variable '%s'; run `pulumi config` to set", key)
	}
	return v, nil
}

// TryObject loads an optional configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TryObject(ctx *pulumi.Context, key string, output interface{}) error {
	v, err := Try(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(v), output)
}

// TryBool loads an optional configuration value by its key, as a bool, or returns an error if it doesn't exist.
func TryBool(ctx *pulumi.Context, key string) (bool, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return false, err
	}
	return cast.ToBool(v), nil
}

// TryFloat32 loads an optional configuration value by its key, as a float32, or returns an error if it doesn't exist.
func TryFloat32(ctx *pulumi.Context, key string) (float32, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToFloat32(v), nil
}

// TryFloat64 loads an optional configuration value by its key, as a float64, or returns an error if it doesn't exist.
func TryFloat64(ctx *pulumi.Context, key string) (float64, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToFloat64(v), nil
}

// TryInt loads an optional configuration value by its key, as a int, or returns an error if it doesn't exist.
func TryInt(ctx *pulumi.Context, key string) (int, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(v), nil
}

// TryInt16 loads an optional configuration value by its key, as a int16, or returns an error if it doesn't exist.
func TryInt16(ctx *pulumi.Context, key string) (int16, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt16(v), nil
}

// TryInt32 loads an optional configuration value by its key, as a int32, or returns an error if it doesn't exist.
func TryInt32(ctx *pulumi.Context, key string) (int32, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt32(v), nil
}

// TryInt64 loads an optional configuration value by its key, as a int64, or returns an error if it doesn't exist.
func TryInt64(ctx *pulumi.Context, key string) (int64, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt64(v), nil
}

// TryInt8 loads an optional configuration value by its key, as a int8, or returns an error if it doesn't exist.
func TryInt8(ctx *pulumi.Context, key string) (int8, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToInt8(v), nil
}

// TryUint loads an optional configuration value by its key, as a uint, or returns an error if it doesn't exist.
func TryUint(ctx *pulumi.Context, key string) (uint, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint(v), nil
}

// TryUint16 loads an optional configuration value by its key, as a uint16, or returns an error if it doesn't exist.
func TryUint16(ctx *pulumi.Context, key string) (uint16, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint16(v), nil
}

// TryUint32 loads an optional configuration value by its key, as a uint32, or returns an error if it doesn't exist.
func TryUint32(ctx *pulumi.Context, key string) (uint32, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint32(v), nil
}

// TryUint64 loads an optional configuration value by its key, as a uint64, or returns an error if it doesn't exist.
func TryUint64(ctx *pulumi.Context, key string) (uint64, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint64(v), nil
}

// TryUint8 loads an optional configuration value by its key, as a uint8, or returns an error if it doesn't exist.
func TryUint8(ctx *pulumi.Context, key string) (uint8, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return 0, err
	}
	return cast.ToUint8(v), nil
}

// TrySecret loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func TrySecret(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := Try(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.String(v)), nil
}

// TrySecretObject loads a configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TrySecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	err := TryObject(ctx, key, output)
	if err != nil {
		return nil, err
	}

	return pulumi.SecretT(output), nil
}

// TrySecretBool loads an optional configuration value by its key, as a bool, or returns an error if it doesn't exist.
func TrySecretBool(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryBool(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Bool(v)), nil
}

// TrySecretFloat32 loads an optional configuration value by its key, as a float32, or returns an error if it doesn't exist.
func TrySecretFloat32(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryFloat32(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Float32(v)), nil
}

// TrySecretFloat64 loads an optional configuration value by its key, as a float64, or returns an error if it doesn't exist.
func TrySecretFloat64(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryFloat64(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Float64(v)), nil
}

// TrySecretInt loads an optional configuration value by its key, as a int, or returns an error if it doesn't exist.
func TrySecretInt(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryInt(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Int(v)), nil
}

// TrySecretInt16 loads an optional configuration value by its key, as a int16, or returns an error if it doesn't exist.
func TrySecretInt16(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryInt16(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Int16(v)), nil
}

// TrySecretInt32 loads an optional configuration value by its key, as a int32, or returns an error if it doesn't exist.
func TrySecretInt32(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryInt32(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Int32(v)), nil
}

// TrySecretInt64 loads an optional configuration value by its key, as a int64, or returns an error if it doesn't exist.
func TrySecretInt64(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryInt64(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Int64(v)), nil
}

// TrySecretInt8 loads an optional configuration value by its key, as a int8, or returns an error if it doesn't exist.
func TrySecretInt8(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryInt8(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Int8(v)), nil
}

// TrySecretUint loads an optional configuration value by its key, as a uint, or returns an error if it doesn't exist.
func TrySecretUint(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryUint(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Uint(v)), nil
}

// TrySecretUint16 loads an optional configuration value by its key, as a uint16, or returns an error if it doesn't exist.
func TrySecretUint16(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryUint16(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Uint16(v)), nil
}

// TrySecretUint32 loads an optional configuration value by its key, as a uint32, or returns an error if it doesn't exist.
func TrySecretUint32(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryUint32(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Uint32(v)), nil
}

// TrySecretUint64 loads an optional configuration value by its key, as a uint64, or returns an error if it doesn't exist.
func TrySecretUint64(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryUint64(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Uint64(v)), nil
}

// TrySecretUint8 loads an optional configuration value by its key, as a uint8, or returns an error if it doesn't exist.
func TrySecretUint8(ctx *pulumi.Context, key string) (pulumi.Output, error) {
	v, err := TryUint8(ctx, key)
	if err != nil {
		return nil, err
	}
	return pulumi.SecretT(pulumi.Uint8(v)), nil
}
