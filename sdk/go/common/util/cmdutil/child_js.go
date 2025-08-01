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

//go:build js
// +build js

package cmdutil

import (
	"errors"
	"os"
	"os/exec"
)

func killProcessGroup(proc *os.Process) error {
	panic(errors.New("unsupported"))
}

func KillChildren(pid int) error {
	panic(errors.New("unsupported"))
}

func processExistsWithParent(pid int, ppid int) (bool, error) {
	panic(errors.New("unsupported"))
}

func RegisterProcessGroup(cmd *exec.Cmd) {
	panic(errors.New("unsupported"))
}

func InterruptChildren(pid int) error {
	panic(errors.New("unsupported"))
}

func CreateProcessGroup() error {
	panic(errors.New("unsupported"))
}
