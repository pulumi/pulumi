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

//go:build !linux

package cmdutil

// isZombie reports whether the process with the given PID is a zombie.
//
// On non-Linux platforms, zombie processes are either not a concern
// (Windows has no zombie concept) or are reaped promptly by the init
// system (macOS launchd). This stub always returns false.
func isZombie(int) bool {
	return false
}
