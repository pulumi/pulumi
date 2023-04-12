package cgstrings

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCamel(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	assert.Equal("", Camel(""))
	assert.Equal("plugh", Camel("plugh"))
	assert.Equal("waldoThudFred", Camel("WaldoThudFred"))
	assert.Equal("graultBaz", Camel("Grault-Baz"))
	assert.Equal("graultBaz", Camel("grault-baz"))
	assert.Equal("graultBaz", Camel("graultBaz"))
	assert.Equal("grault_Baz", Camel("Grault_Baz"))
	assert.Equal("graultBaz", Camel("Grault-baz"))
}

func TestUnhyphenate(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		input, expected string
	}{
		{"", ""},
		{"waldo", "waldo"},
		{"waldo-thud-fred", "waldoThudFred"},
		{"waldo-Thud-Fred", "waldoThudFred"},
		{"waldo-Thud-Fred-", "waldoThudFred"},
		{"-waldo-Thud-Fred", "WaldoThudFred"},
		{"waldoThudFred", "waldoThudFred"},
		{"WaldoThudFred", "WaldoThudFred"},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("Subtest:%q", tc.input), func(t *testing.T) {
			t.Parallel()
			assert := assert.New(t)
			assert.Equal(tc.expected, Unhyphenate(tc.input))
		})
	}
}
