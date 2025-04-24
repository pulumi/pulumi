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

package result

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBail(t *testing.T) {
	t.Parallel()

	err := BailError(errors.New("big boom"))
	assert.EqualError(t, err, "BAIL: big boom")
}

func TestBailf(t *testing.T) {
	t.Parallel()

	err := BailErrorf("%d booms", 5)
	assert.EqualError(t, err, "BAIL: 5 booms")
}

func TestFprintBailf(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	err := FprintBailf(&buff, "%d booms", 5)
	assert.EqualError(t, err, "BAIL: 5 booms")
	assert.Equal(t, "5 booms\n", buff.String())
}

func TestIsBail(t *testing.T) {
	t.Parallel()

	inner := errors.New("big boom")
	bail := BailError(inner)
	wrapped := fmt.Errorf("wrapped: %w", bail)

	assert.False(t, IsBail(nil))
	assert.False(t, IsBail(inner))
	assert.True(t, IsBail(bail))
	assert.True(t, IsBail(wrapped))
}

func TestMergeBails(t *testing.T) {
	t.Parallel()

	// Arrange.
	cases := []struct {
		name     string
		errs     []error
		expected error
	}{
		{
			name:     "no errors",
			errs:     []error{},
			expected: nil,
		},
		{
			name:     "one nil",
			errs:     []error{nil},
			expected: nil,
		},
		{
			name:     "one error",
			errs:     []error{errors.New("boom")},
			expected: errors.New("boom"),
		},
		{
			name:     "one bail",
			errs:     []error{BailErrorf("boom")},
			expected: BailErrorf("BAIL: boom"),
		},
		{
			name:     "all nil",
			errs:     []error{nil, nil, nil},
			expected: nil,
		},
		{
			name: "all bails",
			errs: []error{
				BailError(errors.New("boom")),
				BailError(errors.New("bang")),
				BailErrorf("biff"),
			},
			expected: BailErrorf("BAIL: boom\nBAIL: bang\nBAIL: biff"),
		},
		{
			name: "all errors",
			errs: []error{
				errors.New("boom"),
				errors.New("bang"),
				errors.New("biff"),
			},
			expected: errors.New("boom\nbang\nbiff"),
		},
		{
			name: "nils and errors",
			errs: []error{
				nil,
				errors.New("boom"),
				nil,
				errors.New("bang"),
			},
			expected: errors.New("boom\nbang"),
		},
		{
			name: "nils and bails",
			errs: []error{
				nil,
				BailError(errors.New("boom")),
				nil,
				BailErrorf("bang"),
			},
			expected: BailErrorf("BAIL: boom\nBAIL: bang"),
		},
		{
			name: "errors and bails",
			errs: []error{
				errors.New("boom"),
				BailError(errors.New("bang")),
				errors.New("biff"),
			},
			expected: errors.New("boom\nbiff"),
		},
		{
			name: "errors, bails, and nils",
			errs: []error{
				errors.New("boom"),
				nil,
				BailErrorf("bang"),
				nil,
				nil,
				errors.New("biff"),
				nil,
			},
			expected: errors.New("boom\nbiff"),
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			// Act.
			err := MergeBails(c.errs...)

			// Assert.
			if c.expected == nil {
				assert.Nil(t, err)
			} else {
				assert.Equalf(t, IsBail(c.expected), IsBail(err), "Expected IsBail to be %v", IsBail(c.expected))
				assert.EqualError(t, err, c.expected.Error())
			}
		})
	}
}
