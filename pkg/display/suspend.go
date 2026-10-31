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

package display

import "sync"

var (
	// mu guards suspender and is held for the whole of WhileSuspended, so that two prompts never use the terminal
	// at once.
	mu        sync.Mutex
	suspender func() (resume func())
)

// SetSuspender sets the function that pauses the active interactive display so that a prompt can use the terminal.
// Pass nil when the display closes.
func SetSuspender(suspend func() (resume func())) {
	mu.Lock()
	defer mu.Unlock()

	suspender = suspend
}

// WhileSuspended runs f with the active interactive display, if any, paused.
func WhileSuspended(f func() error) error {
	mu.Lock()
	defer mu.Unlock()

	if suspender != nil {
		resume := suspender()
		defer resume()
	}
	return f()
}
