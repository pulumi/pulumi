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

package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func TestValidateStackConfigValues(t *testing.T) {
	t.Run("No Decrypter Returns Nil", func(t *testing.T) {
		t.Parallel()
		// If decrypter is nil, function should return immediately with no error.
		project := &Project{Name: "testProject"}
		stackCfg := config.Map{}
		err := validateStackConfigValues("stackA", project, stackCfg, nil)
		require.NoError(t, err)
	})

	t.Run("Empty Project With Decrypter Returns Nil", func(t *testing.T) {
		t.Parallel()
		// Non-nil decrypter but no project config entries -> nothing to validate.
		project := &Project{Name: "testProject"}
		stackCfg := config.Map{}

		err := validateStackConfigValues("stackA", project, stackCfg, config.NopDecrypter)
		require.NoError(t, err)
	})

	t.Run("Decrypt Error Is Propagated", func(t *testing.T) {
		t.Parallel()
		// Decrypter returns an error; validateStackConfigValues should return that error.
		project := &Project{Name: "testProject"}
		stackCfg := config.Map{
			config.MustMakeKey("testProject", "someKey"): config.NewSecureValue("someVal"),
		}
		wantErr := "decrypt failed"
		dec := config.NewErrorCrypter(wantErr)

		err := validateStackConfigValues("stackA", project, stackCfg, dec)
		require.Error(t, err)
		require.Contains(t, err.Error(), wantErr)
	})

	t.Run("Secret Enforced", func(t *testing.T) {
		t.Parallel()
		// When project says config is secret, stack value must be secure.
		projectConfigKey := "proj:secretKey"
		pct := ProjectConfigType{
			Secret: true,
		}
		// create a non-secure stack value
		stackVal := config.NewValue("plain")
		err := validateStackConfigValue("stack1", projectConfigKey, pct, stackVal, "plain")
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be encrypted")
	})

	t.Run("Object Unmarshal Error", func(t *testing.T) {
		t.Parallel()
		projectConfigKey := "proj:objKey"
		// object stack value with invalid JSON content should return unmarshal error
		stackVal := config.NewObjectValue("not-a-json")
		pct := ProjectConfigType{}
		err := validateStackConfigValue("stackX", projectConfigKey, pct, stackVal, "not-a-json")
		require.Error(t, err)
		// error should be json unmarshal related
		require.Contains(t, err.Error(), "invalid character")
	})

	t.Run("Type Mismatch", func(t *testing.T) {
		t.Parallel()
		// Project config expects a numeric type but the stack value is a non-numeric string.
		projectConfigKey := "proj:typeKey"
		typ := "number"
		pct := ProjectConfigType{
			Type: &typ,
		}
		// non-numeric stack value
		stackVal := config.NewValue("not-a-number")

		err := validateStackConfigValue("stackX", projectConfigKey, pct, stackVal, "not-a-number")
		require.Error(t, err)
		require.Contains(t, err.Error(), "must be of type")
	})
}

func TestValidateStackConfigAndApplyProjectConfig(t *testing.T) {
	t.Run("Typed Config", func(t *testing.T) {
		t.Parallel()
		intType := integerTypeName
		stringType := stringTypeName
		boolType := booleanTypeName
		project := &Project{
			Name: "testProject",
			Config: map[string]ProjectConfigType{
				"testInt": {
					Type: &intType,
				},
				"testNested1.float": {
					Type: &intType,
				},
				"testNested2.bool": {
					Type: &boolType,
				},
				"testSecret": {
					Type:   &stringType,
					Secret: true,
				},
				"testDefault": {
					Type:    &intType,
					Default: 1,
				},
			},
		}
		stackCfg := config.Map{
			config.MustMakeKey("testProject", "testInt"):     config.NewTypedValue("1", config.TypeInt),
			config.MustMakeKey("testProject", "testNested1"): config.NewObjectValue("{\"float\":1.0}"),
			config.MustMakeKey("testProject", "testNested2"): config.NewObjectValue("{\"bool\":true}"),
			config.MustMakeKey("testProject", "testSecret"):  config.NewSecureValue("dGVzdFZhbHVl"),
		}
		crypter, _, _, calledDecryptValue, calledBatchDecrypt := getCountingBase64Crypter(t.Context(), t)

		err := ValidateStackConfigAndApplyProjectConfig(
			t.Context(), "stackA", project, esc.Value{}, stackCfg, crypter, crypter,
		)
		require.NoError(t, err)

		// Validate that decryption was cached appropriately.
		require.Equal(t, 0, *calledDecryptValue)
		require.Equal(t, 1, *calledBatchDecrypt)
	})
}

func getCountingBase64Crypter(ctx context.Context, t *testing.T) (config.Crypter, *int, *int, *int, *int) {
	calledEncryptValue := 0
	calledBatchEncrypt := 0
	calledDecryptValue := 0
	calledBatchDecrypt := 0
	encrypter := &secrets.MockEncrypter{
		EncryptValueF: func(input string) string {
			calledEncryptValue++
			ct, err := config.Base64Crypter.EncryptValue(ctx, input)
			require.NoError(t, err)
			return ct
		},
		BatchEncryptF: func(input []string) []string {
			calledBatchEncrypt++
			ct, err := config.Base64Crypter.BatchEncrypt(ctx, input)
			require.NoError(t, err)
			return ct
		},
	}
	decrypter := &secrets.MockDecrypter{
		DecryptValueF: func(input string) string {
			calledDecryptValue++
			pt, err := config.Base64Crypter.DecryptValue(ctx, input)
			require.NoError(t, err)
			return pt
		},
		BatchDecryptF: func(input []string) []string {
			calledBatchDecrypt++
			pt, err := config.Base64Crypter.BatchDecrypt(ctx, input)
			require.NoError(t, err)
			return pt
		},
	}
	return config.NewCiphertextToPlaintextCachedCrypter(encrypter, decrypter),
		&calledEncryptValue, &calledBatchEncrypt, &calledDecryptValue, &calledBatchDecrypt
}
