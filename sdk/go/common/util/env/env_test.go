package env_test

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/assert"
)

func init() {
	// This makes env.Global comparable.
	env.Global = &env.MapStore{
		"PULUMI_FOO": "1",
		// "PULUMI_NOT_SET": explicitly not set
		"FOO":           "bar",
		"PULUMI_MY_INT": "3",
		"PULUMI_SECRET": "hidden",
		"PULUMI_SET":    "SET",
		"UNSET":         "SET",

		"PULUMI_INVALID_INT": "forty two",
	}
}

var (
	SomeBool    = env.Bool("FOO", "A bool used for testing")
	SomeFalse   = env.Bool("NOT_SET", "a falsy value")
	SomeString  = env.String("FOO", "A bool used for testing", env.NoPrefix)
	SomeSecret  = env.String("SECRET", "A secret that shouldn't be displayed", env.Secret)
	UnsetString = env.String("PULUMI_UNSET", "Should be unset", env.Needs(SomeFalse))
	SetString   = env.String("SET", "Should be set", env.Needs(SomeBool))
	AnInt       = env.Int("MY_INT", "Should be 3")

	DefaultInt = env.Int("DEFAULT_INT", "An int with a default value",
		env.Default("42"))

	dependentDefaultCount = 0
	DependentDefault      = env.Bool("DEPENDENT_DEFAULT", "A value that depends on another value",
		env.DefaultF(func(e env.Env) (string, error) {
			v := e.GetInt(DefaultInt)
			if v == 42 {
				dependentDefaultCount++
			}

			if v > 0 {
				return "true", nil
			} else if v < 0 {
				return "false", nil
			}
			return "", fmt.Errorf("%s is zero", DefaultInt.Var().Name())
		}))

	DefaultWithNeeds = env.String("HAS_FOO", "A default that only triggers when SomeBool is true",
		env.Needs(SomeBool), env.Default("no"))
)

func TestInt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 3, AnInt.Value())
	assert.Equal(t, 3, env.NewEnv(env.Global).GetInt(AnInt))
	assert.Equal(t, 6, env.NewEnv(
		env.MapStore{"PULUMI_MY_INT": "6"},
	).GetInt(AnInt))
}

func TestBool(t *testing.T) {
	t.Parallel()
	assert.Equal(t, true, SomeBool.Value())
	assert.Equal(t, false, SomeFalse.Value())
}

func TestString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "bar", SomeString.Value())
}

func TestSecret(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "hidden", SomeSecret.Value())
	assert.Equal(t, "[secret]", SomeSecret.String())
}

func TestNeeds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", UnsetString.Value())
	assert.Equal(t, "SET", SetString.Value())
}

func TestGlobalDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 42, DefaultInt.Value())

	assert.Equal(t, true, DependentDefault.Value())
	assert.Equal(t, true, DependentDefault.Value())
	assert.Equal(t, true, DependentDefault.Value())

	assert.Equal(t, 1, dependentDefaultCount)
}

func TestLocalDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 42, DefaultInt.Value())
	assert.Equal(t, true, DependentDefault.Value())

	local := env.NewEnv(env.MapStore{
		"PULUMI_DEFAULT_INT": "-99",
	})

	assert.Equal(t, -99, local.GetInt(DefaultInt))
	assert.Equal(t, false, local.GetBool(DependentDefault))
}

func TestNeedsDefault(t *testing.T) {
	t.Parallel()

	local := env.NewEnv(env.MapStore{
		"PULUMI_FOO":     "true",
		"PULUMI_HAS_FOO": "yes",
	})

	assert.Equal(t, "yes", local.GetString(DefaultWithNeeds))

	local = env.NewEnv(env.MapStore{
		"PULUMI_FOO":     "false",
		"PULUMI_HAS_FOO": "yes",
	})

	assert.Equal(t, "no", local.GetString(DefaultWithNeeds))
}

func TestInvalidDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, env.ValidateError{}, AnInt.Validate())

	local := env.NewEnv(env.MapStore{
		"PULUMI_MY_INT": "three",
	})

	validation := local.Int(AnInt).Validate()
	assert.Nil(t, validation.Warning)
	assert.NotNil(t, validation.Error)

	assert.Equal(t, 0, local.Int(AnInt).Value())
}
