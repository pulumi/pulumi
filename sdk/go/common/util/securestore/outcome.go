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

import (
	"errors"
	"sync"
)

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

// memoizePrecheck caches a precheck per allowPrompt value. One command probes
// several times — reading the existing file, checking stickiness, then
// writing — and without this each probe of a locked store would raise its own
// dialog, including re-asking after the user already declined.
func memoizePrecheck(probe func(allowPrompt bool) (Outcome, error)) func(bool) (Outcome, error) {
	var (
		once [2]sync.Once
		res  [2]struct {
			outcome Outcome
			err     error
		}
	)
	return func(allowPrompt bool) (Outcome, error) {
		i := 0
		if allowPrompt {
			i = 1
		}
		once[i].Do(func() { res[i].outcome, res[i].err = probe(allowPrompt) })
		return res[i].outcome, res[i].err
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
