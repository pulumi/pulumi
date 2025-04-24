// Copyright 2023-2024, Pulumi Corporation.
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

package terminal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ansiCase struct {
	in           string
	kind         ansiKind
	params       string
	intermediate string
	final        byte
}

func (c ansiCase) run(t *testing.T) {
	t.Run(c.in, func(t *testing.T) {
		t.Parallel()

		var d ansiDecoder
		err := d.decode(strings.NewReader(c.in))
		require.NoError(t, err)
		assert.Equal(t, c.kind, d.kind)
		assert.Equal(t, c.params, string(d.params))
		assert.Equal(t, c.intermediate, string(d.intermediate))
		assert.Equal(t, c.final, d.final)
	})
}

func ansiErrorCase(lead, params, intermediate string, final byte) ansiCase {
	return ansiCase{
		in:           fmt.Sprintf("%v%v%v%v", lead, params, intermediate, string([]byte{final})),
		kind:         ansiError,
		params:       params,
		intermediate: intermediate,
		final:        final,
	}
}

func ansiKeyCase(in byte) ansiCase {
	return ansiCase{in: string([]byte{in}), kind: ansiKey, final: in}
}

func ansiEscapeCase(intermediate string, final byte) ansiCase {
	return ansiCase{
		in:           fmt.Sprintf("\x1b%v%v", intermediate, string([]byte{final})),
		kind:         ansiEscape,
		intermediate: intermediate,
		final:        final,
	}
}

func ansiControlCase(params, intermediate string, final byte) ansiCase {
	return ansiCase{
		in:           fmt.Sprintf("\x1b[%v%v%v", params, intermediate, string([]byte{final})),
		kind:         ansiControl,
		params:       params,
		intermediate: intermediate,
		final:        final,
	}
}

func TestANSIDecoder(t *testing.T) {
	t.Parallel()

	// check all key sequences
	for i := 0; i < 0x7f; i++ {
		if i != 0x1b {
			ansiKeyCase(byte(i)).run(t)
		}
	}

	// check a selection of escape sequences
	escapeCases := []ansiCase{
		ansiEscapeCase("", '7'),
		ansiEscapeCase("", '8'),
		ansiEscapeCase("(", 'A'),
		ansiEscapeCase(")", 'A'),
		ansiErrorCase("\x1b", "", "", 3),
		ansiErrorCase("\x1b", "", "(", 3),
		ansiErrorCase("\x1b", "", "", 0x7f),
	}
	for _, c := range escapeCases {
		c.run(t)
	}

	// check a selection of control sequences
	controlCases := []ansiCase{
		ansiControlCase("5", "", '~'),
		ansiControlCase("6", "", '~'),
		ansiControlCase("5;1", "", '~'),
		ansiControlCase("6;1", "", '~'),
		ansiControlCase("", "", 'A'),
		ansiControlCase("", "", 'B'),
		ansiControlCase("80;24", " ", 'T'),
		ansiErrorCase("\x1b[", "5", "", 3),
		ansiErrorCase("\x1b[", "80;24", " ", 4),
		ansiErrorCase("\x1b[", "5;1", "", 0x7f),
	}
	for _, c := range controlCases {
		c.run(t)
	}
}
