// Copyright 2016-2024, Pulumi Corporation.
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
		info:   info{simpleTermInfo(0)},
		keys:   make(chan string),
		dest:   dest,
	}
}

// A mock terminal info implementation that prints out terminal codes as
// explicit strings for identification in test outputs.
type simpleTermInfo int

var _ termInfo = simpleTermInfo(0)

func (ti simpleTermInfo) Parse(attr string, params ...interface{}) (string, error) {
	return "", nil
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

func (t *SimpleTerminal) ClearLine() {
	t.info.ClearLine(t)
}

func (t *SimpleTerminal) ClearEnd() {
	t.info.ClearEnd(t)
}

func (t *SimpleTerminal) CarriageReturn() {
	// No op, as we don't have a cursor to move.
}

func (t *SimpleTerminal) CursorUp(count int) {
	t.info.CursorUp(t, count)
}

func (t *SimpleTerminal) CursorDown(count int) {
	t.info.CursorDown(t, count)
}

func (t *SimpleTerminal) HideCursor() {
	t.info.HideCursor(t)
}

func (t *SimpleTerminal) ShowCursor() {
	t.info.ShowCursor(t)
}

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
