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
	"regexp"
	"strings"
)

// osc8Hyperlink wraps displayText as a clickable hyperlink to url using the
// OSC 8 escape sequence. Terminals that support OSC 8 render displayText as
// a click target; terminals that don't strip the escapes and show displayText
// as plain text.
//
// The wire format is `ESC ] 8 ; ; <url> ESC \ <text> ESC ] 8 ; ; ESC \`.
func osc8Hyperlink(url, displayText string) string {
	if url == "" {
		return displayText
	}
	return "\x1b]8;;" + url + "\x1b\\" + displayText + "\x1b]8;;\x1b\\"
}

// urlPattern matches bare http/https URLs in text. It excludes whitespace,
// ANSI escape (so we never split a URL across an escape boundary), and
// angle/quote characters that commonly bound URLs in prose.
var urlPattern = regexp.MustCompile(`https?://[^\s\x1b<>"']+`)

// osc8Pattern matches a complete OSC 8 hyperlink sequence — the opener with
// its URL, the visible text, and the closer. linkifyURLs uses it to skip
// past existing hyperlinks rather than re-wrapping URLs that are already
// clickable.
var osc8Pattern = regexp.MustCompile(`\x1b\]8;;[^\x1b]*\x1b\\[^\x1b]*\x1b\]8;;\x1b\\`)

// urlAlwaysStripPunct lists characters we unconditionally strip from the tail
// of a matched URL — they're sentence punctuation that practically never
// appears at the end of a real URL (e.g. "see https://example.com.").
const urlAlwaysStripPunct = ".,:;!?"

// urlTrailingBrackets maps each closing bracket we may need to peel off the
// tail of a matched URL to its matching opener. We only strip a trailing
// closer when it's unmatched inside the URL — that way "(see https://x.com)"
// loses the outer ")" but "https://en.wikipedia.org/wiki/Foo_(bar)" keeps
// the balanced one.
var urlTrailingBrackets = map[byte]byte{
	')': '(',
	']': '[',
	'}': '{',
}

// linkifyURLs scans text for bare http/https URLs and wraps each one in an
// OSC 8 hyperlink so terminals that support it render the URL as clickable.
// Existing OSC 8 sequences are passed through untouched, so calling this on
// already-linkified output is safe.
func linkifyURLs(text string) string {
	if text == "" {
		return text
	}
	// Walk the string, splitting on existing OSC 8 sequences so we only
	// linkify the gaps between them. Without this, a URL already wrapped
	// upstream (e.g. by the welcome banner or a future glamour version)
	// would get a second, nested set of escapes.
	var b strings.Builder
	idx := 0
	for _, span := range osc8Pattern.FindAllStringIndex(text, -1) {
		b.WriteString(linkifyPlain(text[idx:span[0]]))
		b.WriteString(text[span[0]:span[1]])
		idx = span[1]
	}
	b.WriteString(linkifyPlain(text[idx:]))
	return b.String()
}

// linkifyPlain wraps every bare URL in s with an OSC 8 hyperlink. Trailing
// sentence punctuation is left outside the hyperlink so a click on
// "https://example.com." doesn't try to open "example.com.".
func linkifyPlain(s string) string {
	return urlPattern.ReplaceAllStringFunc(s, func(match string) string {
		match, trail := trimURLTail(match)
		if match == "" {
			return trail
		}
		return osc8Hyperlink(match, match) + trail
	})
}

// trimURLTail peels sentence punctuation off the end of a URL match. Plain
// punctuation (.,:;!?) always strips. A closing bracket only strips if it's
// unmatched inside what remains of the URL — so balanced brackets stay part
// of the link target (e.g. Wikipedia disambiguation URLs like
// "https://en.wikipedia.org/wiki/Foo_(bar)") while orphan closers from the
// surrounding prose ("(see https://x.com)") get pushed back out.
func trimURLTail(url string) (link, trail string) {
	for len(url) > 0 {
		last := url[len(url)-1]
		if strings.IndexByte(urlAlwaysStripPunct, last) >= 0 {
			trail = string(last) + trail
			url = url[:len(url)-1]
			continue
		}
		if opener, ok := urlTrailingBrackets[last]; ok {
			// Count both brackets in the full current match (which still
			// includes the trailing closer). closes > opens means the closer
			// at the end has no partner inside the URL, so it belongs to the
			// surrounding text.
			if strings.Count(url, string(last)) > strings.Count(url, string(opener)) {
				trail = string(last) + trail
				url = url[:len(url)-1]
				continue
			}
		}
		break
	}
	return url, trail
}
