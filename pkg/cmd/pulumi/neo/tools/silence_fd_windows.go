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

//go:build windows

package tools

import (
	"errors"
	"os"
)

// errFDRedirectUnsupported signals that fd-level stderr redirection is not wired
// up on this platform; silenceStd then falls back to swapping the os.Stdout/
// os.Stderr variables alone. The child-process stderr noise this guards against
// (macOS libmalloc warnings) does not occur on Windows, and Windows console
// redirection (SetStdHandle/DuplicateHandle) is left as a follow-up.
var errFDRedirectUnsupported = errors.New("fd-level stderr redirect not supported on this platform")

func redirectFD2(*os.File) (int, error) { return -1, errFDRedirectUnsupported }

func restoreFD2(int) error { return nil }
