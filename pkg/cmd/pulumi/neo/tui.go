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
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// inputBarHeight is the number of terminal lines reserved for the input area
// (separator + input line + hint line).
const inputBarHeight = 3

// ctrlCArmTimeout is how long the "press Ctrl+C again to exit" gate stays
// armed after the first press. Matches the cadence other agent CLIs use so
// the second press still has to be deliberate but the gate doesn't silently
// linger across long idle periods.
const ctrlCArmTimeout = 1500 * time.Millisecond

// ctrlCDisarmMsg is the deferred disarm signal scheduled when the user first
// presses Ctrl+C. It carries the generation it was scheduled under; the
// handler ignores it if the user has since re-armed (gen advanced) or
// already disarmed by typing another key.
type ctrlCDisarmMsg struct {
	gen int
}

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
	blockApprovalPlan
	blockApprovalGeneral
	blockApprovalChoice
	blockPulumiOp
)

type block struct {
	kind     blockKind
	rendered string
	// raw is the source content for width-dependent kinds. renderBlock
	// recomputes rendered from raw on every WindowSizeMsg so the block
	// reflows to the new width. blockBusy and blockToolComplete leave raw
	// empty; blockApprovalChoice carries its verdict in approved below so
	// renderBlock still runs when raw is empty.
	raw string
	// label and shimmer apply to blockBusy only.
	label   string
	shimmer shimmerKind
	// approved applies to blockApprovalChoice only.
	approved bool
	// pulumi carries per-block state for blockPulumiOp. It is mutated in place
	// as UIPulumiResource / UIPulumiDiag / UIPulumiEnd events arrive, then
	// re-rendered by renderBlock on every update.
	pulumi *pulumiBlockState
}

// pulumiBlockState accumulates the live state of a blockPulumiOp. Resources are
// deduped by URN (the index into resources is stored in resourceByURN so late
// events like ResourceOutputs can upgrade the status of an earlier
// ResourcePre). Diags are append-only because they carry their own severity
// and messages aren't keyed.
type pulumiBlockState struct {
	toolName      string
	stackName     string
	isPreview     bool
	resources     []pulumiResourceRow
	resourceByURN map[string]int
	diags         []pulumiDiagRow
	counts        display.ResourceChanges
	elapsed       string
	err           string
	done          bool
}

type pulumiResourceRow struct {
	op     display.StepOp
	urn    string
	typ    string
	status string
}

type pulumiDiagRow struct {
	severity string
	message  string
	urn      string
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
	// cancelling is true from the moment the user presses ESC (the TUI posts an
	// AgentUserEventCancel upstream) until the next final event arrives. While
	// it is true the busy label is overridden to "Cancelling..." so the user
	// can see their request is being acted on even if the agent is still
	// mid-tool.
	cancelling bool
	// ctrlCArmed is true after the first Ctrl+C (or Ctrl+D) press, until any
	// other key is seen or the timeout fires. While armed the footer hint
	// reads "Press Ctrl+C again to exit" and a second press quits. The first
	// press also acts like ESC when busy: posts user_cancel upstream so users
	// don't need to learn ESC to abort a turn. Any other keypress disarms.
	ctrlCArmed bool
	// ctrlCArmGen increments each time ctrlCArmed flips on. Disarm ticks
	// scheduled for an earlier arm carry the older gen, so a fresh arm racing
	// with a stale tick is not silently disarmed.
	ctrlCArmGen int
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

// renderIndented word-wraps content (ANSI-safe) to termWidth minus the
// 2-space transcript gutter, or returns un-wrapped if the width is too
// small to wrap into.
func renderIndented(style lipgloss.Style, termWidth int, content string) string {
	if termWidth <= 4 {
		return "  " + style.Render(content)
	}
	return "  " + style.Width(termWidth-2).Render(content)
}

// NewModel creates a new TUI Model.
func NewModel(cfg ModelConfig) Model {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.PromptStyle = promptStyle
	ti.Placeholder = "Send a message..."
	ti.Focus()
	ti.CharLimit = 4096

	vp := viewport.New(80, 24-inputBarHeight)
	// The default viewport KeyMap binds plain letters (u/d/f/b/j/k) and
	// space to scroll actions, which collide with typing in the chat
	// input. Restrict to PgUp/PgDn so we don't shadow system or text-input
	// shortcuts (arrows move the cursor, Ctrl+U/Ctrl+D have terminal meanings).
	vp.KeyMap = viewport.KeyMap{
		PageDown: key.NewBinding(key.WithKeys("pgdown")),
		PageUp:   key.NewBinding(key.WithKeys("pgup")),
	}

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
			label:   thinkingLabel,
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

		vpHeight := max(msg.Height-inputBarHeight, 1)
		m.viewport.Width = msg.Width
		m.viewport.Height = vpHeight
		m.textInput.Width = msg.Width - lipgloss.Width(m.textInput.Prompt) - 1

		// Glamour's wrap width is baked in at construction, so rebuild on resize.
		if r, err := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(msg.Width-4),
		); err == nil {
			m.mdRenderer = r
		}
		for i := range m.blocks {
			m.renderBlock(&m.blocks[i])
		}
		m.rebuildContent()

	case ctrlCDisarmMsg:
		// Stale tick: the user already pressed another key (gen still
		// matches but ctrlCArmed=false) or re-armed (gen advanced). Either
		// way, leave the current state alone.
		if msg.gen == m.ctrlCArmGen && m.ctrlCArmed {
			m.ctrlCArmed = false
			m.rebuildContent()
		}
		return m, nil

	case tea.KeyMsg:
		// Ctrl+D mirrors Ctrl+C: same arm/quit gate, same cancel-when-busy
		// semantics. Two bindings is friendlier than picking one and forcing
		// users to discover it.
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlD {
			if m.ctrlCArmed {
				return m, tea.Quit
			}
			m.ctrlCArmed = true
			m.ctrlCArmGen++
			disarmCmd := m.scheduleCtrlCDisarm()
			// First press doubles as a cancel when the agent is mid-turn, so
			// users who reach for Ctrl+C don't need to learn ESC to abort.
			// Same guards as the ESC handler below.
			if m.busy && !m.pendingApproval && !m.cancelling {
				m.sendOut(outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}})
				m.cancelling = true
				cancelCmd := m.showBusy("Cancelling...", shimmerVerb)
				m.rebuildContent()
				return m, tea.Batch(cancelCmd, disarmCmd)
			}
			m.rebuildContent()
			return m, disarmCmd
		}
		// Any other key disarms the second-press-to-exit prompt. Keeps the
		// "two presses in a row" semantics tight: a stray keystroke between
		// presses goes back to needing two presses again. The pending tick
		// will fire later but no-op because ctrlCArmed is already false.
		m.ctrlCArmed = false

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

		// ESC asks the agent to abort the current turn. Posts user_cancel
		// upstream and flips the local cancelling flag so the spinner label
		// switches to "Cancelling..." until the backend acknowledges via
		// cancelled / error / a new final assistant_message. Ignored when the
		// TUI isn't busy or is already waiting on an approval (where the
		// agent is paused for us anyway).
		if msg.Type == tea.KeyEsc {
			if m.busy && !m.pendingApproval && !m.cancelling {
				m.sendOut(outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}})
				m.cancelling = true
				cmd := m.showBusy("Cancelling...", shimmerVerb)
				m.rebuildContent()
				return m, cmd
			}
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
				m.sendOut(outboundEvent{
					event: apitype.AgentUserEventUserConfirmation{
						Type:       userEventUserConfirmation,
						ApprovalID: m.pendingApprovalID,
						Approved:   approved,
						Message:    denialMsg,
					},
					planMode: m.planMode,
				})
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
				m.appendRenderedBlock(block{
					kind:     blockApprovalChoice,
					approved: approved,
					raw:      denialMsg,
				})
				if approved {
					cmd := m.showBusy(thinkingLabel, shimmerVerb)
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
				sent := m.sendOut(outboundEvent{
					event: apitype.AgentUserEventUserMessage{
						Type:    userEventUserMessage,
						Content: text,
					},
					planMode: m.planMode,
				})
				if sent {
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
					return m, m.showBusy(thinkingLabel, shimmerVerb)
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
		// A truly-final message closes out any in-flight streaming block. The
		// final-marker append is guarded because IsFinal=true can arrive with
		// empty content (e.g. a hand-off that became final once the server
		// reconciled pending tool calls) and we don't want a phantom marker.
		if msg.IsFinal && !msg.HasPendingCLIWork {
			m.removeBlockKind(blockAssistantStreaming)
			if msg.Content != "" {
				m.appendRenderedBlock(block{kind: blockAssistantFinal, raw: msg.Content})
			}
		} else if msg.Content != "" {
			if idx := m.findBlockKind(blockAssistantStreaming); idx >= 0 {
				m.blocks[idx].raw = msg.Content
				m.renderBlock(&m.blocks[idx])
			} else {
				m.appendRenderedBlock(block{kind: blockAssistantStreaming, raw: msg.Content})
			}
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolStarted:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolProgress:
		cmds = append(cmds, m.applyBusyForEvent(msg))
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
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIError:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.appendRenderedBlock(block{kind: blockError, raw: msg.Message})
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIWarning:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.appendWarningBlock(msg.Message)
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UICancelled:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.appendRenderedBlock(block{kind: blockCancelled, raw: "Session cancelled."})
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UITaskIdle:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UISessionURL:
		// Informational metadata for the welcome box — has no opinion on busy state.
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
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIAwaitingApprovals:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIContextCompression:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiStart:
		// Reuse an existing open block for the same tool name if one is present
		// (shouldn't happen in practice — each tool call is serialized — but it
		// guards against a stale block if the agent retries). An open block has
		// done==false. On reuse we reset state so the new run starts clean.
		idx := m.findOpenPulumiBlock(msg.ToolName)
		if idx < 0 {
			b := block{kind: blockPulumiOp, pulumi: &pulumiBlockState{
				toolName:      msg.ToolName,
				stackName:     msg.StackName,
				isPreview:     msg.IsPreview,
				resourceByURN: map[string]int{},
			}}
			m.renderBlock(&b)
			m.appendBlock(b)
		} else {
			m.blocks[idx].pulumi = &pulumiBlockState{
				toolName:      msg.ToolName,
				stackName:     msg.StackName,
				isPreview:     msg.IsPreview,
				resourceByURN: map[string]int{},
			}
			m.renderBlock(&m.blocks[idx])
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiResource:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			m.blocks[idx].pulumi.addResource(msg.Op, msg.URN, msg.Type, msg.Status)
			m.renderBlock(&m.blocks[idx])
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiDiag:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			st := m.blocks[idx].pulumi
			st.diags = append(st.diags, pulumiDiagRow{
				severity: msg.Severity,
				message:  msg.Message,
				urn:      msg.URN,
			})
			m.renderBlock(&m.blocks[idx])
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiEnd:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			st := m.blocks[idx].pulumi
			st.counts = msg.Counts
			st.elapsed = msg.Elapsed
			st.err = msg.Err
			st.done = true
			m.renderBlock(&m.blocks[idx])
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.rebuildContent()
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIApprovalRequest:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.pendingApproval = true
		m.pendingApprovalID = msg.ApprovalID
		m.pendingApprovalType = msg.ApprovalType
		if m.pendingApprovalType == approvalTypePlanExit {
			// The plan body is authored as markdown and lives in
			// PlanDescription; msg.Message is just a generic intro.
			m.appendRenderedBlock(block{kind: blockApprovalPlan, raw: msg.PlanDescription})
			m.textInput.Prompt = "Approve plan? [y to approve / reason to deny]: "
		} else {
			m.appendRenderedBlock(block{kind: blockApprovalGeneral, raw: msg.Message})
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
		hintText = "agent is working · enter disabled · esc or ctrl+c to cancel"
	}
	hint := "  "
	if m.planMode {
		hint += planAccentStyle.Render("⏸ plan mode")
		hintText = " · " + hintText
	}
	if m.ctrlCArmed {
		// Override everything else: this is a transient prompt, the user just
		// pressed Ctrl+C and needs to see what a second press will do.
		hint = "  " + inputHintStyle.Render("Press Ctrl+C again to exit")
	} else {
		hint += inputHintStyle.Render(hintText)
	}

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

// applyBusyForEvent is the single point that decides whether the busy
// indicator should be visible after a UIEvent arrives, mirroring the web
// console's "is the last event in the stream final?" rule. Final events hide
// the spinner and re-enable input; non-final events keep it on, with a label
// chosen by labelForUIEvent (or "Cancelling..." overriding everything while
// the user's cancel request is in flight).
func (m *Model) applyBusyForEvent(ev UIEvent) tea.Cmd {
	if isFinalUIEvent(ev) {
		m.cancelling = false
		m.endBusy()
		return nil
	}
	if m.cancelling {
		return m.showBusy("Cancelling...", shimmerVerb)
	}
	label, shim, set := m.labelForUIEvent(ev)
	if !set {
		// Non-opinionated event (warning, foreign user message, session URL,
		// non-final tick): leave the busy state exactly as it is.
		return nil
	}
	return m.showBusy(label, shim)
}

// isFinalUIEvent reports whether ev closes the agent's turn. See
// applyBusyForEvent for the full rule and the CLI-work exception.
func isFinalUIEvent(ev UIEvent) bool {
	switch e := ev.(type) {
	case UIAssistantMessage:
		return e.IsFinal && !e.HasPendingCLIWork
	case UIApprovalRequest, UICancelled, UIError, UITaskIdle:
		return true
	default:
		return false
	}
}

// labelForUIEvent picks the busy-indicator label for a non-final UIEvent. The
// third return value is false when the event has no opinion on the label — in
// that case the caller leaves the current label alone.
func (m *Model) labelForUIEvent(ev UIEvent) (string, shimmerKind, bool) {
	switch e := ev.(type) {
	case UIToolStarted:
		return toolLabel(e.Name, e.Args) + " ...", shimmerWave, true
	case UIToolProgress:
		return toolLabel(e.Name, nil) + ": " + truncate(e.Message, 60), shimmerWave, true
	case UIToolCompleted:
		return thinkingLabel, shimmerVerb, true
	case UIAssistantMessage:
		// Only reached when non-final (streaming) or when IsFinal=true with
		// pending CLI work — i.e. the agent is still working.
		return thinkingLabel, shimmerVerb, true
	case UIAwaitingApprovals:
		return "Awaiting approvals...", shimmerVerb, true
	case UIContextCompression:
		return "Compressing context...", shimmerVerb, true
	}
	return "", 0, false
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

func (m *Model) appendWarningBlock(msg string) {
	m.appendRenderedBlock(block{kind: blockWarning, raw: msg})
}

func (m *Model) appendUserMessageBlock(content string) {
	m.appendRenderedBlock(block{kind: blockUserMessage, raw: content})
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

// scheduleCtrlCDisarm returns a tea.Cmd that, after ctrlCArmTimeout, posts a
// ctrlCDisarmMsg tagged with the current arm generation. The Update handler
// ignores stale ticks (gen mismatch or already-disarmed state) so a rapid
// arm → disarm → re-arm sequence remains correct.
func (m *Model) scheduleCtrlCDisarm() tea.Cmd {
	gen := m.ctrlCArmGen
	return tea.Tick(ctrlCArmTimeout, func(time.Time) tea.Msg {
		return ctrlCDisarmMsg{gen: gen}
	})
}

// sendOut is a non-blocking send on the outbound channel. Returns true on
// success. Safe when m.outCh is nil: select with default falls through,
// since sending on a nil channel blocks forever.
func (m *Model) sendOut(e outboundEvent) bool {
	select {
	case m.outCh <- e:
		return true
	default:
		return false
	}
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

// renderBlock recomputes b.rendered from b.raw using the current terminal
// width and markdown renderer. blockApprovalChoice is the one kind that
// still renders when raw is empty (its verdict is carried by b.approved).
func (m *Model) renderBlock(b *block) {
	if b.kind == blockApprovalChoice {
		m.renderApprovalChoice(b)
		return
	}
	if b.kind == blockPulumiOp {
		b.rendered = m.renderPulumiBlock(b.pulumi)
		return
	}
	if b.raw == "" {
		return
	}
	switch b.kind {
	case blockWarning:
		b.rendered = renderIndented(warningStyle, m.width, "⚠ "+b.raw)
	case blockError:
		b.rendered = renderIndented(errorStyle, m.width, "✗ Error: "+b.raw)
	case blockCancelled:
		b.rendered = renderIndented(cancelledStyle, m.width, b.raw)
	case blockUserMessage:
		b.rendered = m.renderUserBubble(b.raw)
	case blockAssistantStreaming:
		b.rendered = renderAssistantStreaming(m.wrapPlain(b.raw))
	case blockAssistantFinal:
		b.rendered = renderAssistantFinal(m.renderMarkdown(b.raw))
	case blockApprovalPlan:
		header := planAccentStyle.Render("⏺ Proposed plan")
		b.rendered = renderHeaderedBlock(header, m.renderMarkdown(b.raw))
	case blockApprovalGeneral:
		header := warningStyle.Render("⚠ Approval required")
		b.rendered = renderHeaderedBlock(header, m.wrapPlain(b.raw))
	case blockBusy, blockToolComplete:
		// No raw: blockBusy renders live from label, blockToolComplete is
		// pre-styled at event time.
	case blockApprovalChoice, blockPulumiOp:
		// Unreachable: both are handled by early returns above.
	}
}

func (m *Model) renderApprovalChoice(b *block) {
	if b.approved {
		b.rendered = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓ Approved")
		return
	}
	denied := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("✗ Denied")
	if b.raw == "" {
		b.rendered = "  " + denied
		return
	}
	b.rendered = renderIndented(lipgloss.NewStyle(), m.width, denied+" — "+b.raw)
}

// renderUserBubble renders a user-chat bubble. Short messages hug their
// content; only overflow triggers Width, which both wraps and pads so the
// background colour fills every wrapped line evenly.
func (m *Model) renderUserBubble(content string) string {
	prefix := promptStyle.Render("❯") + " " // visible width 2
	padded := " " + content + " "
	bubbleWidth := max(m.width-2, 8)
	style := userMsgBubble
	if m.width > 4 && lipgloss.Width(padded) > bubbleWidth {
		style = style.Width(bubbleWidth)
	}
	return prefix + style.Render(padded)
}

// wrapPlain word-wraps non-markdown text to the terminal width.
func (m *Model) wrapPlain(text string) string {
	if m.width <= 4 {
		return text
	}
	return lipgloss.NewStyle().Width(m.width - 4).Render(text)
}

func (m *Model) appendRenderedBlock(b block) {
	m.renderBlock(&b)
	m.appendBlock(b)
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
