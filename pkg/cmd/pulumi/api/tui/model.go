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

package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/api"
)

// rootModel is the outer Bubble Tea model. It owns the active-tab state,
// the three tab sub-models, and orchestrates message routing between them.
type rootModel struct {
	ctx context.Context
	idx *api.Index
	cfg api.TUIConfig

	theme  Theme
	keymap keyMap

	mode mode

	browse   browseModel
	request  requestModel
	response responseModel

	width, height int

	// Per-request context, so esc/ctrl+c aborts a slow or hung API call
	// without waiting for the goroutine to unwind.
	dispatchCancel context.CancelFunc

	// On quit, stdoutOnExit (if non-nil) is written to stdout and exitStatus
	// is the HTTP status the outer runner propagates as the exit code.
	stdoutOnExit []byte
	exitStatus   int

	// TUI-level error translated into an APIError envelope after Run() returns.
	lastError error
}

func newRootModel(ctx context.Context, idx *api.Index, cfg api.TUIConfig) rootModel {
	theme := newTheme()
	return rootModel{
		ctx:      ctx,
		idx:      idx,
		cfg:      cfg,
		theme:    theme,
		keymap:   newKeyMap(),
		mode:     modeBrowse,
		browse:   newBrowseModel(theme, idx),
		request:  newRequestModel(theme, cfg),
		response: newResponseModel(theme),
	}
}

// Init starts the TUI with any per-sub-model initial commands.
func (m rootModel) Init() tea.Cmd {
	return tea.Batch(
		m.browse.Init(),
		m.request.Init(),
		m.response.Init(),
	)
}

// Update routes messages to the active tab and handles global keybindings.
func (m rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		paneHeight := msg.Height - 4 // tab bar + footer + 2 lines padding
		m.browse.SetSize(msg.Width, paneHeight)
		m.request.SetSize(msg.Width, paneHeight)
		m.response.SetSize(msg.Width, paneHeight)
		return m, nil

	case tea.KeyPressMsg:
		// Global keys are Ctrl-modified so they don't collide with typing in inputs.
		switch {
		case key.Matches(msg, m.keymap.Quit):
			// Cancel any in-flight dispatch so the HTTP goroutine unwinds
			// instead of stranding the program while tea.Quit drains cmds.
			if m.dispatchCancel != nil {
				m.dispatchCancel()
				m.dispatchCancel = nil
			}
			return m, tea.Quit
		case key.Matches(msg, m.keymap.Back):
			// In modeBrowse, Esc has local meaning — cancel the list's
			// filter input or pop the drilled-in category view back to the
			// categories list. Let it flow to browseModel instead of being
			// consumed here.
			if m.mode != modeBrowse {
				return m.handleBack()
			}
		}

		switch m.mode {
		case modeBrowse:
			if key.Matches(msg, m.keymap.Select) {
				if op := m.browse.SelectedOp(); op != nil {
					m.request.SetOp(op)
					m.mode = modeRequest
					return m, nil
				}
			}
		case modeRequest:
			switch {
			case key.Matches(msg, m.keymap.Send):
				return m.startDispatch(false)
			case key.Matches(msg, m.keymap.SendToStdout):
				return m.startDispatch(true)
			case key.Matches(msg, m.keymap.TogglePaging):
				m.request.paginate = !m.request.paginate
				return m, nil
			case key.Matches(msg, m.keymap.EditBody):
				return m, m.request.editBodyCmd()
			}
		case modeResponse:
			// When the filter input is focused, keystrokes belong to the
			// textinput — letting the '/' shortcut fire would clobber
			// whatever the user is typing into their jq expression.
			// Enter applies the filter; everything else falls through to
			// the sub-model.
			if m.response.filtering {
				if msg.String() == "enter" {
					m.response.ApplyFilter()
					return m, nil
				}
				break
			}
			if key.Matches(msg, m.keymap.Filter) {
				return m, m.response.ToggleFilter()
			}
			if key.Matches(msg, m.keymap.Headers) {
				m.response.ToggleHeaders()
				return m, nil
			}
		}

	case dispatchResultMsg:
		return m.handleDispatchResult(msg, false)

	case dispatchExitMsg:
		return m.handleDispatchResult(dispatchResultMsg(msg), true)

	case bodyEditedMsg:
		var cmd tea.Cmd
		m.request, cmd = m.request.Update(msg)
		return m, cmd

	case tea.MouseClickMsg:
		// Tab bar is always at y==0; a click there is a tab switch, not a pane interaction.
		if handled, newModel := m.handleTabClick(msg.Mouse()); handled {
			return newModel, nil
		}

	case tea.MouseReleaseMsg:
		// Swallow tab-row releases so they don't trip pane-level selection.
		if msg.Mouse().Y == 0 {
			return m, nil
		}

	case tea.MouseMotionMsg:
		// Drop tab-row motion to avoid spurious selection changes in the list below.
		if msg.Mouse().Y == 0 {
			return m, nil
		}
	}

	// Forward everything else to the active tab.
	var cmd tea.Cmd
	switch m.mode {
	case modeBrowse:
		m.browse, cmd = m.browse.Update(msg)
	case modeRequest:
		m.request, cmd = m.request.Update(msg)
	case modeResponse:
		m.response, cmd = m.response.Update(msg)
	}
	return m, cmd
}

func (m rootModel) handleBack() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeBrowse:
	case modeRequest:
		m.mode = modeBrowse
	case modeResponse:
		// Escaping a loading response cancels the HTTP call too, otherwise
		// the spinner would tick until the request eventually completed.
		if m.dispatchCancel != nil {
			m.dispatchCancel()
			m.dispatchCancel = nil
			m.response.SetError(context.Canceled)
		}
		m.mode = modeRequest
	}
	return m, nil
}

// handleTabClick checks whether a click lands on the tab bar row and, if
// so, switches to the clicked tab. Returns handled=true when the click was
// on the tab row, so callers know to swallow it rather than fall through
// to the active pane.
func (m rootModel) handleTabClick(mouse tea.Mouse) (bool, tea.Model) {
	if mouse.Y != 0 {
		return false, m
	}
	_, hits := renderTabs(m.theme, m.mode)
	for _, hit := range hits {
		if mouse.X < hit.startCol || mouse.X >= hit.endCol {
			continue
		}
		// Populate the Request form when the user clicks the tab instead of pressing Enter.
		// Skip the reset if op hasn't changed, otherwise re-clicking Request wipes fields.
		if hit.mode == modeRequest {
			if op := m.browse.SelectedOp(); op != nil && op != m.request.op {
				m.request.SetOp(op)
			}
		}
		m.mode = hit.mode
		return true, m
	}
	return true, m
}

// startDispatch builds the request, installs the loading spinner in the
// Response tab, and kicks off the HTTP call. toStdout=true means: on
// completion, quit the program and write the response to stdout.
func (m rootModel) startDispatch(toStdout bool) (tea.Model, tea.Cmd) {
	if !m.request.ready || m.request.op == nil {
		return m, nil
	}

	req, err := m.buildDispatchRequest()
	if err != nil {
		m.request.statusMsg = err.Error()
		m.request.statusErr = true
		return m, nil
	}

	// Cancel any in-flight dispatch so its late result doesn't clobber the new one.
	if m.dispatchCancel != nil {
		m.dispatchCancel()
	}
	dispatchCtx, cancel := context.WithCancel(m.ctx)
	m.dispatchCancel = cancel

	m.mode = modeResponse
	cmd := m.response.StartLoading(req.method, formatURL(req))

	dispatch := runDispatch(dispatchCtx, req)
	if toStdout {
		return m, tea.Batch(cmd, func() tea.Msg {
			r := dispatch().(dispatchResultMsg)
			return dispatchExitMsg(r)
		})
	}
	return m, tea.Batch(cmd, dispatch)
}

func (m rootModel) buildDispatchRequest() (dispatchRequest, error) {
	op := m.request.op

	pathMap := map[string]string{}
	pathVals := m.request.resolvedPathValues()
	for i, name := range m.request.pathNames {
		pathMap[name] = pathVals[i]
	}
	concretePath := api.SubstituteInteractivePath(op, pathMap)

	query := m.request.resolvedQueryValues()

	var body []byte
	contentType := ""
	if m.request.hasBody {
		body = []byte(m.request.body.Value())
		contentType = op.BodyContentType
		if contentType == "" {
			contentType = "application/json"
		}
	}

	accept := "application/json"
	if op.ResponseContentType != "" {
		accept = op.ResponseContentType
	}

	resolved, err := api.ResolveContext(m.ctx, m.cfg.Org, false)
	if err != nil {
		return dispatchRequest{}, fmt.Errorf("auth: %w", err)
	}
	client := api.NewAPIClient(resolved.CloudURL, resolved.Token)

	return dispatchRequest{
		client:      client,
		method:      op.Method,
		path:        concretePath,
		query:       query,
		body:        body,
		contentType: contentType,
		accept:      accept,
		paginate:    m.request.paginate,
		op:          op,
	}, nil
}

// formatURL formats METHOD URL?query for the Response tab header.
func formatURL(req dispatchRequest) string {
	u := req.path
	if enc := req.query.Encode(); enc != "" {
		u += "?" + enc
	}
	return u
}

func (m rootModel) handleDispatchResult(msg dispatchResultMsg, exitAfter bool) (tea.Model, tea.Cmd) {
	// Cancelled dispatches clear dispatchCancel; drop the late result so it doesn't
	// overwrite the "cancelled" error or flip us out of the tab the user navigated to.
	if m.dispatchCancel == nil && !exitAfter {
		return m, nil
	}
	m.dispatchCancel = nil
	if msg.err != nil {
		m.response.SetError(msg.err)
	} else {
		m.response.SetResponse(msg.status, msg.took, msg.body, msg.headers, msg.page, msg.items)
	}
	m.mode = modeResponse

	if exitAfter {
		m.stdoutOnExit = msg.body
		m.exitStatus = msg.status
		m.lastError = msg.err
		return m, tea.Quit
	}
	return m, nil
}

// View renders the whole TUI: tab bar, active tab content, footer.
// Shift-click in most terminals bypasses mouse capture for native
// selection, so we leave cell-motion on unconditionally.
func (m rootModel) View() tea.View {
	if m.width == 0 || m.height == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		v.MouseMode = tea.MouseModeCellMotion
		return v
	}

	tabs, _ := renderTabs(m.theme, m.mode)

	var body string
	switch m.mode {
	case modeBrowse:
		body = m.browse.View()
	case modeRequest:
		body = m.request.View()
	case modeResponse:
		body = m.response.View()
	}

	footer := m.theme.Footer.Render(m.footer())

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, tabs, body, footer))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m rootModel) footer() string {
	hints := []string{}
	add := func(b key.Binding) {
		k, h := b.Keys(), b.Help()
		if len(k) == 0 {
			return
		}
		hints = append(hints, m.theme.Key.Render(h.Key)+" "+m.theme.KeyDesc.Render(h.Desc))
	}
	add(m.keymap.Back)
	add(m.keymap.Quit)

	switch m.mode {
	case modeBrowse:
		add(m.keymap.Select)
	case modeRequest:
		add(m.keymap.Send)
		add(m.keymap.SendToStdout)
		if m.request.hasBody {
			add(m.keymap.EditBody)
		}
		add(m.keymap.TogglePaging)
	case modeResponse:
		add(m.keymap.Filter)
		add(m.keymap.Headers)
	}

	return strings.Join(hints, "   ")
}
