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
	assert.Equal(t, "BAIL: big boom", err.Error())
}

func TestBailf(t *testing.T) {
	t.Parallel()

	err := BailErrorf("%d booms", 5)
	assert.Equal(t, "BAIL: 5 booms", err.Error())
}

func TestFprintBailf(t *testing.T) {
	t.Parallel()

	var buff bytes.Buffer
	err := FprintBailf(&buff, "%d booms", 5)
	assert.Equal(t, "BAIL: 5 booms", err.Error())
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

func TestFromError(t *testing.T) {
	t.Parallel()

	errBail := BailErrorf("bail")
	errSimilarToBail := errors.New("bail")
	errSadness := errors.New("great sadness")

	tests := []struct {
		desc string
		give error

		// Properties of the Result:
		wantIsBail bool
		wantErr    error
	}{
		{
			desc:       "bail",
			give:       errBail,
			wantIsBail: true,
			wantErr:    nil,
		},
		{
			// an error with the same message as ErrBail
			// should not be considered a bail.
			desc:    "similar to bail",
			give:    errSimilarToBail,
			wantErr: errSimilarToBail,
		},
		{
			desc:       "wraps bail",
			give:       fmt.Errorf("wraps bail: %w", errBail),
			wantIsBail: true,
			wantErr:    nil,
		},
		{
			desc:       "other error",
			give:       errSadness,
			wantIsBail: false,
			wantErr:    errSadness,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			res := FromError(tt.give)
			assert.Equal(t, tt.wantIsBail, res.IsBail(), "Result.IsBail")
			assert.ErrorIs(t, res.Error(), tt.wantErr, "Result.Error")
		})
	}
}

func TestFromError_nil(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() {
		FromError(nil)
	})
}
