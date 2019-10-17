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
	"fmt"
	"strings"
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

func codes(codes ...string) string {
	return fmt.Sprintf("\x1b[%sm", strings.Join(codes, ";"))
}

func TestColorizer(t *testing.T) {
	cases := []struct {
		command, codes string
	}{
		{Bold, codes("1")},
		{Underline, codes("4")},
		{Red, codes("38", "5", "1")},
		{Green, codes("38", "5", "2")},
		{Yellow, codes("38", "5", "3")},
		{Blue, codes("38", "5", "4")},
		{Magenta, codes("38", "5", "5")},
		{Cyan, codes("38", "5", "6")},
		{BrightRed, codes("38", "5", "9")},
		{BrightGreen, codes("38", "5", "10")},
		{BrightBlue, codes("38", "5", "12")},
		{BrightMagenta, codes("38", "5", "13")},
		{BrightCyan, codes("38", "5", "14")},
		{RedBackground, codes("48", "5", "1")},
		{GreenBackground, codes("48", "5", "2")},
		{YellowBackground, codes("48", "5", "3")},
		{BlueBackground, codes("48", "5", "4")},
		{Black, codes("38", "5", "0")},
	}

	for _, c := range cases {
		t.Run(c.command, func(t *testing.T) {
			const content = "hello"
			str := c.command + content + Reset + "\n"

			actualRaw := colorizeText(str, Raw, -1)
			assert.Equal(t, str, actualRaw)

			actualAlways := Always.Colorize(str)
			assert.Equal(t, c.codes+content+codes("0")+"\n", actualAlways)

			actualNever := Never.Colorize(str)
			assert.Equal(t, content+"\n", actualNever)

			trimmedContent := content[:3]
			actualTrimmed := TrimColorizedString(str, len(trimmedContent))
			assert.Equal(t, c.command+trimmedContent+Reset, actualTrimmed)
		})
	}
}
