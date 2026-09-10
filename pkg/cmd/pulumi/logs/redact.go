// Copyright 2026, Pulumi Corporation.
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

package logs

import "github.com/pulumi/pulumi/sdk/v3/go/common/resource"

// redactSecretsInValue recursively walks a JSON value and replaces any
// secret objects (identified by the Pulumi secret signature) with a
// redacted placeholder.
func redactSecretsInValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		if isSecretValue(val) {
			delete(val, "ciphertext")
			delete(val, "plaintext")
			delete(val, "value")
			val["plaintext"] = "[secret]"
			return
		}
		for _, child := range val {
			redactSecretsInValue(child)
		}
	case []any:
		for _, child := range val {
			redactSecretsInValue(child)
		}
	}
}

// isSecretValue returns true if the map represents a Pulumi secret value.
func isSecretValue(m map[string]any) bool {
	sigVal, ok := m[resource.SigKey]
	if !ok {
		return false
	}
	sigStr, ok := sigVal.(string)
	return ok && sigStr == resource.SecretSig
}
