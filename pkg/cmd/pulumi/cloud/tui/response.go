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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// responseState tracks where the Response tab is in its lifecycle.
type responseState int

const (
	responseIdle responseState = iota
	responseLoading
	responseReady
	responseError
)

// responseModel renders the Response tab: a header with status/timing,
// a jq filter textinput, and a scrollable viewport of the rendered body.
type responseModel struct {
	theme Theme

	state responseState

	spinner   spinner.Model
	body      viewport.Model
	filter    textinput.Model
	filtering bool

	// For paginated runs, the accumulated + rewrapped bytes.
	bytes []byte

	method string
	url    string
	status int
	size   int
	took   time.Duration
	page   int
	total  int // -1 if unknown

	// showHeaders is preserved across responses so users don't re-toggle each time.
	headers     http.Header
	showHeaders bool

	errorMsg string

	width, height int
}

func newResponseModel(theme Theme) responseModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	vp := viewport.New()

	ti := textinput.New()
	ti.Placeholder = "jq filter (e.g. .resources[].id)"
	ti.Prompt = "/ "

	return responseModel{
		theme:   theme,
		spinner: sp,
		body:    vp,
		filter:  ti,
		state:   responseIdle,
		total:   -1,
	}
}

func (m *responseModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.reflow()
}

// Called whenever the displayed composition changes so the body always fits in the pane.
func (m *responseModel) reflow() {
	if m.width == 0 || m.height == 0 {
		return
	}
	innerW, innerH := m.theme.Inside(m.width, m.height, m.theme.ContentPadding)
	m.body.SetWidth(innerW)
	m.filter.SetWidth(innerW - 2)

	reserved := 3 // method/url + status + trailing blank
	if m.total >= 0 {
		reserved++ // "pages: N  items: M"
	}
	if m.showHeaders && len(m.headers) > 0 {
		reserved += 1 + len(m.headers) + 1 // "Headers" title + entries + blank
	}
	if m.filtering {
		reserved += 2 // filter input + blank
	}
	h := innerH - reserved
	if h < 1 {
		h = 1
	}
	m.body.SetHeight(h)
}

func (m responseModel) Init() tea.Cmd { return nil }

func (m responseModel) Update(msg tea.Msg) (responseModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch m.state {
	case responseIdle:
	case responseLoading:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case responseReady, responseError:
		if m.filtering {
			m.filter, cmd = m.filter.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			m.body, cmd = m.body.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// StartLoading resets the model and enters the loading state. Returns the
// spinner's tick command so the outer model can batch it. Any jq filter
// from the previous response is cleared so the new body isn't
// immediately re-filtered against a stale expression.
func (m *responseModel) StartLoading(method, url string) tea.Cmd {
	m.state = responseLoading
	m.method = method
	m.url = url
	m.status = 0
	m.size = 0
	m.took = 0
	m.page = 0
	m.total = -1
	m.bytes = nil
	m.headers = nil
	m.errorMsg = ""
	m.body.SetContent("")
	m.filter.SetValue("")
	m.filter.Blur()
	m.filtering = false
	return m.spinner.Tick
}

// SetResponse marks the response as ready and primes the viewport.
func (m *responseModel) SetResponse(status int, took time.Duration, body []byte,
	headers http.Header, page, total int,
) {
	m.state = responseReady
	m.status = status
	m.size = len(body)
	m.took = took
	m.page = page
	m.total = total
	m.bytes = body
	m.headers = headers
	m.body.SetContent(renderBodyForView(body))
	m.reflow()
}

// ToggleHeaders flips whether response headers are rendered in the view.
// The preference sticks across subsequent dispatches.
func (m *responseModel) ToggleHeaders() {
	m.showHeaders = !m.showHeaders
	m.reflow()
}

// SetError transitions to the error state with msg shown in the viewport.
func (m *responseModel) SetError(err error) {
	m.state = responseError
	m.errorMsg = err.Error()
	m.body.SetContent(m.theme.Error.Render("Error: " + err.Error()))
}

// ToggleFilter flips the filter input on or off. When turning it on, the
// input is focused; when turning off, we reapply the filter (or clear it).
func (m *responseModel) ToggleFilter() tea.Cmd {
	m.filtering = !m.filtering
	m.reflow()
	if m.filtering {
		return m.filter.Focus()
	}
	m.filter.Blur()
	return nil
}

// ApplyFilter reruns the current jq expression against the accumulated
// response bytes and updates the viewport. Empty filter restores raw body.
//
// jq filtering is not currently wired up in the cloud package, so any
// non-empty expression renders an explanatory message instead of failing
// silently. The input field stays available so a future build with jq
// linked in works without UI changes.
func (m *responseModel) ApplyFilter() {
	expr := strings.TrimSpace(m.filter.Value())
	if expr == "" {
		m.body.SetContent(renderBodyForView(m.bytes))
		return
	}
	m.body.SetContent(m.theme.Error.Render("jq: filter unavailable in this build"))
}

func (m responseModel) View() string {
	var b strings.Builder

	switch m.state {
	case responseIdle:
		b.WriteString(m.theme.Dim.Render("\n  No response yet. Send a request from the Request tab (ctrl+s).\n"))
		return b.String()

	case responseLoading:
		fmt.Fprintf(&b, "\n  %s  %s %s\n", m.spinner.View(),
			m.theme.Dim.Render("sending"), m.url)
		fmt.Fprintf(&b, "\n  %s\n", m.theme.Dim.Render("esc or ctrl+c to cancel"))
		return b.String()

	case responseReady, responseError:
		statusStyle := m.theme.Success
		if m.status == 0 || m.status >= 400 {
			statusStyle = m.theme.Error
		}
		fmt.Fprintf(&b, "%s %s\n", m.theme.Accent.Render(m.method), m.url)
		fmt.Fprintf(&b, "%s  %s  %s\n",
			statusStyle.Render(fmt.Sprintf("HTTP %d", m.status)),
			m.theme.Dim.Render(humanSize(m.size)),
			m.theme.Dim.Render(m.took.Round(time.Millisecond).String()))
		// Only render the paging line when the request actually paginated
		// — single-shot dispatches set total=-1 to signal "not applicable".
		if m.total >= 0 {
			fmt.Fprintf(&b, "%s\n", m.theme.Dim.Render(fmt.Sprintf("pages: %d  items: %d", m.page, m.total)))
		}
		b.WriteString("\n")

		if m.showHeaders && len(m.headers) > 0 {
			b.WriteString(m.theme.Accent.Render("Headers") + "\n")
			b.WriteString(renderHeaders(m.theme, m.headers))
			b.WriteString("\n")
		}

		if m.filtering {
			b.WriteString(m.filter.View())
			b.WriteString("\n\n")
		}
		b.WriteString(m.body.View())
	}
	return b.String()
}

func renderHeaders(t Theme, h http.Header) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s %s\n",
			t.Dim.Render(k+":"),
			strings.Join(h[k], ", "))
	}
	return b.String()
}

// Pretty-prints JSON bodies; non-JSON is returned verbatim.
func renderBodyForView(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, b, "", "  "); err == nil {
		return pretty.String()
	}
	return string(b)
}

func humanSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
}
