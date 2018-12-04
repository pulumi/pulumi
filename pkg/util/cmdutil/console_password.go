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

package cmdutil

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	// If standard input is not a terminal, we must not use ReadPassword as it will fail with an ioctl
	// error when it tries to disable local echo.
	//
	// In this case, just read normally
	if !terminal.IsTerminal(int(os.Stdin.Fd())) {
		return ReadConsole(prompt)
	}

	if prompt != "" {
		fmt.Print(prompt + ": ")
	}

	b, err := terminal.ReadPassword(int(os.Stdin.Fd()))

	fmt.Println() // echo a newline, since the user's keypress did not generate one

	return string(b), err
}
