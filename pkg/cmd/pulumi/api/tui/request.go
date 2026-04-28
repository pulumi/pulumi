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
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/api"
)

// requestModel renders the Request tab: a stack of textinputs for the
// path + query parameters, a huh.MultiSelect for opting into optional
// query params, and (when the op accepts one) a body textarea. Focus
// cycles through the widgets via tab / shift+tab. We manage focus
// ourselves rather than deferring to huh.Form so every field is visible
// at once and the list can grow or shrink as the user toggles options.
type requestModel struct {
	theme Theme
	cfg   api.TUIConfig

	op *api.Operation

	// Path + query state, sized to op. Values live inside the inputs —
	// we read them back via resolvedPathValues / resolvedQueryValues
	// when dispatching or rendering the URL preview.
	pathNames   []string
	pathInputs  []textinput.Model
	queryParams []api.ParamSpec
	queryInputs []textinput.Model

	// queryEnabled is on the heap via a pointer so huh's accessor survives requestModel
	// being copied by value — binding &m.field would leave the accessor pointing at an
	// orphaned struct.
	toggles      *huh.MultiSelect[string]
	queryEnabled *[]string

	// focusIdx is the index into visibleFields(); -1 means nothing focused.
	focusIdx int

	hasBody  bool
	body     textarea.Model
	bodySkel string

	ready bool

	paginate bool

	width, height int

	// statusMsg is a raw (unstyled) one-line hint; statusErr selects
	// success vs. error styling at render time.
	statusMsg string
	statusErr bool
}

func newRequestModel(theme Theme, cfg api.TUIConfig) requestModel {
	return requestModel{theme: theme, cfg: cfg, focusIdx: -1}
}

// SetOp (re)initialises the inputs for a newly-selected Operation.
// Previous field values are discarded; defaults are re-seeded from
// api.InteractiveDefault.
func (m *requestModel) SetOp(op *api.Operation) {
	m.op = op
	m.statusMsg = ""
	m.statusErr = false

	m.pathNames = api.PathParamsInOrder(op)
	m.pathInputs = make([]textinput.Model, len(m.pathNames))
	for i, n := range m.pathNames {
		ti := textinput.New()
		ti.Prompt = fmt.Sprintf("  {%s} ", n)
		ti.SetValue(api.InteractiveDefault(n, m.cfg))
		ti.CharLimit = 256
		m.pathInputs[i] = ti
	}

	m.queryParams = m.queryParams[:0]
	for _, p := range op.Params {
		if p.In == "query" {
			m.queryParams = append(m.queryParams, p)
		}
	}
	m.queryInputs = make([]textinput.Model, len(m.queryParams))
	for i, p := range m.queryParams {
		ti := textinput.New()
		ti.Prompt = fmt.Sprintf("  ?%s = ", p.Name)
		if !p.Required {
			ti.Placeholder = "(leave blank to omit)"
		}
		ti.CharLimit = 512
		m.queryInputs[i] = ti
	}

	// Build the optional-query multiselect.
	var opts []huh.Option[string]
	for _, p := range m.queryParams {
		if p.Required {
			continue
		}
		label := p.Name
		if d := strings.TrimSpace(p.Description); d != "" {
			if len(d) > 60 {
				d = d[:59] + "\u2026"
			}
			label = p.Name + " \u2014 " + d
		}
		opts = append(opts, huh.NewOption(label, p.Name))
	}
	m.queryEnabled = new([]string)
	if len(opts) > 0 {
		m.toggles = huh.NewMultiSelect[string]().
			Title("Optional query parameters").
			Options(opts...).
			Value(m.queryEnabled).
			Height(min(len(opts)+2, 10))
		// Standalone huh fields ship with an empty keymap, so keys look inert without this.
		m.toggles.WithKeyMap(huh.NewDefaultKeyMap())
	} else {
		m.toggles = nil
	}

	m.hasBody = op.HasBody
	if m.hasBody {
		m.bodySkel = api.BodySkeletonSeed(op)
		ta := textarea.New()
		ta.SetValue(m.bodySkel)
		ta.SetHeight(8)
		m.body = ta
	}

	m.focusIdx = -1
	m.setFocus(0)
	m.ready = true
}

// visibleField holds a reference to the currently-focusable widget at a
// given index. Exactly one of input / toggles / body is non-nil.
type visibleField struct {
	input   *textinput.Model
	toggles *huh.MultiSelect[string]
	body    *textarea.Model
}

// visibleFields returns the ordered list of focusable widgets in their
// render order: path inputs, then the toggles (if any), then query
// inputs for required + enabled optional params, then the body textarea
// (if the op accepts a body).
func (m *requestModel) visibleFields() []visibleField {
	fs := make([]visibleField, 0, len(m.pathInputs)+len(m.queryInputs)+2)
	for i := range m.pathInputs {
		fs = append(fs, visibleField{input: &m.pathInputs[i]})
	}
	if m.toggles != nil {
		fs = append(fs, visibleField{toggles: m.toggles})
	}
	for i, p := range m.queryParams {
		if !m.includeQueryParam(p) {
			continue
		}
		fs = append(fs, visibleField{input: &m.queryInputs[i]})
	}
	if m.hasBody {
		fs = append(fs, visibleField{body: &m.body})
	}
	return fs
}

// setFocus blurs whichever field is currently active and focuses the one
// at newIdx. Returns any cmd the focused widget wants to run (cursor
// blink). Out-of-range indices are ignored.
func (m *requestModel) setFocus(newIdx int) tea.Cmd {
	fs := m.visibleFields()
	if len(fs) == 0 {
		m.focusIdx = -1
		return nil
	}
	var cmds []tea.Cmd
	if m.focusIdx >= 0 && m.focusIdx < len(fs) {
		cur := fs[m.focusIdx]
		if cur.input != nil {
			cur.input.Blur()
		}
		if cur.toggles != nil {
			if c := cur.toggles.Blur(); c != nil {
				cmds = append(cmds, c)
			}
		}
		if cur.body != nil {
			cur.body.Blur()
		}
	}
	if newIdx < 0 || newIdx >= len(fs) {
		m.focusIdx = -1
		return tea.Batch(cmds...)
	}
	m.focusIdx = newIdx
	next := fs[newIdx]
	if next.input != nil {
		if c := next.input.Focus(); c != nil {
			cmds = append(cmds, c)
		}
	}
	if next.toggles != nil {
		if c := next.toggles.Focus(); c != nil {
			cmds = append(cmds, c)
		}
	}
	if next.body != nil {
		if c := next.body.Focus(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// advanceFocus moves focus by delta, wrapping around the visible field
// list. After the multiselect is toggled the list may have grown or
// shrunk; advanceFocus always re-reads visibleFields so indices stay
// consistent with the current layout.
func (m *requestModel) advanceFocus(delta int) tea.Cmd {
	fs := m.visibleFields()
	n := len(fs)
	if n == 0 {
		return nil
	}
	from := m.focusIdx
	if from < 0 {
		from = 0
	}
	next := ((from+delta)%n + n) % n
	return m.setFocus(next)
}

// isQueryEnabled reports whether an optional query parameter name is
// currently toggled on via the multiselect.
func (m *requestModel) isQueryEnabled(name string) bool {
	if m.queryEnabled == nil {
		return false
	}
	return slices.Contains(*m.queryEnabled, name)
}

// includeQueryParam reports whether a query param should appear in the
// resolved URL and be sent with the request. Required params are always
// included; optional ones only when the user has selected them.
func (m *requestModel) includeQueryParam(p api.ParamSpec) bool {
	return p.Required || m.isQueryEnabled(p.Name)
}

// resolvedQueryValues collects the effective query string the user has
// configured: only params we're set to include, with non-empty values.
func (m *requestModel) resolvedQueryValues() url.Values {
	q := url.Values{}
	for i, p := range m.queryParams {
		if !m.includeQueryParam(p) {
			continue
		}
		if v := strings.TrimSpace(m.queryInputs[i].Value()); v != "" {
			q.Set(p.Name, v)
		}
	}
	return q
}

// resolvedURL renders a preview of the request URL for the status line.
// Unfilled path params keep their `{name}` placeholder so the preview
// stays readable — e.g. `/api/stacks/{orgName}/…` rather than
// `/api/stacks///…`.
func (m *requestModel) resolvedURL() string {
	if m.op == nil {
		return ""
	}
	path := m.op.Path
	for i, n := range m.pathNames {
		if v := strings.TrimSpace(m.pathInputs[i].Value()); v != "" {
			path = strings.ReplaceAll(path, "{"+n+"}", v)
		}
	}
	if enc := m.resolvedQueryValues().Encode(); enc != "" {
		path += "?" + enc
	}
	return fmt.Sprintf("%s %s", m.op.Method, path)
}

// resolvedPathValues returns the path-parameter values in their declared
// order. Used by the outer model to build the dispatch request.
func (m *requestModel) resolvedPathValues() []string {
	out := make([]string, len(m.pathInputs))
	for i := range m.pathInputs {
		out[i] = strings.TrimSpace(m.pathInputs[i].Value())
	}
	return out
}

// editBodyCmd shells out to $EDITOR on the current body, reads the
// result back, and updates the textarea. The editor runs via
// tea.ExecProcess so bubbletea releases the terminal while the editor
// owns it — otherwise the raw-mode / altscreen state fights with
// full-screen TUIs like vim and corrupts both programs.
func (m *requestModel) editBodyCmd() tea.Cmd {
	if !m.hasBody {
		return nil
	}
	current := []byte(m.body.Value())

	f, err := os.CreateTemp("", "pulumi-cloud-api-body-*.json")
	if err != nil {
		return func() tea.Msg { return bodyEditedMsg{err: fmt.Errorf("temp file: %w", err)} }
	}
	path := f.Name()
	if _, werr := f.Write(current); werr != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return func() tea.Msg { return bodyEditedMsg{err: fmt.Errorf("seeding temp file: %w", werr)} }
	}
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(path)
		return func() tea.Msg { return bodyEditedMsg{err: fmt.Errorf("closing temp file: %w", cerr)} }
	}

	editor := api.PickEditor(os.Getenv)
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		defer os.Remove(path)
		if runErr != nil {
			return bodyEditedMsg{err: fmt.Errorf("%s: %w", editor, runErr)}
		}
		edited, rerr := os.ReadFile(path)
		if rerr != nil {
			return bodyEditedMsg{err: rerr}
		}
		return bodyEditedMsg{content: string(edited)}
	})
}

// bodyEditedMsg is sent after the external editor exits.
type bodyEditedMsg struct {
	content string
	err     error
}

// SetSize forwards a new viewport size to the inputs and body textarea.
// Magic sizes are derived from the theme's ContentPadding and PaneBorder
// styles so if the theme's padding changes, these follow automatically.
func (m *requestModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	innerW, _ := m.theme.Inside(width, height, m.theme.ContentPadding)
	for i := range m.pathInputs {
		m.pathInputs[i].SetWidth(innerW - 4)
	}
	for i := range m.queryInputs {
		m.queryInputs[i].SetWidth(innerW - 4)
	}
	if m.hasBody {
		bodyInnerW, _ := m.theme.Inside(innerW, 0, m.theme.PaneBorder)
		m.body.SetWidth(bodyInnerW)
	}
}

func (m requestModel) Init() tea.Cmd { return nil }

func (m requestModel) Update(msg tea.Msg) (requestModel, tea.Cmd) {
	if !m.ready || m.op == nil {
		return m, nil
	}

	switch msg := msg.(type) {
	case bodyEditedMsg:
		if msg.err != nil {
			m.statusMsg = "editor: " + msg.err.Error()
			m.statusErr = true
			return m, nil
		}
		m.body.SetValue(msg.content)
		m.statusMsg = "body updated from editor"
		m.statusErr = false
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "tab":
			return m, m.advanceFocus(1)
		case "shift+tab":
			return m, m.advanceFocus(-1)
		}
	}

	fs := m.visibleFields()
	if m.focusIdx < 0 || m.focusIdx >= len(fs) {
		return m, nil
	}
	target := fs[m.focusIdx]
	var cmd tea.Cmd
	if target.input != nil {
		*target.input, cmd = target.input.Update(msg)
		return m, cmd
	}
	if target.toggles != nil {
		updated, c := target.toggles.Update(msg)
		// Type-assert back because Update returns through the huh.Model interface.
		// The widget mutates *m.queryEnabled in place, so visibleFields() picks up changes.
		if ms, ok := updated.(*huh.MultiSelect[string]); ok {
			m.toggles = ms
		}
		return m, c
	}
	if target.body != nil {
		*target.body, cmd = target.body.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m requestModel) View() string {
	if !m.ready || m.op == nil {
		return m.theme.Dim.Render("\n  Select an endpoint in the Browse tab to begin.\n")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", m.theme.Accent.Render(m.op.Method+" "+m.op.Path))
	if m.op.Summary != "" {
		fmt.Fprintf(&b, "%s\n", m.theme.Dim.Render(m.op.Summary))
	}
	b.WriteString("\n")

	if len(m.pathNames) > 0 {
		b.WriteString(m.theme.Accent.Render("Path parameters") + "\n")
		for i := range m.pathInputs {
			req := m.theme.Error.Render(" *")
			v := strings.TrimSpace(m.pathInputs[i].Value())
			marker := ""
			if v == "" {
				marker = req
			}
			b.WriteString(m.pathInputs[i].View() + marker + "\n")
		}
		b.WriteString("\n")
	}

	if len(m.queryParams) > 0 {
		b.WriteString(m.theme.Accent.Render("Query parameters") + "\n")
		if m.toggles != nil {
			b.WriteString(m.toggles.View())
			b.WriteString("\n")
		}
		for i, p := range m.queryParams {
			if !m.includeQueryParam(p) {
				continue
			}
			req := ""
			if p.Required && strings.TrimSpace(m.queryInputs[i].Value()) == "" {
				req = m.theme.Error.Render(" *")
			}
			b.WriteString(m.queryInputs[i].View() + req + "\n")
			if d := strings.TrimSpace(p.Description); d != "" {
				if len(d) > 80 {
					d = d[:79] + "\u2026"
				}
				b.WriteString(m.theme.Dim.Render("    "+d) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if m.hasBody {
		hint := "tab to focus · ctrl+e to edit in $EDITOR"
		if m.body.Focused() {
			hint = "typing here · tab to move on · ctrl+e for $EDITOR"
		}
		label := m.theme.Dim.Render("\u25B8 Request body (" + hint + ")")
		b.WriteString(label + "\n")
		b.WriteString(m.theme.PaneBorder.Render(m.body.View()))
		b.WriteString("\n\n")
	}

	b.WriteString(m.theme.Accent.Render("\u2192 ") + m.resolvedURL() + "\n")
	if m.statusMsg != "" {
		style := m.theme.Success
		if m.statusErr {
			style = m.theme.Error
		}
		b.WriteString(style.Render(m.statusMsg) + "\n")
	}
	if m.paginate {
		b.WriteString(m.theme.Dim.Render("--paginate") + "\n")
	}
	return m.theme.ContentPadding.Render(b.String())
}
