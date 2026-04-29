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

package neo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// containsAllRunes returns true if every rune in want appears in s. Lipgloss
// wraps each rune in ANSI escape codes, so we can't compare against a literal
// rendered string — but each source rune still appears exactly once.
func containsAllRunes(s, want string) bool {
	for _, r := range want {
		if !strings.ContainsRune(s, r) {
			return false
		}
	}
	return true
}

func TestShimmerLabel_EmptyText(t *testing.T) {
	t.Parallel()

	assert.Empty(t, shimmerLabel("", shimmerVerb, 0))
	assert.Empty(t, shimmerLabel("", shimmerWave, 5))
}

func TestShimmerLabel_Dispatch(t *testing.T) {
	t.Parallel()

	// The sealing contract of shimmerKind is "wave or spotlight"; any unknown
	// kind (e.g. a future enum extended without updating callers) must still
	// render rather than crash or return empty. The default arm is
	// spotlight, so an out-of-range kind must match the spotlight output.
	const text = "hello"
	const frame = 3

	wave := shimmerLabel(text, shimmerWave, frame)
	spot := shimmerLabel(text, shimmerVerb, frame)
	unknown := shimmerLabel(text, shimmerKind(999), frame)

	// shimmerLabel is a thin dispatcher; each branch must return exactly what
	// the underlying builder returns. Comparing against the raw builders both
	// proves dispatch and pins the fall-through: an unknown kind must behave
	// like the spotlight (the zero value / default arm), not crash or fall
	// back to empty.
	assert.Equal(t, buildWave(text, frame), wave)
	assert.Equal(t, buildSpotlight(text, frame), spot)
	assert.Equal(t, spot, unknown, "unknown shimmerKind must fall back to spotlight")
}

func TestBuildSpotlight_EmptyText(t *testing.T) {
	t.Parallel()
	assert.Empty(t, buildSpotlight("", 0))
	assert.Empty(t, buildSpotlight("", 42))
}

func TestBuildSpotlight_PositionWraps(t *testing.T) {
	t.Parallel()

	// Every input rune must survive in the rendered output for every frame —
	// styling adds ANSI bytes but never drops characters. Check across a full
	// orbit (frame = 0 .. len-1) to exercise both the raw position and the
	// wrap-around branch (`wrap := n - diff; if wrap < diff`).
	const text = "Thinking"
	for frame := range len([]rune(text)) {
		got := buildSpotlight(text, frame)
		assert.Truef(t, containsAllRunes(got, text), "frame %d: missing runes from %q", frame, got)
	}
}

func TestBuildWave_EmptyText(t *testing.T) {
	t.Parallel()
	assert.Empty(t, buildWave("", 0))
	assert.Empty(t, buildWave("", 42))
}

func TestBuildWave_PreservesRunes(t *testing.T) {
	t.Parallel()

	const text = "read_file ..."
	for frame := range 20 {
		got := buildWave(text, frame)
		assert.Truef(t, containsAllRunes(got, text), "frame %d: missing runes from %q", frame, got)
	}
}

func TestBuildWave_CyclesWithPeriod(t *testing.T) {
	t.Parallel()

	// The wave position is frame mod (len(runes) + len(waveStyles)); one full
	// period must produce identical output. This locks the cycle length so a
	// refactor of the wave math (e.g. changing the lull length) can't silently
	// drift.
	text := "abcdef"
	period := len([]rune(text)) + len(waveStyles)
	assert.Equal(t, buildWave(text, 0), buildWave(text, period))
	assert.Equal(t, buildWave(text, 2), buildWave(text, 2+period))
}
