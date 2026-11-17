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

package cmdutil

import (
	"os"
	"strconv"
	"strings"
)

// isZombie reports whether the process with the given PID is a zombie.
//
// On Linux, a zombie process (state 'Z') is effectively dead but retains
// an entry in the process table until its parent reaps it via wait(2).
// In containers where PID 1 doesn't reap orphaned zombies (e.g. when
// PID 1 is a simple stub like "tail -f /dev/null"), zombies persist
// indefinitely, causing go-ps FindProcess to report the process as
// still alive.
func isZombie(pid int) bool {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// /proc/[pid]/stat format: pid (comm) state ...
	// Find the closing ')' to skip the comm field which may contain spaces
	// or parentheses.
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 || idx+2 >= len(s) {
		return false
	}
	state := strings.TrimSpace(s[idx+1:])
	return len(state) > 0 && state[0] == 'Z'
}
