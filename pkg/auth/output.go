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

package auth

import (
	"io"
	"os"
	"sync"
)

type helperOutput struct {
	mu       sync.Mutex
	terminal io.Writer
	sink     func(string) bool
}

func (w *helperOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.sink != nil && w.sink(string(p)) {
		return len(p), nil
	}
	terminal := w.terminal
	if terminal == nil {
		terminal = os.Stderr
	}
	return terminal.Write(p)
}

// capture routes helper diagnostics to a renderer until the returned cleanup function is called.
// Cleanup restores terminal output and releases any helper waiting for the renderer to read its diagnostics.
func (w *helperOutput) capture() (<-chan string, func()) {
	messages := make(chan string)
	done := make(chan struct{})
	w.mu.Lock()
	previous := w.sink
	w.sink = func(message string) bool {
		select {
		case messages <- message:
			return true
		case <-done:
			return false
		}
	}
	w.mu.Unlock()
	return messages, func() {
		close(done)
		w.mu.Lock()
		w.sink = previous
		w.mu.Unlock()
	}
}
