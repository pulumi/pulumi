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
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Config is a struct that permits access to config as a "bag" with a package name.  This avoids needing to access
// config with the fully qualified name all of the time (e.g., a bag whose namespace is "p" automatically translates
// attempted reads of keys "k" into "p:k").  This is optional but can save on some boilerplate when accessing config.
type Config struct {
	ctx       *pulumi.Context
	namespace string
}

// New creates a new config bag with the given context and namespace.
func New(ctx *pulumi.Context, namespace string) *Config {
	if namespace == "" {
		namespace = ctx.Project()
	}

	return &Config{ctx: ctx, namespace: namespace}
}

// fullKey turns a simple configuration key into a fully resolved one, by prepending the bag's name.
func (c *Config) fullKey(key string) string {
	return c.namespace + ":" + key
}

// Get loads an optional configuration value by its key, or returns "" if it doesn't exist.
func (c *Config) Get(key string) string {
	return Get(c.ctx, c.fullKey(key))
}

// GetObject loads an optional configuration value into the specified output by its key,
// or returns an error if unable to do so.
func (c *Config) GetObject(key string, output interface{}) error {
	return GetObject(c.ctx, c.fullKey(key), output)
}

// GetBool loads an optional bool configuration value by its key, or returns false if it doesn't exist.
func (c *Config) GetBool(key string) bool {
	return GetBool(c.ctx, c.fullKey(key))
}

// GetFloat64 loads an optional float64 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetFloat64(key string) float64 {
	return GetFloat64(c.ctx, c.fullKey(key))
}

// GetInt loads an optional int configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt(key string) int {
	return GetInt(c.ctx, c.fullKey(key))
}

// Require loads a configuration value by its key, or panics if it doesn't exist.
func (c *Config) Require(key string) string {
	return Require(c.ctx, c.fullKey(key))
}

// RequireObject loads a required configuration value into the specified output by its key,
// or panics if unable to do so.
func (c *Config) RequireObject(key string, output interface{}) {
	RequireObject(c.ctx, c.fullKey(key), output)
}

// RequireBool loads a bool configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireBool(key string) bool {
	return RequireBool(c.ctx, c.fullKey(key))
}

// RequireFloat64 loads a float64 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireFloat64(key string) float64 {
	return RequireFloat64(c.ctx, c.fullKey(key))
}

// RequireInt loads a int configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt(key string) int {
	return RequireInt(c.ctx, c.fullKey(key))
}

// Try loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func (c *Config) Try(key string) (string, error) {
	return Try(c.ctx, c.fullKey(key))
}

// TryObject loads an optional configuration value into the specified output by its key,
// or returns an error if unable to do so.
func (c *Config) TryObject(key string, output interface{}) error {
	return TryObject(c.ctx, c.fullKey(key), output)
}

// TryBool loads an optional bool configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryBool(key string) (bool, error) {
	return TryBool(c.ctx, c.fullKey(key))
}

// TryFloat64 loads an optional float64 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryFloat64(key string) (float64, error) {
	return TryFloat64(c.ctx, c.fullKey(key))
}

// TryInt loads an optional int configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt(key string) (int, error) {
	return TryInt(c.ctx, c.fullKey(key))
}

// GetSecret loads an optional configuration value by its key
// or "" if it doesn't exist, and returns it wrapped in a secret Output.
func (c *Config) GetSecret(key string) pulumi.StringOutput {
	return GetSecret(c.ctx, c.fullKey(key))
}

// GetSecretObject loads an optional configuration value into the specified output by its key,
// returning it wrapped in a secret Output or an error if unable to do so.
func (c *Config) GetSecretObject(key string, output interface{}) (pulumi.Output, error) {
	return GetSecretObject(c.ctx, c.fullKey(key), output)
}

// GetSecretBool loads an optional bool configuration value by its key
// or false if it doesn't exist, and returns it wrapped in a secret Output.
func (c *Config) GetSecretBool(key string) pulumi.BoolOutput {
	return GetSecretBool(c.ctx, c.fullKey(key))
}

// GetSecretFloat64 loads an optional float64 configuration value by its key
// or 0 if it doesn't exist, and returns it wrapped in a secret Output.
func (c *Config) GetSecretFloat64(key string) pulumi.Float64Output {
	return GetSecretFloat64(c.ctx, c.fullKey(key))
}

// GetSecretInt loads an optional int configuration value by its key
// or 0 if it doesn't exist, and returns it wrapped in a secret Output.
func (c *Config) GetSecretInt(key string) pulumi.IntOutput {
	return GetSecretInt(c.ctx, c.fullKey(key))
}

// RequireSecret loads a configuration value by its key
// and returns it wrapped in a secret output, or panics if it doesn't exist.
func (c *Config) RequireSecret(key string) pulumi.StringOutput {
	return RequireSecret(c.ctx, c.fullKey(key))
}

// RequireSecretObject loads a required configuration value into the specified output by its key
// and returns it wrapped in a secret Output, or panics if unable to do so.
func (c *Config) RequireSecretObject(key string, output interface{}) pulumi.Output {
	return RequireSecretObject(c.ctx, c.fullKey(key), output)
}

// RequireSecretBool loads a bool configuration value by its key
// and returns is wrapped in a secret Output, or panics if it doesn't exist.
func (c *Config) RequireSecretBool(key string) pulumi.BoolOutput {
	return RequireSecretBool(c.ctx, c.fullKey(key))
}

// RequireSecretFloat64 loads a float64 configuration value by its key
// and returns is wrapped in a secret Output, or panics if it doesn't exist.
func (c *Config) RequireSecretFloat64(key string) pulumi.Float64Output {
	return RequireSecretFloat64(c.ctx, c.fullKey(key))
}

// RequireSecretInt loads a int configuration value by its key
// and returns is wrapped in a secret Output, or panics if it doesn't exist.
func (c *Config) RequireSecretInt(key string) pulumi.IntOutput {
	return RequireSecretInt(c.ctx, c.fullKey(key))
}

// TrySecret loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func (c *Config) TrySecret(key string) (pulumi.StringOutput, error) {
	return TrySecret(c.ctx, c.fullKey(key))
}

// TrySecretObject loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func (c *Config) TrySecretObject(key string, output interface{}) (pulumi.Output, error) {
	return TrySecretObject(c.ctx, c.fullKey(key), output)
}

// TrySecretBool loads an optional bool configuration value by its key into a secret Output,
// or returns an error if it doesn't exist.
func (c *Config) TrySecretBool(key string) (pulumi.BoolOutput, error) {
	return TrySecretBool(c.ctx, c.fullKey(key))
}

// TrySecretFloat64 loads an optional float64 configuration value by its key into a secret Output,
// or returns an error if it doesn't exist.
func (c *Config) TrySecretFloat64(key string) (pulumi.Float64Output, error) {
	return TrySecretFloat64(c.ctx, c.fullKey(key))
}

// TrySecretInt loads an optional int configuration value by its key into a secret Output,
// or returns an error if it doesn't exist.
func (c *Config) TrySecretInt(key string) (pulumi.IntOutput, error) {
	return TrySecretInt(c.ctx, c.fullKey(key))
}
