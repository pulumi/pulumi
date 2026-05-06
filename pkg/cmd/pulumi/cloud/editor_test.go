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

package cloud

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPickEditor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		env     map[string]string
		wantGOT string
	}{
		{"visual wins", map[string]string{"VISUAL": "nano", "EDITOR": "vim"}, "nano"},
		{"editor fallback", map[string]string{"EDITOR": "emacs"}, "emacs"},
		{"empty", map[string]string{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(k string) string { return tc.env[k] }
			got := PickEditor(getenv)
			want := tc.wantGOT
			if want == "" {
				if runtime.GOOS == "windows" {
					want = "notepad"
				} else {
					want = "vim"
				}
			}
			if got != want {
				t.Fatalf("PickEditor = %q, want %q", got, want)
			}
		})
	}
}

// TestEditInTempFile exercises the shell-out with a stub editor script that
// appends a marker line. The round-trip proves the helper writes the seed,
// runs the editor against the file, and reads the modified contents back.
func TestEditInTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub requires a POSIX shell")
	}
	t.Parallel()

	dir := t.TempDir()
	stub := filepath.Join(dir, "edit.sh")
	// The stub appends "EDITED\n" to whichever file it's given.
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf 'EDITED\\n' >> \"$1\"\n"), 0o600); err != nil {
		t.Fatalf("writing stub editor: %v", err)
	}
	if err := os.Chmod(stub, 0o700); err != nil {
		t.Fatalf("chmod stub editor: %v", err)
	}

	getenv := func(k string) string {
		if k == "EDITOR" {
			return stub
		}
		return ""
	}

	seed := []byte(`{"hello":"world"}`)
	out, err := EditInTempFile(seed, "pulumi-api-edit-*.json", getenv)
	if err != nil {
		t.Fatalf("EditInTempFile: %v", err)
	}
	got := string(out)
	if !strings.HasPrefix(got, `{"hello":"world"}`) {
		t.Fatalf("seed missing from result: %q", got)
	}
	if !strings.Contains(got, "EDITED") {
		t.Fatalf("editor marker missing from result: %q", got)
	}
}
