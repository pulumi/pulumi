// Copyright 2016-2022, Pulumi Corporation.
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

package sysutil

import (
	"fmt"
	"syscall"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

var generateConsoleCtrlEvent *syscall.Proc

func init() {
	dll, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		panic(fmt.Errorf("loading kernel32.dll: %w", err))
	}
	proc, err := dll.FindProc("GenerateConsoleCtrlEvent")
	if err != nil {
		panic(fmt.Errorf("finding GenerateConsoleCtrlEvent: %w", err))
	}
	generateConsoleCtrlEvent = proc
}

func Sigint(pid int) {
	_, _, err := generateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(pid))
	contract.IgnoreError(err)
}
