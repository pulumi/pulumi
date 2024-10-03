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
	"os"

	"golang.org/x/term"
)

// ReadConsoleNoEcho reads from the console without echoing.  This is useful for reading passwords.
func ReadConsoleNoEcho(prompt string) (string, error) {
	// If standard input is not a terminal, we must not use ReadPassword as it will fail with an ioctl
	// error when it tries to disable local echo.
	//
	// In this case, just read normally
	//nolint:gosec // os.Stdin.Fd() == 0: uintptr -> int conversion is always safe
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return readConsolePlain(os.Stdout, os.Stdin, prompt)
	}

	return readConsoleFancy(os.Stdout, os.Stdin, prompt, true /* secret */)
}
