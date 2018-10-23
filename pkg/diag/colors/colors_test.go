// Copyright 2016-2018, Pulumi Corporation.
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

package colors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimPartialCommand(t *testing.T) {
	noPartial := Red + "foo" + Green + "bar" + Reset
	assert.Equal(t, noPartial, TrimPartialCommand(noPartial))

	expected := Red + "foo" + Green + "bar"
	for partial := noPartial[:len(noPartial)-1]; partial[len(partial)-3:] != "bar"; partial = partial[:len(partial)-1] {
		assert.Equal(t, expected, TrimPartialCommand(partial))
	}
}
