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

package archive

import (
	"runtime"

	"golang.org/x/text/unicode/norm"
)

// precomposeUnicode returns name in NFC form on filesystems that decompose
// Unicode (macOS), and otherwise returns it unchanged. macOS stores filenames
// in a decomposed (roughly NFD) form, so the name "café" read back from readdir
// is a different byte sequence than the composed (NFC) "café" a user typically
// types into a .gitignore — and our pattern matching is byte/rune-exact. Git
// solves this with core.precomposeunicode, which precomposes readdir output to
// NFC and defaults to on for macOS; we mirror that gating so our ignore matching
// agrees with git's on the same filesystem.
//
// Normalizing only on macOS also mirrors git for correctness: on a decomposing
// filesystem the NFC name still resolves to the same file (lookup is
// normalization-insensitive), while on a byte-preserving filesystem rewriting
// the name could point at a file that doesn't exist.
func precomposeUnicode(name string) string {
	if runtime.GOOS != "darwin" {
		return name
	}
	return norm.NFC.String(name)
}
