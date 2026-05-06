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
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// EditInTempFile seeds a temp file with the given bytes, launches the user's
// editor against it, and returns the file contents on exit. filenamePattern
// follows os.CreateTemp's convention (e.g. "pulumi-cloud-api-body-*.json") so
// the suffix survives for editor syntax highlighting. getenv indirects
// environment lookups for test seams.
func EditInTempFile(seed []byte, filenamePattern string, getenv func(string) string) ([]byte, error) {
	f, err := os.CreateTemp("", filenamePattern)
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	path := f.Name()
	defer os.Remove(path)

	if _, err := f.Write(seed); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("seeding temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("closing temp file: %w", err)
	}

	editor := PickEditor(getenv)
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("running %s: %w", editor, err)
	}

	return os.ReadFile(path)
}

// PickEditor returns the command to invoke for interactive editing. Honors
// $VISUAL and $EDITOR before falling back to a platform-appropriate default.
func PickEditor(getenv func(string) string) string {
	if e := getenv("VISUAL"); e != "" {
		return e
	}
	if e := getenv("EDITOR"); e != "" {
		return e
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vim"
}
