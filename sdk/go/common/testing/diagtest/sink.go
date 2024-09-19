// Copyright 2016-2023, Pulumi Corporation.
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

// Package diagtest provides testing utilities
// for code that uses the common/diag package.
package diagtest

import (
	"fmt"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/iotest"
)

// LogSink builds a diagnostic sink that logs to the given testing.TB.
//
// Messages are prefixed with [stdout] or [stderr]
// to indicate which stream they were written to.
func LogSink(t testing.TB) diag.Sink {
	return diag.DefaultSink(
		iotest.LogWriterPrefixed(t, "[stdout] "),
		iotest.LogWriterPrefixed(t, "[stderr] "),
		diag.FormatOptions{
			// Don't colorize test output.
			Color: colors.Never,
			Debug: true,
		},
	)
}

// Wraps a testing.TB and intercepts log messages.
type FakeT struct {
	testing.TB

	Msgs     []string
	Cleanups []func()
}

func (t *FakeT) Logf(msg string, args ...interface{}) {
	t.Msgs = append(t.Msgs, fmt.Sprintf(msg, args...))
}

func (t *FakeT) Cleanup(f func()) {
	t.Cleanups = append(t.Cleanups, f)
}

func (t *FakeT) RunCleanup() {
	// cleanup functions are called in reverse order.
	for i := len(t.Cleanups) - 1; i >= 0; i-- {
		t.Cleanups[i]()
	}
}
