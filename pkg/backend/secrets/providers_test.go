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

package secrets

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The blinding provider must build a secrets manager for any type without needing credentials, so that operations
// that only round-trip a checkpoint (or display secrets masked) can proceed offline. This is what lets commands
// like `pulumi stack output` (without --show-secrets) and `pulumi about` run without a passphrase.
func TestBlindingProviderOfTypeNeverErrors(t *testing.T) {
	t.Parallel()

	// Even for the passphrase provider (which normally requires PULUMI_CONFIG_PASSPHRASE), OfType succeeds without
	// any credentials configured.
	sm, err := BlindingProvider.OfType(t.Context(), "passphrase", json.RawMessage(`{"salt":"v1:xyz"}`))
	require.NoError(t, err)
	require.NotNil(t, sm)
	assert.Equal(t, "passphrase", sm.Type())
}

// The blinding provider's decrypter must redact every secret to the "[secret]" sentinel rather than decrypting it.
// The value is JSON-encoded because deployment deserialization unmarshals each decrypted plaintext as a JSON value.
func TestBlindingProviderRedactsSecrets(t *testing.T) {
	t.Parallel()

	sm, err := BlindingProvider.OfType(t.Context(), "passphrase", nil)
	require.NoError(t, err)

	plaintext, err := sm.Decrypter().DecryptValue(t.Context(), "any-ciphertext")
	require.NoError(t, err)
	assert.JSONEq(t, `"[secret]"`, plaintext)
}
