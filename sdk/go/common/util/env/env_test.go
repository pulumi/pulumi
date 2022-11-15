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
		"FOO":           "bar",
		"UNSET":         "SET",
		"PULUMI_SET":    "SET",
		"PULUMI_MY_INT": "3",
	}
}

var (
	SomeBool    = env.Bool("FOO", "A bool used for testing")
	SomeFalse   = env.Bool("NOT_SET", "a falsy value")
	SomeString  = env.String("FOO", "A bool used for testing", env.NoPrefix)
	UnsetString = env.String("PULUMI_UNSET", "Should be unset", env.Needs(SomeFalse))
	SetString   = env.String("SET", "Should be set", env.Needs(SomeBool))
	AnInt       = env.Int("MY_INT", "Should be 3")
)

func TestInt(t *testing.T) {
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

func TestNeeds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", UnsetString.Value())
	assert.Equal(t, "SET", SetString.Value())
}
