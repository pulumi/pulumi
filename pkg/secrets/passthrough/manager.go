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

// Package passthrough implements a passthrough secrets manager for cases where the user wants to disable pulumi encryption.
package passthrough

import (
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

const Type = "passthrough"

// NewPassthroughSecretsManager returns a secrets manager that doesn't do any encrypting.
func NewPassthroughSecretsManager() secrets.Manager {
	return &manager{}
}

type manager struct{}

func (m *manager) Type() string                         { return Type }
func (m *manager) State() interface{}                   { return map[string]string{} }
func (m *manager) Encrypter() (config.Encrypter, error) { return config.NopEncrypter, nil }
func (m *manager) Decrypter() (config.Decrypter, error) { return config.NopDecrypter, nil }
