// Copyright 2016-2025, Pulumi Corporation.
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

package terminal

import (
	"io"
	"sync"
)

// SimpleTerminal is a terminal which ignores terminal codes and writes its
// output to a buffer.
type SimpleTerminal struct {
	m sync.Mutex

	width, height int
	raw           bool
	info          Info

	keys chan string

	dest io.Writer
}

var _ Terminal = &SimpleTerminal{}

func NewSimpleTerminal(dest io.Writer, width, height int) *SimpleTerminal {
	return &SimpleTerminal{
		width:  width,
		height: height,
		raw:    true,
		info:   nil,
		keys:   make(chan string),
		dest:   dest,
	}
}

func (t *SimpleTerminal) IsRaw() bool {
	return t.raw
}

func (t *SimpleTerminal) Close() error {
	close(t.keys)
	return nil
}

func (t *SimpleTerminal) Size() (width, height int, err error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.width, t.height, nil
}

func (t *SimpleTerminal) Write(b []byte) (int, error) {
	return t.dest.Write(b)
}

func (t *SimpleTerminal) ClearLine()           {}
func (t *SimpleTerminal) ClearEnd()            {}
func (t *SimpleTerminal) CarriageReturn()      {}
func (t *SimpleTerminal) CursorUp(count int)   {}
func (t *SimpleTerminal) CursorDown(count int) {}
func (t *SimpleTerminal) HideCursor()          {}
func (t *SimpleTerminal) ShowCursor()          {}

func (t *SimpleTerminal) ReadKey() (string, error) {
	k, ok := <-t.keys
	if !ok {
		return "", io.EOF
	}
	return k, nil
}

func (t *SimpleTerminal) SetSize(width, height int) {
	t.m.Lock()
	defer t.m.Unlock()

	t.width, t.height = width, height
}

func (t *SimpleTerminal) SendKey(key string) {
	t.keys <- key
}
