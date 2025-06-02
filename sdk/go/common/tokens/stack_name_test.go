// Copyright 2016-2023, Pulumi Corporation.
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

package tokens

import (
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/stretchr/testify/assert"
)

func TestParseStackName_AcceptsValidInput(t *testing.T) {
	t.Parallel()

	// Arrange.
	tests := []string{
		"my-stack",
		"stack",
		"s-92",
		"stack_foo",
		"foo_bar_123",
	}

	for _, tt := range tests {
		// Act.
		sn, err := ParseStackName(tt)

		// Assert.
		assert.Equal(t, tt, sn.String())
		assert.NoError(t, err)
	}
}

func TestParseStackName_RejectsInvalidInput(t *testing.T) {
	t.Parallel()

	// Arrange.
	tests := []struct {
		desc  string
		input string
		err   string
	}{
		{
			desc:  "stack name empty",
			input: "",
			err:   "a stack name may not be empty",
		},
		{
			desc: "stack name too long",
			input: "this-stack-name-is-just-too-long-for-the-service-to-handle-and-should-be-rejected-by-the-service-" +
				"because-it-is-just-too-long-for-the-service-to-handle-and-should-be-rejected-by-the-service",
			err: "a stack name cannot exceed 100 characters",
		},
		{
			desc:  "invalid start to stack name",
			input: "!my stack!",
			err: "a stack name may only contain alphanumeric, hyphens, underscores, " +
				"or periods: invalid character '!' at position 0",
		},
		{
			desc:  "invalid rest of stack name",
			input: "my bad",
			err: "a stack name may only contain alphanumeric, hyphens, underscores, " +
				"or periods: invalid character ' ' at position 2",
		},
		{
			desc:  "invalid end of stack name",
			input: "mybad%",
			err: "a stack name may only contain alphanumeric, hyphens, underscores, " +
				"or periods: invalid character '%' at position 5",
		},
		{
			desc:  "invalid slash in stack name",
			input: "foo/bar",
			err: "a stack name may only contain alphanumeric, hyphens, underscores, " +
				"or periods: invalid character '/' at position 3",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			// Act.
			_, err := ParseStackName(tt.input)

			// Assert.
			assert.EqualError(t, err, tt.err)
		})
	}
}

func TestStackNameValidation_AssertsNonEmpty(t *testing.T) {
	t.Parallel()

	// Act.
	sn := &StackName{}

	// Assert.
	assert.PanicsWithValue(t, "fatal: An assertion has failed: stack name must not be empty", func() {
		contract.Ignore(sn.String())
	})
}

func TestStackNameValidation_CanBeDisabled(t *testing.T) {
	t.Setenv("PULUMI_DISABLE_VALIDATION", "true")

	// Act.
	sn1 := StackName{}
	sn2, err := ParseStackName("")

	// Assert.
	assert.NoError(t, err)
	assert.Equal(t, "", sn1.String())
	assert.Equal(t, "", sn2.String())
}
