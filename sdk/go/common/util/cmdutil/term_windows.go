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

//go:build windows
// +build windows

package cmdutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func shutdownProcess(proc *os.Process) error {
	// TODO: Can we just use:
	// return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(proc.Pid))
	// from golang.org/x/sys/windows

	kernel32, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("load kernel32.dll: %w", err)
	}
	defer func() {
		_ = kernel32.Release()
	}()

	pid := proc.Pid

	generateConsoleCtrlEvent, err := kernel32.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return fmt.Errorf("find GenerateConsoleCtrlEvent: %w", err)
	}

	if r, _, err := generateConsoleCtrlEvent.Call(windows.CTRL_BREAK_EVENT, uintptr(pid)); r == 0 {
		return fmt.Errorf("generate console ctrl event: %w", err)
	}

	return nil
}
