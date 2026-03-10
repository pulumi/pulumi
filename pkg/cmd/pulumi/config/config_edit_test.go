// Copyright 2016-2025, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"testing"

	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/stretchr/testify/require"
)

func TestEditableConfigRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	encryptedSecret, err := config.Base64Crypter.EncryptValue(ctx, "hunter2")
	require.NoError(t, err)

	initial := config.Map{
		config.MustMakeKey("test", "plain"):  config.NewValue("value"),
		config.MustMakeKey("test", "secret"): config.NewSecureValue(encryptedSecret),
		config.MustMakeKey("test", "object"): config.NewObjectValue(`{"enabled":true,"retries":3}`),
	}

	editable, err := encodeEditableConfig(initial, config.Base64Crypter)
	require.NoError(t, err)

	var editableMap map[string]any
	require.NoError(t, json.Unmarshal(editable, &editableMap))

	secretJSON := editableMap["test:secret"].(map[string]any)
	require.Equal(t, true, secretJSON["secret"])
	require.Equal(t, "hunter2", secretJSON["value"])

	objectJSON := editableMap["test:object"].(map[string]any)
	require.Equal(t, false, objectJSON["secret"])
	require.NotNil(t, objectJSON["objectValue"])

	roundTripped, err := decodeEditableConfig(ctx, &pkgWorkspace.MockContext{}, editable, config.Base64Crypter)
	require.NoError(t, err)

	initialValues, err := initial.Decrypt(config.Base64Crypter)
	require.NoError(t, err)

	roundTripValues, err := roundTripped.Decrypt(config.Base64Crypter)
	require.NoError(t, err)

	require.Equal(t, initialValues, roundTripValues)
	for key, initialValue := range initial {
		require.Equal(t, initialValue.Secure(), roundTripped[key].Secure())
		require.Equal(t, initialValue.Object(), roundTripped[key].Object())
	}
}

func TestEncodeEditableConfigSecureObject(t *testing.T) {
	t.Parallel()

	secureObject := config.Map{
		config.MustMakeKey("test", "obj"): config.NewSecureObjectValue(`{"inner":{"secure":"c2VjcmV0"}}`),
	}

	editable, err := encodeEditableConfig(secureObject, config.Base64Crypter)
	require.NoError(t, err)

	var editableMap map[string]any
	require.NoError(t, json.Unmarshal(editable, &editableMap))

	secretValue := editableMap["test:obj"].(map[string]any)
	require.Equal(t, true, secretValue["secret"])
	require.Equal(t, map[string]any{"inner": "secret"}, secretValue["objectValue"])
}

func TestDecodeEditableConfigNeedsEncrypter(t *testing.T) {
	t.Parallel()

	_, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key":{"value":"plaintext","secret":true}}`),
		nil,
	)
	require.ErrorIs(t, err, errConfigEditNeedsEncrypter)
}

func TestDecodeEditableConfigSecretString(t *testing.T) {
	t.Parallel()

	cfg, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key":{"value":"plaintext","secret":true}}`),
		config.Base64Crypter,
	)
	require.NoError(t, err)

	value := cfg[config.MustMakeKey("test", "key")]
	require.True(t, value.Secure())
	require.False(t, value.Object())

	decrypted, err := value.Value(config.Base64Crypter)
	require.NoError(t, err)
	require.Equal(t, "plaintext", decrypted)
}

func TestDecodeEditableConfigLiteralSecretPrefixString(t *testing.T) {
	t.Parallel()

	cfg, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key":{"value":"secret:plaintext","secret":false}}`),
		config.Base64Crypter,
	)
	require.NoError(t, err)

	key := config.MustMakeKey("test", "key")
	value := cfg[key]
	require.False(t, value.Secure())
	require.False(t, value.Object())

	decrypted, err := value.Value(config.NewPanicCrypter())
	require.NoError(t, err)
	require.Equal(t, "secret:plaintext", decrypted)
}

func TestDecodeEditableConfigRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	_, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key:invalid":{"value":"value","secret":false}}`),
		config.Base64Crypter,
	)
	require.ErrorContains(t, err, "invalid edited config key")
}

func TestDecodeEditableConfigSecretObject(t *testing.T) {
	t.Parallel()

	cfg, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:obj":{"value":"{\"inner\":\"value\",\"arr\":[1,true]}","objectValue":{"inner":"value","arr":[1,true]},"secret":true}}`),
		config.Base64Crypter,
	)
	require.NoError(t, err)

	key := config.MustMakeKey("test", "obj")
	value := cfg[key]
	require.True(t, value.Secure())
	require.True(t, value.Object())

	decrypted, err := cfg.Decrypt(config.Base64Crypter)
	require.NoError(t, err)
	require.JSONEq(t, `{"arr":[1,true],"inner":"value"}`, decrypted[key])
}

func TestDecodeEditableConfigLiteralSecretObject(t *testing.T) {
	t.Parallel()

	cfg, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:obj":{"value":"{\"secret\":\"literal\"}","objectValue":{"secret":"literal"},"secret":false}}`),
		config.Base64Crypter,
	)
	require.NoError(t, err)

	key := config.MustMakeKey("test", "obj")
	value := cfg[key]
	require.False(t, value.Secure())
	require.True(t, value.Object())

	decrypted, err := cfg.Decrypt(config.Base64Crypter)
	require.NoError(t, err)
	require.Equal(t, `{"secret":"literal"}`, decrypted[key])
}

func TestDecodeEditableConfigSingleDocument(t *testing.T) {
	t.Parallel()

	_, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key":{"value":"value","secret":false}} {"test:other":{"value":"value","secret":false}}`),
		config.Base64Crypter,
	)
	require.ErrorContains(t, err, "single JSON object")
}

func TestDecodeEditableConfigValueRequired(t *testing.T) {
	t.Parallel()

	_, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:key":{"secret":false}}`),
		config.Base64Crypter,
	)
	require.ErrorContains(t, err, "value is nil")
}

func TestDecodeEditableConfigObjectValueWins(t *testing.T) {
	t.Parallel()

	cfg, err := decodeEditableConfig(
		context.Background(),
		&pkgWorkspace.MockContext{},
		[]byte(`{"test:obj":{"value":"{\"stale\":true}","objectValue":{"fresh":true},"secret":false}}`),
		config.Base64Crypter,
	)
	require.NoError(t, err)

	key := config.MustMakeKey("test", "obj")
	value := cfg[key]
	require.False(t, value.Secure())
	require.True(t, value.Object())

	decrypted, err := cfg.Decrypt(config.Base64Crypter)
	require.NoError(t, err)
	require.JSONEq(t, `{"fresh":true}`, decrypted[key])
}
