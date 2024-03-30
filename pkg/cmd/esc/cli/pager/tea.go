// Copyright 2023, Pulumi Corporation.

package pager

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type moreWriter struct {
	send func(m tea.Msg)
}

func (w moreWriter) Write(b []byte) (int, error) {
	c := make([]byte, len(b))
	copy(c, b)

	w.send(moreMsg{bytes: c})
	return len(b), nil
}

type doneMsg struct{}

type moreMsg struct {
	bytes []byte
}

type teaPager struct {
	cancel   func()
	content  strings.Builder
	ready    bool
	done     bool
	viewport viewport.Model
}

func runTeaPager(stdout io.Writer, f func(context.Context, io.Writer) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := tea.NewProgram(
		teaPager{cancel: cancel},
		tea.WithOutput(stdout),
		tea.WithAltScreen(),       // use the full size of the terminal in its "alternate screen buffer"
		tea.WithMouseCellMotion(), // turn on mouse support so we can track the mouse wheel
	)

	done := make(chan error)
	go func() {
		done <- func() error {
			defer p.Send(doneMsg{})
			return f(ctx, moreWriter{send: p.Send})
		}()
	}()

	if _, pagerErr := p.Run(); pagerErr != nil {
		return fmt.Errorf("running pager: %w", pagerErr)
	}
	cancel()

	return <-done
}

func (p teaPager) Init() tea.Cmd {
	return nil
}

func (p teaPager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case doneMsg:
		p.done = true

	case moreMsg:
		p.content.Write(msg.bytes)
		p.viewport.SetContent(p.content.String())

	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return p, tea.Quit
		}

	case tea.WindowSizeMsg:
		footerHeight := lipgloss.Height(p.footerView())
		verticalMarginHeight := footerHeight

		if !p.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			p.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			p.viewport.SetContent(p.content.String())
			p.ready = true
		} else {
			p.viewport.Width = msg.Width
			p.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	// Handle keyboard and mouse events in the viewport
	p.viewport, cmd = p.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return p, tea.Batch(cmds...)
}

func (p teaPager) View() string {
	if !p.ready {
		return "Initializing..."
	}
	return fmt.Sprintf("%s\n%s", p.viewport.View(), p.footerView())
}

func (p teaPager) footerView() string {
	if p.done && p.viewport.AtBottom() {
		return "(end)"
	}
	return "(more)"
}
