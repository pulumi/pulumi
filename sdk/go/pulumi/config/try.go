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

	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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
