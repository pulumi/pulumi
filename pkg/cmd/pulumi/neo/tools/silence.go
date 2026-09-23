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

package tools

import (
	"io"
	"os"
	"sync"
)

// stdSilencer isolates terminal output while an in-process engine operation runs
// under the Neo TUI. bubbletea owns os.Stdout (fd 1); the engine and any child
// processes it spawns (plugins, language hosts) would otherwise scribble on that
// same terminal. stdSilencer (a) swaps the os.Stdout/os.Stderr package variables
// to /dev/null so in-process fmt.Print* writes vanish, and (b) redirects the real
// stderr file descriptor (fd 2) into a capture pipe so child-process stderr can't
// reach the terminal. fd 1 is deliberately left alone so bubbletea keeps drawing.
//
// Swapping only the Go variables (the previous behavior) is not enough: child
// processes inherit fd 2 at the OS level, so noise like macOS libmalloc's
// "MallocStackLogging: can't turn off malloc stack logging" lands on the terminal
// regardless. Only an fd-level redirect contains it.
type stdSilencer struct {
	once       sync.Once
	null       *os.File
	origStdout *os.File
	origStderr *os.File

	savedFD2  int      // dup of the original fd 2; -1 when the fd redirect is inactive
	pipeW     *os.File // write end installed on fd 2
	captured  *cappedBuffer
	drainDone chan struct{}

	capturedStderr string
}

// silenceStd installs the redirections and returns a silencer whose Restore method
// reverts them. It is safe even if any individual step fails — whatever could not
// be installed is simply left untouched.
//
//nolint:forbidigo // Intentionally manipulating os.Stdout/os.Stderr.
func silenceStd() *stdSilencer {
	s := &stdSilencer{savedFD2: -1}

	// (a) Swap the Go stdout/stderr variables so in-process writes (e.g. the
	// engine's "Previewing update (stack)" banner) don't corrupt the TUI. This
	// only affects code that looks up os.Stdout/os.Stderr dynamically; it does
	// not touch fd 1, so bubbletea's captured handle keeps rendering.
	if null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		s.null = null
		s.origStdout, s.origStderr = os.Stdout, os.Stderr
		os.Stdout, os.Stderr = null, null
	}

	// (b) Redirect the process's real fd 2 into a pipe we drain. Child processes
	// inherit fd 2 at the OS level, so the variable swap above cannot keep their
	// stderr off the terminal — only an fd-level redirect does.
	if pr, pw, err := os.Pipe(); err == nil {
		saved, err := redirectFD2(pw)
		if err == nil {
			s.savedFD2 = saved
			s.pipeW = pw
			s.captured = &cappedBuffer{limit: maxOutputBytes}
			s.drainDone = make(chan struct{})
			go func() {
				defer close(s.drainDone)
				_, _ = io.Copy(s.captured, pr)
				_ = pr.Close()
			}()
		} else {
			// fd redirect unavailable (e.g. Windows); fall back to the variable
			// swap alone and discard the unused pipe.
			_ = pr.Close()
			_ = pw.Close()
		}
	}

	return s
}

// Restore reverts the redirections installed by silenceStd and returns any
// child-process stderr captured while fd 2 was redirected. It is idempotent; only
// the first call has an effect, and subsequent calls return the same captured text.
//
//nolint:forbidigo // Intentionally manipulating os.Stdout/os.Stderr.
func (s *stdSilencer) Restore() string {
	s.once.Do(func() {
		if s.null != nil {
			os.Stdout, os.Stderr = s.origStdout, s.origStderr
			_ = s.null.Close()
		}
		if s.savedFD2 >= 0 {
			// Put the real fd 2 back first so nothing else lands in the pipe,
			// then close the write end so the drain goroutine sees EOF.
			_ = restoreFD2(s.savedFD2)
			_ = s.pipeW.Close()
			<-s.drainDone
			s.capturedStderr = s.captured.buf.String()
		}
	})
	return s.capturedStderr
}
