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

package neo

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// inputBarHeight is the number of terminal lines reserved for the input area
// (separator + input line + hint line).
const inputBarHeight = 3

// blockKind identifies the type of rendered block in the output log.
type blockKind int

const (
	blockBusy blockKind = iota
	blockToolComplete
	blockAssistantStreaming
	blockAssistantFinal
	blockError
	blockWarning
	blockCancelled
	blockUserMessage
	blockApproval
)

// block is a single rendered item in the TUI output log.
type block struct {
	kind blockKind
	// rendered is the cached rendered string for non-busy kinds.
	rendered string
	// label is the text shown after the spinner for blockBusy only.
	label string
	// shimmer selects how label is animated; only meaningful for blockBusy.
	shimmer shimmerKind
}

// ModelConfig holds the parameters needed to create a TUI Model.
type ModelConfig struct {
	Org      string
	WorkDir  string
	Username string
	EventCh  <-chan UIEvent
	// OutCh carries every TUI-originated user event (chat messages, approval
	// answers, …) to the dispatcher in runNeo. Each send also carries the
	// TUI's current planMode, which the dispatcher reads on the first
	// user_message to configure CreateNeoTask.
	OutCh chan<- outboundEvent
	// Busy seeds the input-gating state. True when the caller has already
	// handed a prompt to the backend — the TUI starts with Enter disabled
	// until the first UITaskIdle.
	Busy bool
	// InitialPrompt, when non-empty, is rendered as the first user message
	// in the transcript and seeds the self-echo suppression queue. Use this
	// for the prompt passed on the command line (e.g. `pulumi neo "..."`)
	// which is sent to the backend via CreateNeoTask rather than outCh and
	// would otherwise only appear once the SSE stream echoes it back.
	InitialPrompt string
	// MessageSent seeds the post-first-message gate. Set it to true in tests
	// that want to exercise the Shift+Tab post-send warning path without
	// having to simulate a full Enter-driven send first.
	MessageSent bool
}

// Model is the top-level bubbletea model for the Neo TUI.
type Model struct {
	welcome   welcomeModel
	viewport  viewport.Model
	textInput textinput.Model
	blocks    []block
	eventCh   <-chan UIEvent
	outCh     chan<- outboundEvent
	// busy is true from the moment the user sends a message (or a prompt was
	// provided up front) until the session emits UITaskIdle / UICancelled /
	// UIError. While busy, Enter is swallowed so the user can't talk over
	// the agent and messages can't race task creation. The spinner animation
	// ticks for as long as busy is true.
	busy       bool
	spinner    spinner.Model
	mdRenderer *glamour.TermRenderer
	width      int
	height     int
	// frame advances on each spinner.TickMsg while busy and drives the
	// shimmer animation on the busy block's label.
	frame             int
	pendingApproval   bool
	pendingApprovalID string
	// pendingUserEchoes is a FIFO of user-message contents the TUI has
	// already rendered optimistically on submit. When a UIUserMessage
	// arrives (the server's echo of our own input) and the front of the
	// queue matches, it is popped and the redundant render is skipped.
	// Non-matching UIUserMessage events still render — they originated
	// from another client (e.g. the web UI).
	pendingUserEchoes []string
	// planMode is the user's current plan-mode choice, toggled via Shift+Tab.
	// Pre-first-message this is the live affordance; once messageSent flips
	// true, Shift+Tab stops toggling and planMode is effectively frozen
	// (further changes happen only via plan-approval auto-clear below).
	planMode bool
	// messageSent flips to true when the TUI successfully dispatches its
	// first user_message on outCh. From that point on, Shift+Tab emits the
	// "plan mode is task-level" warning instead of toggling.
	messageSent bool
	// pendingApprovalType is the raw wire approval_type for the currently
	// pending approval (empty when none). The Enter handler checks for
	// approvalTypePlanExit so it can auto-clear planMode on approval.
	pendingApprovalType string
}

var (
	inputSepStyle  = lipgloss.NewStyle().Faint(true)
	inputHintStyle = lipgloss.NewStyle().Faint(true)
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cancelledStyle = lipgloss.NewStyle().Faint(true)
	toolOKMarker   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("⏺")
	toolErrMarker  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⏺")
	finalMarker    = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render("⏺")
	userMsgBubble  = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("8"))
	// planAccentStyle is a distinct cyan+bold used for both the footer banner
	// and the "Proposed plan" block header so they read as the same visual cue.
	planAccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
)

// NewModel creates a new TUI Model.
func NewModel(cfg ModelConfig) Model {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.PromptStyle = promptStyle
	ti.Placeholder = "Send a message..."
	ti.Focus()
	ti.CharLimit = 4096

	vp := viewport.New(80, 24-inputBarHeight)

	sp := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6"))),
	)

	m := Model{
		welcome: welcomeModel{
			org:       cfg.Org,
			workDir:   cfg.WorkDir,
			username:  cfg.Username,
			termWidth: 80,
			greeting:  pickGreeting(cfg.Username),
		},
		viewport:    vp,
		textInput:   ti,
		eventCh:     cfg.EventCh,
		outCh:       cfg.OutCh,
		busy:        cfg.Busy,
		spinner:     sp,
		width:       80,
		height:      24,
		messageSent: cfg.MessageSent,
	}
	m.viewport.SetContent(m.welcome.View())
	if cfg.InitialPrompt != "" {
		m.appendUserMessageBlock(cfg.InitialPrompt)
		m.pendingUserEchoes = append(m.pendingUserEchoes, cfg.InitialPrompt)
	}
	if cfg.Busy {
		m.blocks = append(m.blocks, block{
			kind:    blockBusy,
			label:   pickThinkingVerb() + "...",
			shimmer: shimmerVerb,
		})
	}
	return m
}

// Init returns the initial command that starts listening for events.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{waitForEvent(m.eventCh), textinput.Blink}
	if m.busy {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.welcome.termWidth = msg.Width

		vpHeight := msg.Height - inputBarHeight
		if vpHeight < 1 {
			vpHeight = 1
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		m.textInput.Width = msg.Width - lipgloss.Width(m.textInput.Prompt) - 1

		// (Re)initialize the glamour renderer with the actual terminal width.
		if r, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(msg.Width-4),
		); err == nil {
			m.mdRenderer = r
		}
		m.rebuildContent()

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// Shift+Tab toggles plan mode. The toggle must run before the approval
		// and busy guards so users can flip the indicator at any point in the
		// pre-task window, even while the startup spinner is up. It also has to
		// intercept the key before textinput.Update sees it, since textinput
		// otherwise treats Shift+Tab as a keypress with no visible effect.
		if msg.Type == tea.KeyShiftTab {
			if m.messageSent {
				// Plan mode is task-level on the wire and gets snapshotted at
				// the moment the first message is sent. A post-send toggle
				// would be misleading — it could not affect the task.
				m.appendWarningBlock(
					"Plan mode is task-level — start a new `pulumi neo` session to change it.")
				m.rebuildContent()
				return m, nil
			}
			m.planMode = !m.planMode
			m.rebuildContent()
			return m, nil
		}

		// Handled before the busy check because the agent is intentionally
		// paused here waiting for the user.
		if m.pendingApproval {
			if msg.Type == tea.KeyEnter {
				text := strings.TrimSpace(m.textInput.Value())
				approved := strings.EqualFold(text, "y") || strings.EqualFold(text, "yes")
				var denialMsg string
				if !approved {
					denialMsg = text
				}
				wasPlanApproval := m.pendingApprovalType == approvalTypePlanExit
				if m.outCh != nil {
					select {
					case m.outCh <- outboundEvent{
						event: apitype.AgentUserEventUserConfirmation{
							Type:       userEventUserConfirmation,
							ApprovalID: m.pendingApprovalID,
							Approved:   approved,
							Message:    denialMsg,
						},
						planMode: m.planMode,
					}:
					default:
					}
				}
				m.pendingApproval = false
				m.pendingApprovalID = ""
				m.pendingApprovalType = ""
				// Approving a plan exits plan mode server-side (the PlanModeTracker
				// stops gating writes), so mirror that locally. Denial leaves the
				// mode on — the agent will re-plan and gate-out again on the next
				// exit_plan_mode call.
				if wasPlanApproval && approved {
					m.planMode = false
				}
				m.textInput.Prompt = "❯ "
				m.textInput.PromptStyle = promptStyle
				m.textInput.Placeholder = "Send a message..."
				m.textInput.Reset()
				choice := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓ Approved")
				if !approved {
					choice = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("✗ Denied")
					if denialMsg != "" {
						choice += " — " + denialMsg
					}
				}
				m.appendBlock(block{kind: blockUserMessage, rendered: "  " + choice})
				if approved {
					cmd := m.showBusy(pickThinkingVerb()+"...", shimmerVerb)
					m.rebuildContent()
					return m, cmd
				}
				m.rebuildContent()
				return m, nil
			}
			var tiCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)
			return m, tiCmd
		}

		if msg.Type == tea.KeyEnter {
			if m.busy {
				// Agent is mid-turn — leave the typed text in the input so
				// the user can send it after the next UITaskIdle.
				return m, nil
			}
			text := strings.TrimSpace(m.textInput.Value())
			if text != "" {
				m.textInput.Reset()
				if m.outCh != nil {
					select {
					case m.outCh <- outboundEvent{
						event: apitype.AgentUserEventUserMessage{
							Type:    userEventUserMessage,
							Content: text,
						},
						planMode: m.planMode,
					}:
						// Render optimistically so the user sees their message in
						// the transcript before the server echoes it back. The
						// echo is reconciled against pendingUserEchoes in the
						// UIUserMessage handler to avoid duplicates.
						m.appendUserMessageBlock(text)
						m.pendingUserEchoes = append(m.pendingUserEchoes, text)
						// Freeze the plan-mode affordance: planMode has now been
						// committed to the dispatcher and any later Shift+Tab
						// would be a no-op on the server.
						m.messageSent = true
						return m, m.showBusy(pickThinkingVerb()+"...", shimmerVerb)
					default:
					}
				}
			}
			return m, nil
		}

		// Pass to text input first for typing.
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		cmds = append(cmds, tiCmd)

		// Also pass to viewport for scrolling (pgup/pgdn/etc).
		var vpCmd tea.Cmd
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// Advance the shimmer animation in lockstep with the spinner glyph.
		// Modulo a large bound keeps the int from growing unbounded across
		// long sessions; the value itself only matters mod label length.
		m.frame = (m.frame + 1) & 0x3fffffff
		m.rebuildContent()
		return m, cmd

	case UIAssistantMessage:
		m.removeBlockKind(blockBusy)
		if msg.IsFinal {
			m.removeBlockKind(blockAssistantStreaming)
			rendered := m.renderMarkdown(msg.Content)
			m.appendBlock(block{
				kind:     blockAssistantFinal,
				rendered: renderAssistantFinal(rendered),
			})
		} else if idx := m.findBlockKind(blockAssistantStreaming); idx >= 0 {
			m.blocks[idx].rendered = renderAssistantStreaming(msg.Content)
		} else {
			m.appendBlock(block{
				kind:     blockAssistantStreaming,
				rendered: renderAssistantStreaming(msg.Content),
			})
		}
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolStarted:
		if cmd := m.showBusy(toolLabel(msg.Name, msg.Args)+" ...", shimmerWave); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolProgress:
		if cmd := m.showBusy(toolLabel(msg.Name, nil)+": "+truncate(msg.Message, 60), shimmerWave); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolCompleted:
		marker := toolOKMarker
		if msg.IsError {
			marker = toolErrMarker
		}
		m.appendBlock(block{
			kind:     blockToolComplete,
			rendered: "  " + marker + " " + styledToolLabel(msg.Name, msg.Args),
		})
		// Keep the busy block alive across the inter-tool gap so the spinner
		// stays visible while the agent decides its next move.
		if cmd := m.showBusy(pickThinkingVerb()+"...", shimmerVerb); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIError:
		m.endBusy()
		m.appendBlock(block{
			kind:     blockError,
			rendered: "  " + errorStyle.Render("✗ Error: "+msg.Message),
		})
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIWarning:
		m.appendWarningBlock(msg.Message)
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UICancelled:
		m.endBusy()
		m.appendBlock(block{
			kind:     blockCancelled,
			rendered: "  " + cancelledStyle.Render("Session cancelled."),
		})
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UITaskIdle:
		m.endBusy()
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UISessionURL:
		m.welcome.consoleURL = msg.URL
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIUserMessage:
		// If the queue's front matches, this is the server echoing a message
		// we already rendered optimistically on submit — pop and skip.
		// Otherwise it came from another client (e.g. the web UI) and we
		// render it normally.
		if len(m.pendingUserEchoes) > 0 && m.pendingUserEchoes[0] == msg.Content {
			m.pendingUserEchoes = m.pendingUserEchoes[1:]
		} else {
			m.appendUserMessageBlock(msg.Content)
			m.rebuildContent()
		}
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIApprovalRequest:
		m.endBusy()
		m.pendingApproval = true
		m.pendingApprovalID = msg.ApprovalID
		m.pendingApprovalType = msg.ApprovalType
		if m.pendingApprovalType == approvalTypePlanExit {
			// Plan bodies are authored as markdown — headings, bullet lists, code
			// blocks. Route through the same glamour renderer used for final
			// assistant messages so the approval reads as a proper plan document
			// rather than a single-line warning. The approval event's `message`
			// field holds a generic intro ("I've finished exploring..."); the
			// actual plan is in `context.plan_description`.
			header := planAccentStyle.Render("⏺ Proposed plan")
			body := m.renderMarkdown(msg.PlanDescription)
			m.appendBlock(block{
				kind:     blockApproval,
				rendered: renderHeaderedBlock(header, body),
			})
			m.textInput.Prompt = "Approve plan? [y to approve / reason to deny]: "
		} else {
			m.appendBlock(block{
				kind:     blockApproval,
				rendered: "  " + warningStyle.Render("⚠ Approval required") + "\n    " + msg.Message,
			})
			m.textInput.Prompt = "Approve? [y to approve / reason to deny]: "
		}
		m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		m.textInput.Placeholder = ""
		m.textInput.Reset()
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	default:
		// Pass unhandled messages to textinput (e.g. blink).
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		cmds = append(cmds, tiCmd)
	}

	return m, tea.Batch(cmds...)
}

// View returns the rendered TUI: viewport on top, input bar at the bottom.
func (m Model) View() string {
	sep := inputSepStyle.Render(strings.Repeat("─", m.width))
	// The hint stays on a single line to keep inputBarHeight constant; the
	// plan-mode indicator is prepended inline when active so the viewport sizing
	// code doesn't need to track hint-line count.
	hintText := "enter to send · shift+tab to toggle plan mode · ctrl+c to quit"
	if m.busy {
		hintText = "agent is working · enter disabled · ctrl+c to quit"
	}
	hint := "  "
	if m.planMode {
		hint += planAccentStyle.Render("⏸ plan mode")
		hintText = " · " + hintText
	}
	hint += inputHintStyle.Render(hintText)

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		sep,
		m.textInput.View(),
		hint,
	)
}

// rebuildContent concatenates the welcome box and all blocks into the viewport.
// The busy block's spinner glyph is read from m.spinner.View() at render time
// so the animation tracks the current frame without re-caching per block.
func (m *Model) rebuildContent() {
	parts := []string{m.welcome.View()}
	for _, b := range m.blocks {
		if b.kind == blockBusy {
			parts = append(parts, "  "+m.spinner.View()+" "+shimmerLabel(b.label, b.shimmer, m.frame))
		} else {
			parts = append(parts, b.rendered)
		}
	}

	wasAtBottom := m.viewport.AtBottom()
	m.viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, parts...))
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

// showBusy ensures the busy indicator is the last block, with the given
// label and shimmer style, and the spinner is ticking. Always remove-then-
// append so blockBusy is guaranteed to be at the bottom regardless of prior
// state. Returns the spinner Tick cmd if we weren't already busy; nil
// otherwise. Callers batch the return value into their cmds.
func (m *Model) showBusy(label string, shimmer shimmerKind) tea.Cmd {
	m.removeBlockKind(blockBusy)
	m.blocks = append(m.blocks, block{kind: blockBusy, label: label, shimmer: shimmer})
	if m.busy {
		return nil
	}
	m.busy = true
	return m.spinner.Tick
}

// appendWarningBlock appends a standard "⚠ msg" warning block to the transcript.
// Used by UIWarning and by local-only warnings (e.g. the Shift+Tab post-send
// warning) so both paths share the same rendering.
func (m *Model) appendWarningBlock(msg string) {
	m.appendBlock(block{
		kind:     blockWarning,
		rendered: "  " + warningStyle.Render("⚠ "+msg),
	})
}

// appendUserMessageBlock renders a user's chat message as a styled bubble and
// appends it to the transcript. Used both for optimistic rendering on submit
// and for echoes that originated outside this TUI.
func (m *Model) appendUserMessageBlock(content string) {
	m.appendBlock(block{
		kind:     blockUserMessage,
		rendered: promptStyle.Render("❯") + " " + userMsgBubble.Render(" "+content+" "),
	})
}

// appendBlock appends a non-busy block, keeping any existing blockBusy
// pinned at the bottom.
func (m *Model) appendBlock(b block) {
	if idx := m.findBlockKind(blockBusy); idx >= 0 {
		m.blocks = append(m.blocks[:idx], append([]block{b}, m.blocks[idx:]...)...)
		return
	}
	m.blocks = append(m.blocks, b)
}

// endBusy clears the busy flag (the spinner drops its next tick) and
// removes the busy indicator block. Resets the shimmer frame so the next
// busy session starts clean rather than mid-wave.
func (m *Model) endBusy() {
	m.busy = false
	m.frame = 0
	m.removeBlockKind(blockBusy)
}

func (m *Model) removeBlockKind(kind blockKind) {
	filtered := m.blocks[:0]
	for _, b := range m.blocks {
		if b.kind != kind {
			filtered = append(filtered, b)
		}
	}
	m.blocks = filtered
}

// findBlockKind returns the index of the last block of the given kind, or -1.
func (m *Model) findBlockKind(kind blockKind) int {
	for i := len(m.blocks) - 1; i >= 0; i-- {
		if m.blocks[i].kind == kind {
			return i
		}
	}
	return -1
}

// renderMarkdown renders text through glamour, falling back to plain text.
func (m *Model) renderMarkdown(text string) string {
	if m.mdRenderer == nil {
		return text
	}
	rendered, err := m.mdRenderer.Render(text)
	if err != nil {
		return text
	}
	return strings.TrimRight(rendered, "\n")
}

// renderHeaderedBlock renders "  header" followed by body indented by 4 spaces,
// matching the visual style of tool/assistant/plan blocks in the transcript. If
// body is empty only the header line is returned, so callers can pass the split
// first line of some content as the header and the remainder as the body.
func renderHeaderedBlock(header, body string) string {
	first := "  " + header
	if strings.TrimSpace(body) == "" {
		return first
	}
	indented := lipgloss.NewStyle().MarginLeft(4).Render(strings.TrimRight(body, "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, first, indented)
}

// renderAssistantFinal renders a final assistant message with a white circle marker.
func renderAssistantFinal(rendered string) string {
	trimmed := strings.TrimLeft(rendered, "\n ")
	if trimmed == "" {
		return ""
	}
	firstLine, rest, _ := strings.Cut(trimmed, "\n")
	return renderHeaderedBlock(finalMarker+" "+firstLine, rest)
}

// renderAssistantStreaming renders streaming text with a dim indicator.
func renderAssistantStreaming(text string) string {
	if text == "" {
		return ""
	}
	return "  " + text
}

// waitForEvent returns a tea.Cmd that reads from the UIEvent channel.
// When the channel closes, it returns tea.Quit.
func waitForEvent(ch <-chan UIEvent) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return evt
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
