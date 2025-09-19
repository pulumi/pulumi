package workspace

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
)

func TestValidateStackConfigValues(t *testing.T) {
	t.Run("No Decrypter Returns Nil", func(t *testing.T) {
		// If decrypter is nil, function should return immediately with no error.
		project := &Project{Name: "testProject"}
		stackCfg := config.Map{}
		err := validateStackConfigValues("stackA", project, stackCfg, nil)
		require.NoError(t, err)
	})

	t.Run("Empty Project With Decrypter Returns Nil", func(t *testing.T) {
		// Non-nil decrypter but no project config entries -> nothing to validate.
		project := &Project{Name: "testProject"}
		stackCfg := config.Map{}

		err := validateStackConfigValues("stackA", project, stackCfg, config.NopDecrypter)
		require.NoError(t, err)
	})

	t.Run("Decrypt Error Is Propagated", func(t *testing.T) {
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
