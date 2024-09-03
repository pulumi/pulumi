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
	"fmt"
	"io"
	"strings"
	"sync"
)

type MockTerminal struct {
	m sync.Mutex

	width, height int
	raw           bool
	info          Info

	keys chan string

	dest io.Writer
}

var _ Terminal = &MockTerminal{}

func NewMockTerminal(dest io.Writer, width, height int, raw bool) *MockTerminal {
	return &MockTerminal{
		width:  width,
		height: height,
		raw:    raw,
		info:   info{mockTermInfo(0)},
		keys:   make(chan string),
		dest:   dest,
	}
}

// A mock terminal info implementation that prints out terminal codes as
// explicit strings for identification in test outputs.
type mockTermInfo int

var termOps = map[string]string{
	"el1":   "clear-to-cursor",
	"el":    "clear-to-end",
	"cuu":   "cursor-up",
	"cud":   "cursor-down",
	"civis": "hide-cursor",
	"cnorm": "show-cursor",
}

func (ti mockTermInfo) Parse(attr string, params ...interface{}) (string, error) {
	opName, ok := termOps[attr]
	if !ok {
		opName = attr
	}

	if len(params) == 0 {
		return fmt.Sprintf("<%%%s%%>", opName), nil
	}

	// If the operation has parameters, format them all as a colon-delimited
	// string, e.g. "cursor-up:2".
	var op strings.Builder
	op.WriteString(opName)
	for _, p := range params {
		op.WriteRune(':')
		op.WriteString(fmt.Sprint(p))
	}

	return fmt.Sprintf("<%%%s%%>", op.String()), nil
}

func (t *MockTerminal) IsRaw() bool {
	return t.raw
}

func (t *MockTerminal) Close() error {
	close(t.keys)
	return nil
}

func (t *MockTerminal) Size() (width, height int, err error) {
	t.m.Lock()
	defer t.m.Unlock()

	return t.width, t.height, nil
}

func (t *MockTerminal) Write(b []byte) (int, error) {
	return t.dest.Write(b)
}

func (t *MockTerminal) ClearLine() {
	t.info.ClearLine(t)
}

func (t *MockTerminal) ClearEnd() {
	t.info.ClearEnd(t)
}

func (t *MockTerminal) CarriageReturn() {
	t.info.CarriageReturn(t)
}

func (t *MockTerminal) CursorUp(count int) {
	t.info.CursorUp(t, count)
}

func (t *MockTerminal) CursorDown(count int) {
	t.info.CursorDown(t, count)
}

func (t *MockTerminal) HideCursor() {
	t.info.HideCursor(t)
}

func (t *MockTerminal) ShowCursor() {
	t.info.ShowCursor(t)
}

func (t *MockTerminal) ReadKey() (string, error) {
	k, ok := <-t.keys
	if !ok {
		return "", io.EOF
	}
	return k, nil
}

func (t *MockTerminal) SetSize(width, height int) {
	t.m.Lock()
	defer t.m.Unlock()

	t.width, t.height = width, height
}

func (t *MockTerminal) SendKey(key string) {
	t.keys <- key
}
