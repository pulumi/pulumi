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

package securestore

import "errors"

// Outcome is the result of probing one backend. Fallback keys off the
// outcome, not off the raw error: only Absent and Locked mean "try something
// else".
type Outcome int

const (
	// Ready means the backend is present and usable right now.
	Ready Outcome = iota
	// Absent means there is nothing to use here: no provider, no session
	// bus, no TPM.
	Absent
	// Locked means the store exists but needs an unlock we were not
	// permitted to ask for.
	Locked
	// Declined means the store exists, we asked, and the user said no.
	Declined
)

func (o Outcome) String() string {
	switch o {
	case Ready:
		return "ready"
	case Absent:
		return "absent"
	case Locked:
		return "locked"
	case Declined:
		return "declined"
	default:
		return "unknown"
	}
}

func outcomeOf(err error) Outcome {
	switch {
	case err == nil:
		return Ready
	case errors.Is(err, ErrDeclined):
		return Declined
	case errors.Is(err, ErrLocked):
		return Locked
	default:
		return Absent
	}
}
