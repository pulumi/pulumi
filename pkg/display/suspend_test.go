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

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // mutates the package-level suspender
func TestWhileSuspended(t *testing.T) {
	var calls []string
	f := func() error {
		calls = append(calls, "f")
		return errors.New("boom")
	}

	assert.EqualError(t, WhileSuspended(f), "boom")
	assert.Equal(t, []string{"f"}, calls)

	calls = nil
	SetSuspender(func() func() {
		calls = append(calls, "suspend")
		return func() { calls = append(calls, "resume") }
	})
	assert.EqualError(t, WhileSuspended(f), "boom")
	assert.Equal(t, []string{"suspend", "f", "resume"}, calls)

	calls = nil
	SetSuspender(nil)
	assert.EqualError(t, WhileSuspended(f), "boom")
	assert.Equal(t, []string{"f"}, calls)
}
