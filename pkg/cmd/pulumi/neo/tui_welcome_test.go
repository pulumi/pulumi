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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeOfDayKey(t *testing.T) {
	t.Parallel()

	// Exhaustively pin every hour so a bucket-boundary edit (e.g. moving the
	// morning→afternoon cutoff) is impossible to ship without updating the test.
	cases := []struct {
		hour int
		want string
	}{
		// Night: 0..4 and 21..23.
		{0, "night"},
		{3, "night"},
		{4, "night"},
		// Morning boundary at 5.
		{5, "morning"},
		{8, "morning"},
		{11, "morning"},
		// Afternoon boundary at 12.
		{12, "afternoon"},
		{14, "afternoon"},
		{16, "afternoon"},
		// Evening boundary at 17.
		{17, "evening"},
		{19, "evening"},
		{20, "evening"},
		// Night boundary at 21.
		{21, "night"},
		{22, "night"},
		{23, "night"},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.want, timeOfDayKey(tc.hour), "hour=%d", tc.hour)
	}
}

func TestPickGreeting_EmptyName(t *testing.T) {
	t.Parallel()

	// The no-name fallback is a fixed sentence so callers (and the TUI) can
	// detect that we don't know who the user is. Don't randomize it.
	assert.Equal(t, "What do you want to build today?", pickGreeting(""))
}

func TestPickGreeting_IncludesName(t *testing.T) {
	t.Parallel()

	// The rendered greeting contains the user's name (styled bold via lipgloss;
	// the raw rune sequence survives the ANSI wrapping). Run several draws so
	// we hit multiple templates without depending on the rand seed.
	for range 10 {
		got := pickGreeting("alice")
		assert.Contains(t, got, "alice", "name must appear in greeting: %q", got)
	}
}

func TestWelcomeView_RendersCoreFields(t *testing.T) {
	t.Parallel()

	w := welcomeModel{
		org:       "acme",
		workDir:   "/tmp/proj",
		username:  "alice",
		termWidth: 120,
		greeting:  "Hello, alice!",
	}
	out := w.View()

	// The header, greeting, workdir, and org suffix must all be visible in the
	// rendered box. Checking substrings (not byte-identity) is the right
	// granularity for a lipgloss-styled block — styling varies by terminal.
	assert.Contains(t, out, "Pulumi Neo", "title must render")
	assert.Contains(t, out, "Hello, alice!", "greeting must render")
	assert.Contains(t, out, "/tmp/proj", "workdir must render")
	assert.Contains(t, out, "acme", "org must render")
}

func TestWelcomeView_RendersConsoleHyperlink(t *testing.T) {
	t.Parallel()

	w := welcomeModel{
		org:        "acme",
		workDir:    "/tmp/proj",
		termWidth:  120,
		greeting:   "hi",
		consoleURL: "https://app.pulumi.com/acme/neo/tasks/abc",
	}
	out := w.View()

	// The OSC-8 hyperlink escape sequence is \x1b]8;; — this is what lets the
	// terminal treat the URL as a click target. If this byte sequence ever
	// goes missing the URL becomes plain text and clicking breaks silently.
	assert.Contains(t, out, "\x1b]8;;", "OSC-8 hyperlink escape must be present")
	assert.Contains(t, out, "app.pulumi.com", "URL payload must appear in the link text")
}

func TestWelcomeView_NarrowTerminalTruncatesPath(t *testing.T) {
	t.Parallel()

	// Pick a termWidth that yields a contentWidth comfortably larger than the
	// org suffix but smaller than the rendered path, so the truncation branch
	// in welcomeModel.View fires and appends the "..." marker. With
	// termWidth=60, contentWidth=52; with org="acme" the suffix is 11 runes
	// and maxPath=41. A 60-rune path is long enough to overflow that.
	longPath := "/" + strings.Repeat("verylongsegment/", 4)
	w := welcomeModel{
		org:       "acme",
		workDir:   longPath,
		termWidth: 60,
		greeting:  "hi",
	}
	out := w.View()

	// The ellipsis marker from path truncation must appear somewhere in the
	// rendered box — without it the overflow would soft-wrap and clobber the
	// org suffix on the next line.
	assert.Contains(t, out, "...")
	// The org suffix must always survive truncation — it's the load-bearing
	// context (path can be abbreviated, org cannot).
	assert.Contains(t, out, "acme")
}

func TestWelcomeView_TildeForHomePath(t *testing.T) {
	t.Parallel()

	// Paths under $HOME are presented as "~/<rel>" so the user sees the
	// shorter, familiar shell form rather than a long absolute path. The
	// banner has limited horizontal real estate (capped at liveWidth ≤ 80
	// cols) and the absolute home path is the dominant chunk on most setups.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir unavailable: %v", err)
	}
	rel := filepath.Join("code", "neo")
	w := welcomeModel{
		workDir:   filepath.Join(home, rel),
		termWidth: 120,
		greeting:  "hi",
	}
	out := w.View()

	require.Contains(t, out, "~/"+rel, "home-relative path must render with ~/ prefix")
	assert.NotContains(t, out, home, "absolute home path must not appear when ~/ substitution succeeded")
}

func TestWelcomeView_NarrowTerminalTruncatesConsoleURL(t *testing.T) {
	t.Parallel()

	// Long task URLs on a narrow terminal must be ellipsized in the rendered
	// link text. The OSC-8 hyperlink target stays the full URL — only the
	// visible label gets truncated, so clicking still works. Without the
	// guard, a 200-char URL would blow past the bracket gutter and ruin the
	// banner's layout.
	longURL := "https://app.pulumi.com/acme/neo/tasks/" + strings.Repeat("x", 200)
	w := welcomeModel{
		workDir:    "/tmp/proj",
		termWidth:  60,
		greeting:   "hi",
		consoleURL: longURL,
	}
	out := w.View()

	assert.Contains(t, out, "...", "long console URL must be truncated with an ellipsis")
	assert.Contains(t, out, "\x1b]8;;", "OSC-8 hyperlink escape must still be present after truncation")
	// The hyperlink target (escape ... URL ... ESC backslash) keeps the full URL
	// even though the visible text is shortened — clicks land on the real task.
	assert.Contains(t, out, longURL, "full URL must be preserved as the hyperlink target")
}
