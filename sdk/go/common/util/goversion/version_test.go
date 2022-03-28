package goversion

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_checkMinimumGoVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		goVersionOutput string
		err             error
	}{
		{
			name:            "ExactVersion",
			goVersionOutput: "go version go1.14.0 darwin/amd64",
		},
		{
			name:            "NewerVersion",
			goVersionOutput: "go version go1.15.1 darwin/amd64",
		},
		{
			name:            "BetaVersion",
			goVersionOutput: "go version go1.18beta2 darwin/amd64",
		},
		{
			name:            "OlderGoVersion",
			goVersionOutput: "go version go1.13.8 linux/amd64",
			err:             errors.New("go version must be 1.14.0 or higher (1.13.8 detected)"),
		},
		{
			name:            "MalformedVersion",
			goVersionOutput: "go version xyz",
			err:             errors.New("parsing go version: Malformed version: xyz"),
		},
		{
			name:            "GarbageVersionOutput",
			goVersionOutput: "gobble gobble",
			err:             errors.New("unexpected format for go version output: \"gobble gobble\""),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := checkMinimumGoVersion(tt.goVersionOutput)
			if err != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tt.err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
