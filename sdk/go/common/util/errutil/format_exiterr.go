// Copyright 2025, Pulumi Corporation.
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

package errutil

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrorWithStderr returns an error that includes the stderr output if the error is an ExitError.
func ErrorWithStderr(err error, message string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if len(stderr) > 0 {
			return fmt.Errorf("%s: %w: %s", message, exitErr, exitErr.Stderr)
		}
	}
	return fmt.Errorf("%s: %w", message, err)
}
