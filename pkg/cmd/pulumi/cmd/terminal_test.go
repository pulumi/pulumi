// Copyright 2016-2024, Pulumi Corporation.
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

// Terminal detection utilities.
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptimalPageSize(t *testing.T) {
	t.Parallel()
	opt := func(nopts, termHeight int) int {
		return OptimalPageSize(OptimalPageSizeOpts{
			Nopts:          nopts,
			TerminalHeight: termHeight,
		})
	}

	// Case 1: termHeight > nopts
	assert.Equal(t, 0, opt(0, 15))
	assert.Equal(t, 1, opt(1, 15))
	assert.Equal(t, 2, opt(2, 15))
	assert.Equal(t, 3, opt(3, 15))
	assert.Equal(t, 4, opt(4, 15))
	assert.Equal(t, 5, opt(5, 15))
	assert.Equal(t, 1, opt(6, 15))
	assert.Equal(t, 2, opt(7, 15))
	assert.Equal(t, 3, opt(8, 15))

	// Case 2: termHeight <= nopts
	assert.Equal(t, 10, opt(15, 15))
	assert.Equal(t, 10, opt(16, 15))
	assert.Equal(t, 10, opt(17, 15))
}
