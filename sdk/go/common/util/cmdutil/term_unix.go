// Copyright 2016-2023, Pulumi Corporation.
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

//go:build !windows && !js
// +build !windows,!js

package cmdutil

import (
	"errors"
	"syscall"
)

func Interrupt(pid int) error {
	return syscall.Kill(pid, syscall.SIGINT)
}

// shutdownProcessGroup sends a SIGINT to the given process group.
// It returns immediately, and does not wait for the process to exit.
//
// A Windows version of this function is defined in term_windows.go.
func shutdownProcessGroup(pid int) error {
	// Processes spawned after calling RegisterProcessGroup
	// will be part of the same process group as the parent.
	//
	// -pid means send the signal to the entire process group.
	//
	// See: https://linux.die.net/man/2/kill
	return syscall.Kill(-pid, syscall.SIGINT)
}

// isWaitAlreadyExited returns true
// if the error is due to the process already having exited.
//
// On Linux, this is indicated by ESRCH or ECHILD.
//
// A Windows version of this function is defined in term_windows.go.
func isWaitAlreadyExited(err error) bool {
	return errors.Is(err, syscall.ESRCH) || //  no such process
		errors.Is(err, syscall.ECHILD) //  no child processes
}
