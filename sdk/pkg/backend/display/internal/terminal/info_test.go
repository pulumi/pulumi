// Copyright 2025-2025, Pulumi Corporation.
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
	"bytes"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfoXTerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	t.Parallel()
	info := OpenInfo("xterm")

	out := bytes.NewBuffer(nil)
	info.ClearLine(out)
	require.Equal(t, "\x1b[1K\x1b[K", out.String())

	out = bytes.NewBuffer(nil)
	info.ClearEnd(out)
	require.Equal(t, "\x1b[K", out.String())

	out = bytes.NewBuffer(nil)
	info.CarriageReturn(out)
	require.Equal(t, "\r", out.String())

	out = bytes.NewBuffer(nil)
	info.CursorUp(out, 3)
	require.Equal(t, "\x1b[3A", out.String())

	out = bytes.NewBuffer(nil)
	info.CursorDown(out, 3)
	require.Equal(t, "\x1b[3B", out.String())

	out = bytes.NewBuffer(nil)
	info.HideCursor(out)
	require.Equal(t, "\x1b[?25l", out.String())

	out = bytes.NewBuffer(nil)
	info.ShowCursor(out)
	require.Equal(t, "\x1b[?12l\x1b[?25h", out.String())
}

func TestInfoVT102(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}
	t.Parallel()

	info := OpenInfo("vt102")

	out := bytes.NewBuffer(nil)
	info.ClearLine(out)
	// VT102 has delays in the commands $<3>
	require.Equal(t, "\x1b[1K$<3>\x1b[K$<3>", out.String())
}

func TestInfoDefault(t *testing.T) {
	t.Parallel()
	info := &defaultInfo{}

	out := bytes.NewBuffer(nil)
	info.ClearLine(out)
	require.Equal(t, "\x1b[1K\x1b[K", out.String())

	out = bytes.NewBuffer(nil)
	info.ClearEnd(out)
	require.Equal(t, "\x1b[K", out.String())

	out = bytes.NewBuffer(nil)
	info.CarriageReturn(out)
	require.Equal(t, "\r", out.String())

	out = bytes.NewBuffer(nil)
	info.CursorUp(out, 3)
	require.Equal(t, "\x1b[3A", out.String())

	out = bytes.NewBuffer(nil)
	info.CursorDown(out, 3)
	require.Equal(t, "\x1b[3B", out.String())

	out = bytes.NewBuffer(nil)
	info.HideCursor(out)
	require.Equal(t, "", out.String())

	out = bytes.NewBuffer(nil)
	info.ShowCursor(out)
	require.Equal(t, "", out.String())
}
