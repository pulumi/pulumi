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
