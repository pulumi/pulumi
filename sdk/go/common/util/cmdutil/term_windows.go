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

//go:build windows
// +build windows

package cmdutil

import (
	"errors"

	"golang.org/x/sys/windows"
)

// shutdownProcessGroup sends a CTRL_BREAK_EVENT to the given process group.
// It returns immediately, and does not wait for the process to exit.
func shutdownProcessGroup(pid int) error {
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(pid))
}

// isWaitAlreadyExited returns true
// if the error is due to the process already having exited.
//
// On Windows, this is indicated by the process handle being invalid.
func isWaitAlreadyExited(err error) bool {
	return errors.Is(err, windows.ERROR_INVALID_HANDLE)
}
