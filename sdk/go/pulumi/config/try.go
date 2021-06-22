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

func try(ctx *pulumi.Context, key, use, insteadOf string) (string, error) {
	v, ok := get(ctx, key, use, insteadOf)
	if !ok {
		return "",
			fmt.Errorf("missing required configuration variable '%s'; run `pulumi config` to set", key)
	}
	return v, nil
}

// Try loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func Try(ctx *pulumi.Context, key string) (string, error) {
	return try(ctx, key, "TrySecret", "Try")
}

func tryObject(ctx *pulumi.Context, key string, output interface{}, use, insteadOf string) error {
	v, err := try(ctx, key, use, insteadOf)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(v), output)
}

// TryObject loads an optional configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TryObject(ctx *pulumi.Context, key string, output interface{}) error {
	return tryObject(ctx, key, output, "TrySecretObject", "TryObject")
}

func tryBool(ctx *pulumi.Context, key, use, insteadOf string) (bool, error) {
	v, err := try(ctx, key, use, insteadOf)
	if err != nil {
		return false, err
	}
	return cast.ToBool(v), nil
}

// TryBool loads an optional configuration value by its key, as a bool, or returns an error if it doesn't exist.
func TryBool(ctx *pulumi.Context, key string) (bool, error) {
	return tryBool(ctx, key, "TrySecretBool", "TryBool")
}

func tryFloat64(ctx *pulumi.Context, key, use, insteadOf string) (float64, error) {
	v, err := try(ctx, key, use, insteadOf)
	if err != nil {
		return 0, err
	}
	return cast.ToFloat64(v), nil
}

// TryFloat64 loads an optional configuration value by its key, as a float64, or returns an error if it doesn't exist.
func TryFloat64(ctx *pulumi.Context, key string) (float64, error) {
	return tryFloat64(ctx, key, "TrySecretFloat64", "TryFloat64")
}

func tryInt(ctx *pulumi.Context, key, use, insteadOf string) (int, error) {
	v, err := try(ctx, key, use, insteadOf)
	if err != nil {
		return 0, err
	}
	return cast.ToInt(v), nil
}

// TryInt loads an optional configuration value by its key, as a int, or returns an error if it doesn't exist.
func TryInt(ctx *pulumi.Context, key string) (int, error) {
	return tryInt(ctx, key, "TrySecretInt", "TryInt")
}

// TrySecret loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func TrySecret(ctx *pulumi.Context, key string) (pulumi.StringOutput, error) {
	v, err := try(ctx, key, "", "")
	if err != nil {
		var empty pulumi.StringOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.String(v)).(pulumi.StringOutput), nil
}

// TrySecretObject loads a configuration value by its key into the output variable,
// or returns an error if unable to do so.
func TrySecretObject(ctx *pulumi.Context, key string, output interface{}) (pulumi.Output, error) {
	err := tryObject(ctx, key, output, "", "")
	if err != nil {
		return nil, err
	}

	return pulumi.ToSecret(output), nil
}

// TrySecretBool loads an optional configuration value by its key, as a bool,
// or returns an error if it doesn't exist.
func TrySecretBool(ctx *pulumi.Context, key string) (pulumi.BoolOutput, error) {
	v, err := tryBool(ctx, key, "", "")
	if err != nil {
		var empty pulumi.BoolOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Bool(v)).(pulumi.BoolOutput), nil
}

// TrySecretFloat64 loads an optional configuration value by its key, as a float64,
// or returns an error if it doesn't exist.
func TrySecretFloat64(ctx *pulumi.Context, key string) (pulumi.Float64Output, error) {
	v, err := tryFloat64(ctx, key, "", "")
	if err != nil {
		var empty pulumi.Float64Output
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Float64(v)).(pulumi.Float64Output), nil
}

// TrySecretInt loads an optional configuration value by its key, as a int,
// or returns an error if it doesn't exist.
func TrySecretInt(ctx *pulumi.Context, key string) (pulumi.IntOutput, error) {
	v, err := tryInt(ctx, key, "", "")
	if err != nil {
		var empty pulumi.IntOutput
		return empty, err
	}
	return pulumi.ToSecret(pulumi.Int(v)).(pulumi.IntOutput), nil
}
