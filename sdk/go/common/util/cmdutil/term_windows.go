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
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func shutdownProcess(proc *os.Process) error {
	kernel32, err := windows.LoadDLL("kernel32.dll")
	if err != nil {
		return fmt.Errorf("load kernel32.dll: %w", err)
	}
	defer func() {
		_ = kernel32.Release()
	}()

	attachConsole, err := kernel32.FindProc("AttachConsole")
	if err != nil {
		return fmt.Errorf("find AttachConsole: %w", err)
	}

	freeConsole, err := kernel32.FindProc("FreeConsole")
	if err != nil {
		return fmt.Errorf("find FreeConsole: %w", err)
	}

	// setConsoleCtrlHandler, err := kernel32.FindProc("SetConsoleCtrlHandler")
	// if err != nil {
	// 	return fmt.Errorf("find SetConsoleCtrlHandler: %w", err)
	// }

	generateConsoleCtrlEvent, err := kernel32.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		return fmt.Errorf("find GenerateConsoleCtrlEvent: %w", err)
	}

	pid := proc.Pid
	if r, _, err := attachConsole.Call(uintptr(pid)); r == 0 {
		// If we're already attached to the console,
		// we'll get an access denied error.
		// We can ignore it and let the rest of the
		// shutdown process continue.
		//
		// Worst case, we can't send the Ctrl-C event.
		if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return fmt.Errorf("attach console: %w", err)
		}
	} else {
		// Schedule a freeConsole only if attachConsole succeeded.
		defer func() {
			_, _, _ = freeConsole.Call()
		}()
	}

	// // Disable Ctrl-C handling for our program.
	// if r, _, err := setConsoleCtrlHandler.Call(0, 0); r == 0 {
	// 	return fmt.Errorf("set console ctrl handler: %w", err)
	// }

	// Send Ctrl-C event to the process group.
	if r, _, err := generateConsoleCtrlEvent.Call(windows.CTRL_C_EVENT, uintptr(pid)); r == 0 {
		return fmt.Errorf("generate console ctrl event: %w", err)
	}

	return nil
}
