// Copyright 2024, Pulumi Corporation.
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

package nosleep

type DoneFunc func()

// KeepRunning attempts to prevent idle sleep on the system.  This is useful for long running processes, e.g. updates
// that should not be interrupted by the system going to sleep.  It's not guaranteed to work on all systems or at all
// times.  Users can still manually put the system to sleep.
func KeepRunning() DoneFunc {
	return keepRunning()
}
