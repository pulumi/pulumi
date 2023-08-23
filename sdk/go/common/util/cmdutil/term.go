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
//   - On Windows, it sends a CTRL_C_EVENT
//
// If the process does not exit gracefully within the given duration,
// it will be forcibly terminated.
func TerminateProcess(proc *os.Process, cooldown time.Duration) error {
	// The choice to use SIGINT and CTRL_C_EVENT bears some explanation.
	//
	// First, for context, typically on Unix systems,
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
	// So SIGTERM and CTRL_BREAK_EVENT would be the obvious choices here
	// except when you bring cross-language support into the picture.
	//
	// When writing a signal handler on Windows in different langauges,
	// Go maps both, CTRL_BREAK_EVENT and CTRL_C_EVENT to SIGINT,
	// Node and Python map CTRL_BREAK_EVENT to SIGBREAK*,
	// and Node and Python map CTRL_C_EVENT to SIGINT.
	//
	//     *SIGBREAK is a special, Windows-only signal.
	//
	// In short:
	//
	//    |  OS  | Signal sent      | Language | Handled as      |
	//    |------|------------------|----------|-----------------|
	//    | *nix | SIGTERM          | Go       | SIGTERM         |
	//    |      |                  | Node     | SIGTERM         |
	//    |      |                  | Python   | SIGTERM         |
	//    |      |------------------|----------|-----------------|
	//    |      | SIGINT           | Go       | SIGINT          |
	//    |      |                  | Node     | SIGINT          |
	//    |      |                  | Python   | SIGINT          |
	//    |------|------------------|----------|-----------------|
	//    | Win  | CTRL_BREAK_EVENT | Go       | SIGINT          |
	//    |      |                  | Node     | SIGBREAK        |
	//    |      |                  | Python   | SIGBREAK        |
	//    |      |------------------|----------|-----------------|
	//    |      | CTRL_C_EVENT     | Go       | SIGINT          |
	//    |      |                  | Node     | SIGINT          |
	//    |      |                  | Python   | SIGINT          |
	//
	// So the SIGINT+CTRL_C_EVENT combo is the only one that works
	// consistently across these languages and platforms.
	// That is, we can say that a process should only handle SIGINT
	// and that'll be handled correctly in all cases.

	if err := shutdownProcess(proc); err != nil {
		return err
		// TODO: if this failed, log it and force kill the process.
	}

	return nil
}
