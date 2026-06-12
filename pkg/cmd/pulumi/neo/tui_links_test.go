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

func TestOSC8Hyperlink_WrapsURL(t *testing.T) {
	t.Parallel()

	got := osc8Hyperlink("https://example.com", "click me")
	// The OSC 8 wire format is `ESC ] 8 ; ; <url> ESC \ <text> ESC ] 8 ; ; ESC \`.
	// Pinning the exact bytes here so a refactor that drops one of the escape
	// terminators (a common mistake) gets caught immediately rather than
	// silently producing visible garbage in scrollback.
	assert.Equal(t, "\x1b]8;;https://example.com\x1b\\click me\x1b]8;;\x1b\\", got)
}

func TestOSC8Hyperlink_EmptyURLPassesText(t *testing.T) {
	t.Parallel()

	// With no URL we have nothing to hyperlink to — return the display text
	// unchanged rather than emitting an empty-target escape, which some
	// terminals render as a broken link.
	assert.Equal(t, "plain", osc8Hyperlink("", "plain"))
}

func TestLinkifyURLs_WrapsBareURL(t *testing.T) {
	t.Parallel()

	got := linkifyURLs("see https://example.com for details")
	assert.Contains(t, got, "\x1b]8;;https://example.com\x1b\\")
	assert.Contains(t, got, "https://example.com\x1b]8;;\x1b\\")
	assert.Contains(t, got, "for details")
}

func TestLinkifyURLs_StripsTrailingPunctuation(t *testing.T) {
	t.Parallel()

	// Sentence punctuation that follows a URL almost never belongs to it —
	// "see https://example.com." should hyperlink "https://example.com" and
	// leave the period outside, otherwise clicking opens the wrong URL.
	got := linkifyURLs("see https://example.com.")
	assert.Contains(t, got, "\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\.")
	assert.NotContains(t, got, "https://example.com.\x1b\\", "period must not be inside the hyperlink target")
}

func TestLinkifyURLs_HandlesParenthesizedURL(t *testing.T) {
	t.Parallel()

	got := linkifyURLs("(see https://example.com)")
	// The trailing ")" belongs to the surrounding prose, not the URL, so it
	// must land outside the hyperlink — otherwise terminals try to open
	// "example.com)" which 404s.
	assert.Contains(t, got, "\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\)")
}

func TestLinkifyURLs_WrapsMultipleURLs(t *testing.T) {
	t.Parallel()

	got := linkifyURLs("first https://a.example and second https://b.example")
	// Each URL gets its own complete OSC 8 envelope (opener + closer), so two
	// URLs means four `\x1b]8;;` occurrences — confirms we didn't accidentally
	// fuse the two URLs into one nested hyperlink or drop a closer.
	assert.Equal(t, 4, strings.Count(got, "\x1b]8;;"))
	assert.Contains(t, got, "https://a.example")
	assert.Contains(t, got, "https://b.example")
}

func TestLinkifyURLs_LeavesExistingOSC8Alone(t *testing.T) {
	t.Parallel()

	// If the text is already a hyperlink (e.g. the welcome banner already ran
	// it through osc8Hyperlink) we must not add a second pair of escapes —
	// nested OSC 8 sequences confuse some terminals into rendering nothing
	// clickable at all.
	already := osc8Hyperlink("https://example.com", "https://example.com")
	got := linkifyURLs(already)
	assert.Equal(t, already, got)
}

func TestLinkifyURLs_MixesPlainAndExisting(t *testing.T) {
	t.Parallel()

	// A line containing a pre-wrapped URL plus a bare URL should end up with
	// the pre-wrapped one untouched and the bare one freshly wrapped — two
	// hyperlinks total, not three (no double-wrapping).
	already := osc8Hyperlink("https://a.example", "a")
	input := already + " then https://b.example"
	got := linkifyURLs(input)
	// Two complete hyperlinks = 4 `\x1b]8;;` occurrences (each has an opener
	// and a closer). If linkify re-wrapped the existing one we'd see 6.
	assert.Equal(t, 4, strings.Count(got, "\x1b]8;;"))
	assert.Contains(t, got, already)
	assert.Contains(t, got, "\x1b]8;;https://b.example\x1b\\https://b.example\x1b]8;;\x1b\\")
}

func TestLinkifyURLs_NoURLs(t *testing.T) {
	t.Parallel()

	// Plain text without any URLs passes through unchanged. This is the hot
	// path for the vast majority of streamed assistant tokens, so it must
	// not introduce spurious escapes.
	const plain = "no urls here, just words"
	assert.Equal(t, plain, linkifyURLs(plain))
}

func TestLinkifyURLs_EmptyString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", linkifyURLs(""))
}

// hyperlink is a test helper that builds the exact OSC 8 envelope we expect
// linkifyURLs to emit for a given target URL and display text. Pinning the
// full byte sequence in each assertion catches regressions where one of the
// escape terminators gets dropped — a common refactor mistake that produces
// visible garbage rather than a silent wrong-link bug.
func hyperlink(target, display string) string {
	return "\x1b]8;;" + target + "\x1b\\" + display + "\x1b]8;;\x1b\\"
}

func TestLinkifyURLs_PreservesBalancedTrailingParens(t *testing.T) {
	t.Parallel()

	// Wikipedia disambiguation URLs end in a real ")" that closes a "(" inside
	// the URL. Stripping it would point clicks at a 404 — the bracket-balance
	// logic must keep balanced closers as part of the link target.
	const url = "https://en.wikipedia.org/wiki/Foo_(bar)"
	got := linkifyURLs("see " + url + " for more")
	assert.Contains(t, got, hyperlink(url, url))
}

func TestLinkifyURLs_PreservesParensInMiddleOfURL(t *testing.T) {
	t.Parallel()

	// Mid-URL parens have nothing to do with trailing-punct stripping, but
	// pin the case so a future change can't accidentally consume them while
	// scanning the tail.
	const url = "https://example.com/path(with)parens/more"
	assert.Equal(t, hyperlink(url, url), linkifyURLs(url))
}

func TestLinkifyURLs_PreservesMultipleBalancedGroups(t *testing.T) {
	t.Parallel()

	// Two balanced "()" groups: opens=2, closes=2, so the trailing ")" is
	// balanced and must stay in the URL.
	const url = "https://example.com/(a)/(b)"
	assert.Equal(t, hyperlink(url, url), linkifyURLs(url))
}

func TestLinkifyURLs_PreservesBalancedBracketsInURL(t *testing.T) {
	t.Parallel()

	// Same balance rule applies to "[]" — square brackets are rarer in URLs
	// (mostly IPv6 hosts in literal form) but the trim logic treats them
	// uniformly with parens, so pin the behavior.
	const url = "https://example.com/x[y]"
	assert.Equal(t, hyperlink(url, url), linkifyURLs(url))
}

func TestLinkifyURLs_StripsUnbalancedTrailingParen(t *testing.T) {
	t.Parallel()

	// The inner "(bar)" is balanced inside the URL and stays put; only the
	// outer ")" — opened by the prose's leading "(" — gets pushed back out.
	const url = "https://en.wikipedia.org/wiki/Foo_(bar)"
	got := linkifyURLs("(see " + url + ")")
	assert.Contains(t, got, hyperlink(url, url)+")")
	assert.NotContains(t, got, url+")\x1b\\", "outer paren must not be inside the link target")
}

func TestLinkifyURLs_StripsMultipleUnbalancedCloses(t *testing.T) {
	t.Parallel()

	// No "(" inside the URL means both trailing ")"s are unmatched and peel
	// off into the prose one at a time.
	got := linkifyURLs("https://example.com/foo))")
	assert.Contains(t, got, hyperlink("https://example.com/foo", "https://example.com/foo")+"))")
}

func TestLinkifyURLs_StripsCombinedTrailingPunct_PeriodInside(t *testing.T) {
	t.Parallel()

	// Strip order matters: outer ")" is unbalanced and goes first, then the
	// "." that's now exposed at the tail. Both end up in the trailing prose
	// in their original order.
	got := linkifyURLs("(https://example.com.)")
	assert.Contains(t, got, hyperlink("https://example.com", "https://example.com")+".)")
}

func TestLinkifyURLs_StripsCombinedTrailingPunct_PeriodAfterParen(t *testing.T) {
	t.Parallel()

	// "." strips first (always-strip punct), exposing a balanced ")" that
	// must stay in the URL. Naive "strip every trailing punct char" logic
	// would peel the ")" off here and break the link.
	const url = "https://example.com/(foo)"
	got := linkifyURLs(url + ".")
	assert.Contains(t, got, hyperlink(url, url)+".")
}

func TestLinkifyURLs_HandlesMixedPunctWithBalancedInner(t *testing.T) {
	t.Parallel()

	// Combined case: outer ")" peels first (closes=2 > opens=1), then ".",
	// leaving the inner "(foo)" balanced and intact. Exercises the loop's
	// recount-after-each-strip behavior.
	const url = "https://example.com/(foo)"
	got := linkifyURLs("(" + url + ".)")
	assert.Contains(t, got, hyperlink(url, url)+".)")
}

func TestLinkifyURLs_MultipleURLsBalanceIndependently(t *testing.T) {
	t.Parallel()

	// Two URLs in one line: the first must keep its balanced trailing ")",
	// the second must drop its trailing ".". This guards against a future
	// refactor that accidentally hoists trim state across matches.
	const wiki = "https://en.wikipedia.org/wiki/Foo_(bar)"
	const ex = "https://example.com"
	got := linkifyURLs("Compare " + wiki + " and " + ex + ".")
	assert.Contains(t, got, hyperlink(wiki, wiki))
	assert.Contains(t, got, hyperlink(ex, ex)+".")
	// 2 hyperlinks * (opener + closer) = 4 OSC 8 marker sequences. More
	// would mean a URL got double-wrapped; fewer would mean one got dropped.
	assert.Equal(t, 4, strings.Count(got, "\x1b]8;;"))
}
