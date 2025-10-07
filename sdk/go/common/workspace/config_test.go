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
	"testing"

	"github.com/stretchr/testify/require"

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
