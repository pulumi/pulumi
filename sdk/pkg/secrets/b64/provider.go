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
)

// Base64SecretsProvider is a SecretsProvider that only supports base64 secrets, it is intended to be used for tests
// where actual encryption is not needed.
var Base64SecretsProvider *secrets.MockProvider = (&secrets.MockProvider{}).Add(
	Type, func(_ json.RawMessage) (secrets.Manager, error) { mgr := NewBase64SecretsManager(); return mgr, nil },
)
