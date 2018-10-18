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
	"github.com/pulumi/pulumi/sdk/go/pulumi"
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

// GetBool loads an optional bool configuration value by its key, or returns false if it doesn't exist.
func (c *Config) GetBool(key string) bool {
	return GetBool(c.ctx, c.fullKey(key))
}

// GetFloat32 loads an optional float32 configuration value by its key, or returns 0.0 if it doesn't exist.
func (c *Config) GetFloat32(key string) float32 {
	return GetFloat32(c.ctx, c.fullKey(key))
}

// GetFloat64 loads an optional float64 configuration value by its key, or returns 0.0 if it doesn't exist.
func (c *Config) GetFloat64(key string) float64 {
	return GetFloat64(c.ctx, c.fullKey(key))
}

// GetInt loads an optional int configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt(key string) int {
	return GetInt(c.ctx, c.fullKey(key))
}

// GetInt8 loads an optional int8 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt8(key string) int8 {
	return GetInt8(c.ctx, c.fullKey(key))
}

// GetInt16 loads an optional int16 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt16(key string) int16 {
	return GetInt16(c.ctx, c.fullKey(key))
}

// GetInt32 loads an optional int32 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt32(key string) int32 {
	return GetInt32(c.ctx, c.fullKey(key))
}

// GetInt64 loads an optional int64 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetInt64(key string) int64 {
	return GetInt64(c.ctx, c.fullKey(key))
}

// GetUint loads an optional uint configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetUint(key string) uint {
	return GetUint(c.ctx, c.fullKey(key))
}

// GetUint8 loads an optional uint8 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetUint8(key string) uint8 {
	return GetUint8(c.ctx, c.fullKey(key))
}

// GetUint16 loads an optional uint16 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetUint16(key string) uint16 {
	return GetUint16(c.ctx, c.fullKey(key))
}

// GetUint32 loads an optional uint32 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetUint32(key string) uint32 {
	return GetUint32(c.ctx, c.fullKey(key))
}

// GetUint64 loads an optional uint64 configuration value by its key, or returns 0 if it doesn't exist.
func (c *Config) GetUint64(key string) uint64 {
	return GetUint64(c.ctx, c.fullKey(key))
}

// Require loads a configuration value by its key, or panics if it doesn't exist.
func (c *Config) Require(key string) string {
	return Require(c.ctx, c.fullKey(key))
}

// RequireBool loads a bool configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireBool(key string) bool {
	return RequireBool(c.ctx, c.fullKey(key))
}

// RequireFloat32 loads a float32 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireFloat32(key string) float32 {
	return RequireFloat32(c.ctx, c.fullKey(key))
}

// RequireFloat64 loads a float64 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireFloat64(key string) float64 {
	return RequireFloat64(c.ctx, c.fullKey(key))
}

// RequireInt loads a int configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt(key string) int {
	return RequireInt(c.ctx, c.fullKey(key))
}

// RequireInt8 loads a int8 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt8(key string) int8 {
	return RequireInt8(c.ctx, c.fullKey(key))
}

// RequireInt16 loads a int16 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt16(key string) int16 {
	return RequireInt16(c.ctx, c.fullKey(key))
}

// RequireInt32 loads a int32 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt32(key string) int32 {
	return RequireInt32(c.ctx, c.fullKey(key))
}

// RequireInt64 loads a int64 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireInt64(key string) int64 {
	return RequireInt64(c.ctx, c.fullKey(key))
}

// RequireUint loads a uint configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireUint(key string) uint {
	return RequireUint(c.ctx, c.fullKey(key))
}

// RequireUint8 loads a uint8 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireUint8(key string) uint8 {
	return RequireUint8(c.ctx, c.fullKey(key))
}

// RequireUint16 loads a uint16 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireUint16(key string) uint16 {
	return RequireUint16(c.ctx, c.fullKey(key))
}

// RequireUint32 loads a uint32 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireUint32(key string) uint32 {
	return RequireUint32(c.ctx, c.fullKey(key))
}

// RequireUint64 loads a uint64 configuration value by its key, or panics if it doesn't exist.
func (c *Config) RequireUint64(key string) uint64 {
	return RequireUint64(c.ctx, c.fullKey(key))
}

// Try loads a configuration value by its key, returning a non-nil error if it doesn't exist.
func (c *Config) Try(key string) (string, error) {
	return Try(c.ctx, c.fullKey(key))
}

// TryBool loads an optional bool configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryBool(key string) (bool, error) {
	return TryBool(c.ctx, c.fullKey(key))
}

// TryFloat32 loads an optional float32 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryFloat32(key string) (float32, error) {
	return TryFloat32(c.ctx, c.fullKey(key))
}

// TryFloat64 loads an optional float64 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryFloat64(key string) (float64, error) {
	return TryFloat64(c.ctx, c.fullKey(key))
}

// TryInt loads an optional int configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt(key string) (int, error) {
	return TryInt(c.ctx, c.fullKey(key))
}

// TryInt8 loads an optional int8 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt8(key string) (int8, error) {
	return TryInt8(c.ctx, c.fullKey(key))
}

// TryInt16 loads an optional int16 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt16(key string) (int16, error) {
	return TryInt16(c.ctx, c.fullKey(key))
}

// TryInt32 loads an optional int32 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt32(key string) (int32, error) {
	return TryInt32(c.ctx, c.fullKey(key))
}

// TryInt64 loads an optional int64 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryInt64(key string) (int64, error) {
	return TryInt64(c.ctx, c.fullKey(key))
}

// TryUint loads an optional uint configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryUint(key string) (uint, error) {
	return TryUint(c.ctx, c.fullKey(key))
}

// TryUint8 loads an optional uint8 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryUint8(key string) (uint8, error) {
	return TryUint8(c.ctx, c.fullKey(key))
}

// TryUint16 loads an optional uint16 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryUint16(key string) (uint16, error) {
	return TryUint16(c.ctx, c.fullKey(key))
}

// TryUint32 loads an optional uint32 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryUint32(key string) (uint32, error) {
	return TryUint32(c.ctx, c.fullKey(key))
}

// TryUint64 loads an optional uint64 configuration value by its key, or returns an error if it doesn't exist.
func (c *Config) TryUint64(key string) (uint64, error) {
	return TryUint64(c.ctx, c.fullKey(key))
}
