// Copyright 2023, Pulumi Corporation.

package pager

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/acarl005/stripansi"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTeaPager(t *testing.T) (context.Context, *teatest.TestModel) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return ctx, teatest.NewTestModel(
		t,
		teaPager{cancel: cancel},
		teatest.WithInitialTermSize(80, 20),
	)
}

func genLines(count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		fmt.Fprintf(&b, "line %v\n", i+1)
	}
	return b.String()
}

func TestTeaPager(t *testing.T) {
	t.Run("no content", func(t *testing.T) {
		_, p := newTestTeaPager(t)
		p.Send(doneMsg{})
		err := p.Quit()
		require.NoError(t, err)

		final, err := io.ReadAll(p.FinalOutput(t))
		require.NoError(t, err)
		const expected = "\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n\r\n(end)"
		assert.Equal(t, expected, stripansi.Strip(string(final)))
	})

	t.Run("partial content", func(t *testing.T) {
		_, p := newTestTeaPager(t)

		p.Send(moreMsg{bytes: []byte(genLines(10))})
		err := p.Quit()
		require.NoError(t, err)

		final, err := io.ReadAll(p.FinalOutput(t))
		require.NoError(t, err)
		const expected = "line 1 \r\nline 2 \r\nline 3 \r\nline 4 \r\nline 5 \r\nline 6 \r\nline 7 \r\nline 8 \r\nline 9 \r\nline 10\r\n       \r\n       \r\n       \r\n       \r\n       \r\n       \r\n       \r\n       \r\n       \r\n(more)"
		assert.Equal(t, expected, stripansi.Strip(string(final)))
	})

	t.Run("extra content", func(t *testing.T) {
		_, p := newTestTeaPager(t)

		p.Send(moreMsg{bytes: []byte(genLines(40))})
		err := p.Quit()
		require.NoError(t, err)

		final, err := io.ReadAll(p.FinalOutput(t))
		require.NoError(t, err)
		const expected = "line 1 \r\nline 2 \r\nline 3 \r\nline 4 \r\nline 5 \r\nline 6 \r\nline 7 \r\nline 8 \r\nline 9 \r\nline 10\r\nline 11\r\nline 12\r\nline 13\r\nline 14\r\nline 15\r\nline 16\r\nline 17\r\nline 18\r\nline 19\r\n(more)"
		assert.Equal(t, expected, stripansi.Strip(string(final)))
	})

	t.Run("scrolled content", func(t *testing.T) {
		_, p := newTestTeaPager(t)

		lines := genLines(40)
		chunk0 := lines[:len(lines)/2]
		chunk1 := lines[len(lines)/2:]

		p.Send(moreMsg{bytes: []byte(chunk0)})
		p.Send(moreMsg{bytes: []byte(chunk1)})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(doneMsg{})
		err := p.Quit()
		require.NoError(t, err)

		final, err := io.ReadAll(p.FinalOutput(t))
		require.NoError(t, err)
		const expected = "line 5 \r\nline 6 \r\nline 7 \r\nline 8 \r\nline 9 \r\nline 10\r\nline 11\r\nline 12\r\nline 13\r\nline 14\r\nline 15\r\nline 16\r\nline 17\r\nline 18\r\nline 19\r\nline 20\r\nline 21\r\nline 22\r\nline 23\r\n(more)"
		assert.Equal(t, expected, stripansi.Strip(string(final)))
	})

	t.Run("scrolled to bottom", func(t *testing.T) {
		_, p := newTestTeaPager(t)

		p.Send(moreMsg{bytes: []byte(genLines(21))})
		p.Send(doneMsg{})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		p.Send(tea.KeyMsg{Type: tea.KeyDown})
		err := p.Quit()
		require.NoError(t, err)

		final, err := io.ReadAll(p.FinalOutput(t))
		require.NoError(t, err)
		const expected = "line 4 \r\nline 5 \r\nline 6 \r\nline 7 \r\nline 8 \r\nline 9 \r\nline 10\r\nline 11\r\nline 12\r\nline 13\r\nline 14\r\nline 15\r\nline 16\r\nline 17\r\nline 18\r\nline 19\r\nline 20\r\nline 21\r\n       \r\n(end)"
		assert.Equal(t, expected, stripansi.Strip(string(final)))
	})

}
