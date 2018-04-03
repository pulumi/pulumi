// Copyright 2017-2018, Pulumi Corporation.
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

package cmdutil

import (
	"fmt"

	"github.com/bgentry/speakeasy"

	"github.com/pulumi/pulumi/pkg/diag/colors"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	if prompt != "" {
		prompt = colors.ColorizeText(
			fmt.Sprintf("%s%s:%s ", colors.BrightCyan, prompt, colors.Reset))
		fmt.Print(prompt)
	}

	s, err := speakeasy.Ask("")

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return s, err
}
