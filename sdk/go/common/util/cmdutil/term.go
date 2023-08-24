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
//   - On Linux and macOS, it sends a SIGINT
//   - On Windows, it sends a CTRL_BREAK_EVENT
//
// If the process does not exit gracefully within the given duration,
// it will be forcibly terminated.
func TerminateProcess(proc *os.Process, cooldown time.Duration) error {
	// The choice to use SIGINT and CTRL_BREAK_EVENT
	// merits some explanation.
	//
	// On *nix, typically,
	// SIGTERM is used for programmatic graceful shutdown,
	// and SIGINT is used when the user presses Ctrl+C.
	// e.g. Kubernetes sends SIGTERM to signal shutdown.
	// So in short, SIGTERM is for computers, SIGINT is for humans.
	//
	// On Windows,
	// there's CTRL_C_EVENT which is obviously analogous to SIGINT
	// because they both handle Ctrl+C.
	// But there's also CTRL_BREAK_EVENT which is special to Windows,
	// but we can decide it's analogous to SIGTERM.
	//
	// However, when writing a signal handler on Windows,
	// different languages map these signals differently.
	// Go maps both, CTRL_BREAK_EVENT and CTRL_C_EVENT to SIGINT,
	// Node and Python map CTRL_BREAK_EVENT to SIGBREAK,
	// and CTRL_C_EVENT to SIGINT.
	//
	// (SIGBREAK is a special, Windows-only signal.)
	//
	// In short:
	//
	//    |  OS  | Signal sent      | Language | Handled as |
	//    |------|------------------|----------|------------|
	//    | *nix | SIGTERM          | Go       | SIGTERM    |
	//    |      |                  | Node     | SIGTERM    |
	//    |      |                  | Python   | SIGTERM    |
	//    |      |------------------|----------|------------|
	//    |      | SIGINT           | Go       | SIGINT     |
	//    |      |                  | Node     | SIGINT     |
	//    |      |                  | Python   | SIGINT     |
	//    |------|------------------|----------|------------|
	//    | Win  | CTRL_BREAK_EVENT | Go       | SIGINT     |
	//    |      |                  | Node     | SIGBREAK   |
	//    |      |                  | Python   | SIGBREAK   |
	//    |      |------------------|----------|------------|
	//    |      | CTRL_C_EVENT     | Go       | SIGINT     |
	//    |      |                  | Node     | SIGINT     |
	//    |      |                  | Python   | SIGINT     |
	//
	// So the SIGINT+CTRL_C_EVENT combo would be the obvious choice here
	// since it's consistent across languages and platforms;
	// plugins would define a single SIGINT handler
	// and it would work in all cases.
	//
	// Unfortunately, Winodws does not support sending CTRL_C_EVENT
	// to a specific child process.
	// It's "current process and all child processes" or nothing.
	/// Per the docs [1], the CTRL_C_EVENT
	// "cannot be limited to a specific process group."
	//
	// [1]: https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent
	//
	// So we have to use CTRL_BREAK_EVENT for Windows instead.
	// At that point, using SIGINT for *nix makes sense because:
	//
	// - It'll at least simplify the Go plugins.
	// - Users will want to handle SIGINT anyway
	//   because they'll want to be able to press Ctrl+C in the terminal.

	if err := shutdownProcess(proc); err != nil {
		return err
		// TODO: if this failed, log it and force kill the process.
	}

	return nil
}
