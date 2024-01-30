package env_test

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/stretchr/testify/assert"
)

func init() {
	env.Global = env.MapStore{
		"PULUMI_FOO": "1",
		// "PULUMI_NOT_SET": explicitly not set
		"FOO":                "bar",
		"PULUMI_MY_INT":      "3",
		"PULUMI_SECRET":      "hidden",
		"PULUMI_SET":         "SET",
		"UNSET":              "SET",
		"PULUMI_ALTERNATIVE": "SET",
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
	Alternative = env.String("NOT_ALTERNATIVE", "Should be set with alt name", env.Alternative("ALTERNATIVE"))
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

func TestAlternative(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "SET", Alternative.Value())
}
