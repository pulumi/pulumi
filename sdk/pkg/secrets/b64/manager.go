// Copyright 2016-2023, Pulumi Corporation.
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

// Package b64 implements a base64 secrets manager for testing purposes.
package b64

import (
	"encoding/json"

	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

const Type = "b64"

// NewBase64SecretsManager returns a secrets manager that just base64 encodes instead of encrypting. Useful for testing.
func NewBase64SecretsManager() secrets.Manager {
	return &manager{}
}

type manager struct{}

func (m *manager) Type() string                { return Type }
func (m *manager) State() json.RawMessage      { return nil }
func (m *manager) Encrypter() config.Encrypter { return config.Base64Crypter }
func (m *manager) Decrypter() config.Decrypter { return config.Base64Crypter }
