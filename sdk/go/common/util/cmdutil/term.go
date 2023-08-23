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
	"time"
)

// TerminateProcess terminates the given process.
// It does so by sending a termination signal to the process.
//
//   - On Linux and macOS, it sends a SIGTERM
//   - On Windows, it sends a CTRL_BREAK_EVENT
//
// If the process does not exit gracefully within the given duration,
// it will be forcibly terminated.
func TerminateProcess(proc *os.Process, cooldown time.Duration) error {
	if err := shutdownProcess(proc); err != nil {
		return err
	}

	return nil
}
