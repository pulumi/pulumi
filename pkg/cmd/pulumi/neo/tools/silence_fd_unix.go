// Copyright 2026, Pulumi Corporation.
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

//go:build !linux && !windows

package tools

import (
	"os"

	"golang.org/x/sys/unix"
)

// redirectFD2 saves the current stderr file descriptor and points fd 2 at target,
// returning the saved descriptor for restoreFD2.
func redirectFD2(target *os.File) (int, error) {
	saved, err := unix.Dup(unix.Stderr)
	if err != nil {
		return -1, err
	}
	if err := unix.Dup2(int(target.Fd()), unix.Stderr); err != nil {
		_ = unix.Close(saved)
		return -1, err
	}
	return saved, nil
}

// restoreFD2 points fd 2 back at the descriptor saved by redirectFD2 and closes it.
func restoreFD2(saved int) error {
	err := unix.Dup2(saved, unix.Stderr)
	_ = unix.Close(saved)
	return err
}
