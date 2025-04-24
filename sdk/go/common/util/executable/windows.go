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

//go:build windows

package executable

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32       = windows.NewLazySystemDLL("kernel32.dll")
	getBinaryTypeW = kernel32.NewProc("GetBinaryTypeW")
)

func IsExecutable(path string) (bool, error) {
	// Use GetBinaryTypeW to determine if the file is an executable.
	// https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getbinarytypew
	pathp, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, fmt.Errorf("unable to convert path to UTF16: %s\n", err)
	}
	r, _, err := getBinaryTypeW.Call(uintptr(unsafe.Pointer(pathp)))
	// On success, err is syscall.Errno(0).
	if !errors.Is(err, syscall.Errno(0)) {
		return false, err
	}
	return r > 0, nil
}
