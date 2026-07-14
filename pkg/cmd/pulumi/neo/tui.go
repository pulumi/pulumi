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
	"encoding/json"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"
	"github.com/muesli/reflow/wordwrap"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// ctrlCArmTimeout is how long the "press Ctrl+C again to exit" gate stays
// armed after the first press. Matches the cadence other agent CLIs use so
// the second press still has to be deliberate but the gate doesn't silently
// linger across long idle periods.
const ctrlCArmTimeout = 1500 * time.Millisecond

// minUsableTUIWidth is the floor below which a startup WindowSizeMsg is more
// likely to be a transient terminal initialization artifact than a useful
// interactive viewport. Rendering the input at those widths leaves only "Se"
// from the placeholder in large terminals until the next resize.
const minUsableTUIWidth = 40

// ctrlCDisarmMsg is the deferred disarm signal scheduled when the user first
// presses Ctrl+C. It carries the generation it was scheduled under; the
// handler ignores it if the user has since re-armed (gen advanced) or
// already disarmed by typing another key.
type ctrlCDisarmMsg struct {
	gen int
}

// modeToggleDebounce is the trailing-edge window for Ctrl+A / Ctrl+R: each
// press updates local state immediately but only schedules a PATCH against the
// live task after the user stops pressing for this long. Rapid mashing
// collapses to one PATCH carrying the final value, so the cloud and the
// dispatcher both see the user's intent instead of a noisy burst of updates.
const modeToggleDebounce = 500 * time.Millisecond

// approvalDebounceTickMsg / permissionDebounceTickMsg fire `modeToggleDebounce`
// after each Ctrl+A / Ctrl+R press. They carry the generation they were
// scheduled under so a newer press (which advances the generation) cancels the
// stale tick by gen-mismatch.
type approvalDebounceTickMsg struct {
	gen int
}

type permissionDebounceTickMsg struct {
	gen int
}

// firstFlushReadyMsg defers the welcome banner / pre-seeded blocks to
// tea.Println until after bubbletea v2's first renderer flush. Calling
// Println inside the WindowSizeMsg handler runs while cellbuf is still
// sized to the terminal, so insertAbove would scroll a screenful of blank
// lines above the prompt. rendered is the whole flush pre-joined into one
// string so it can be emitted as a single, atomic Println — Update returns
// its cmds via tea.Batch, which runs them concurrently, so per-block
// Printlns would race each other (and any event-driven print) for
// scrollback order.
type firstFlushReadyMsg struct {
	rendered string
}

// blockKind identifies the type of rendered block in the output log.
type blockKind int

const (
	blockBusy blockKind = iota
	blockToolComplete
	blockAssistantFinal
	blockError
	blockWarning
	blockCancelled
	blockUserMessage
	blockApprovalPlan
	blockApprovalGeneral
	blockApprovalChoice
	blockApprovalAuto
	blockQuestion
	blockAnswerSubmitted
	blockPulumiOp
	blockTodoList
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
	// approved applies to blockApprovalChoice and blockApprovalAuto. For
	// blockApprovalAuto it carries the "ok" field from the cloud's
	// user_confirmation (true=auto-approved, false=auto-denied).
	approved bool
	// autoIsQuestion applies to blockApprovalAuto only — true if the underlying
	// request was an ask-user call (so the renderer says "Auto-answered" rather
	// than "Auto-approved").
	autoIsQuestion bool
	// pulumi carries per-block state for blockPulumiOp. It is mutated in place
	// as UIPulumiResource / UIPulumiDiag / UIPulumiEnd events arrive, then
	// re-rendered by renderBlock on every update.
	pulumi *pulumiBlockState
	// todos is populated for blockTodoList and folded into blockApprovalPlan
	// as a Tasks: subsection.
	todos []UITodoItem
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
	// Version is the Pulumi CLI version stamped at build time (e.g. "v3.235.0").
	// Empty in dev builds — the welcome banner omits it when blank.
	Version string
	EventCh <-chan UIEvent
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
	// InitialApprovalMode and InitialPermissionMode seed the TUI's view of the
	// task's approval / permission policy. These come from the --approval-mode /
	// --permission-mode flags (or their defaults) and are sent on the first
	// user_message; subsequent toggles via Ctrl+A / Ctrl+R PATCH the live task.
	InitialApprovalMode   client.NeoApprovalMode
	InitialPermissionMode client.NeoPermissionMode
	// MessageSent seeds the post-first-message gate. Set it to true in tests
	// that want to exercise the Shift+Tab post-send warning path without
	// having to simulate a full Enter-driven send first.
	MessageSent bool
	// TaskCreated seeds the post-task-creation gate. Set it to true in tests
	// that want to exercise the post-task Ctrl+A / Ctrl+R path without driving
	// a UISessionURL through the event channel.
	TaskCreated bool
	// History seeds the transcript for a resumed session. These events are
	// applied before the first render so long histories do not flow through the
	// bounded live EventCh.
	History []UIEvent
	// InitialWidth seeds the first render before Bubble Tea sends WindowSizeMsg.
	// It is also used as a guard against bogus tiny startup resize events.
	InitialWidth int
	// HasDarkBackground selects light- or dark-friendly style variants for the
	// whole TUI, including the textarea. runNeo detects it synchronously before
	// the program starts and defaults it to dark for terminals it can't probe.
	HasDarkBackground bool
}

// Model is the top-level bubbletea model for the Neo TUI.
//
// The TUI runs inline (no alt-screen). Completed blocks (user messages, tool
// completions, final assistant replies, errors, warnings, finished pulumi ops)
// are committed to terminal scrollback via tea.Println as soon as they reach a
// terminal state. Only "live" blocks render in the bubbletea frame above the
// input bar: the busy spinner, an in-flight streaming assistant message, and
// an in-flight pulumi op that hasn't received UIPulumiEnd yet.
type Model struct {
	welcome   welcomeModel
	textInput textarea.Model
	// approvalPromptText, when non-empty, is rendered as a header line above
	// the textarea. Used in pending-approval and ask-user-question states
	// where the chrome that used to live in textinput.Prompt has nowhere to
	// go in a multi-line textarea (textarea's Prompt is per-line, not a
	// global prefix). Cleared by clearPendingPrompt.
	approvalPromptText string
	blocks             []block
	eventCh            <-chan UIEvent
	outCh              chan<- outboundEvent
	// sizeReceived flips on the first WindowSizeMsg. The first resize is also
	// the moment we know the terminal width, so it's when we emit the welcome
	// banner and any pre-seeded committed blocks (e.g. an InitialPrompt user
	// message) to scrollback.
	sizeReceived bool
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
	// hasDarkBackground selects light- or dark-friendly style variants;
	// seeded from ModelConfig.HasDarkBackground.
	hasDarkBackground bool
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
	// approvalMode is the current approval policy, cycled via Ctrl+A. Pre-first-
	// message it's purely local — the value is snapshotted into CreateNeoTask on
	// the first user_message. Post-send toggles dispatch an outboundEvent.update
	// that the runNeo dispatcher routes through UpdateNeoTask, so cloud
	// ApprovalHandler picks up the change without restarting the session.
	approvalMode client.NeoApprovalMode
	// permissionMode is the current capability scope, toggled via Ctrl+R. Same
	// pre/post-send semantics as approvalMode.
	permissionMode client.NeoPermissionMode
	// approvalDebounceGen / permissionDebounceGen increment on every post-send
	// Ctrl+A / Ctrl+R press. Each press schedules a tea.Tick carrying the
	// current generation; the tick handler dispatches the PATCH only if the
	// gen still matches, so rapid presses collapse to a single update with the
	// final mode value.
	approvalDebounceGen   int
	permissionDebounceGen int
	// messageSent flips to true when the TUI successfully dispatches its
	// first user_message on outCh. From that point on, Shift+Tab emits the
	// "plan mode is task-level" warning instead of toggling.
	messageSent bool
	// taskCreated flips to true when UISessionURL arrives — the dispatcher
	// emits that immediately after CreateNeoTask returns OK, so it's the
	// natural signal that the task is now addressable for PATCH. The window
	// between messageSent and taskCreated is the race the Ctrl+A / Ctrl+R
	// handlers gate on: in that window the dispatcher would silently drop a
	// UpdateNeoTaskOptions event (no taskID yet), leaving the local mode out
	// of sync with the cloud. Swallow the keypress instead so the status bar
	// can't lie about what the cloud is enforcing.
	taskCreated bool
	// pendingApprovalType is the raw wire approval_type for the currently
	// pending approval (empty when none). The Enter handler checks for
	// approvalTypePlanExit so it can auto-clear planMode on approval.
	pendingApprovalType string
	// pendingIsQuestion is true when the pending request is an ask-user
	// question (see isAskUserToolName) rather than an approval. Tells the
	// Enter handler to take the typed text as a free-form answer instead of
	// running the y/yes-or-deny parsing.
	pendingIsQuestion bool
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
	// hasEmittedScrollback flips on the first call to printlnBlock. Subsequent
	// emissions prepend a blank line so each committed block has visual
	// breathing room from whatever came before.
	hasEmittedScrollback bool
	// pendingTodos buffers the latest UITodoList while planMode is true so
	// the list lands inside the same block as the Proposed plan, not above it.
	pendingTodos  []UITodoItem
	toolHistory   []toolCallRecord
	overlayActive bool
	overlay       overlayModel
	// history holds the prompts the user has submitted this session, oldest
	// first, recalled with the up/down arrows. There is intentionally no
	// footer hint for it — the affordance mirrors shell history and is
	// meant to be discovered by muscle memory.
	history []string
	// historyIdx is the cursor into history during up/down navigation. It
	// equals len(history) when not navigating, i.e. the user is editing the
	// live draft; stepping back with Up walks it down toward the oldest entry.
	historyIdx int
	// historyDraft stashes the in-progress draft when history navigation
	// begins, so pressing Down past the newest entry restores what the user
	// had typed rather than leaving a recalled prompt behind.
	historyDraft string
}

var (
	inputHintStyle = lipgloss.NewStyle().Faint(true)
	promptStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	warningStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cancelledStyle = lipgloss.NewStyle().Faint(true)
	toolOKMarker   = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("⏺")
	toolErrMarker  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⏺")
	// planAccentStyle is a distinct cyan+bold used for both the footer banner
	// and the "Proposed plan" block header so they read as the same visual cue.
	planAccentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	// Styles for in-progress and completed todo items. Pending items render
	// in the default style — no entry here.
	todoActiveStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	todoCompletedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Faint(true)
	todoListHeader     = lipgloss.NewStyle().Bold(true).Render("⏺ TODO")
)

func placeholderStyle(hasDarkBackground bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(
		lipgloss.LightDark(hasDarkBackground)(lipgloss.Color("240"), lipgloss.Color("245")))
}

// renderLeftBracket decorates content with a left-only bracket border: a
// "╭─" tick above the first content line, a "│ " prefix on every content
// line, and a "╰─" tick below the last line. The right edge and full
// horizontal bars are intentionally omitted so no rendered line ever
// stretches across the terminal width — that would wrap on resize-shrink
// and desync the inline-renderer line accounting. All bracket glyphs are
// colored via borderStyle.
//
// Empty content collapses to two corner ticks ("╭─" / "╰─") with no body.
func renderLeftBracket(borderStyle lipgloss.Style, content string) string {
	bar := borderStyle.Render("│")
	topTick := borderStyle.Render("╭─")
	bottomTick := borderStyle.Render("╰─")

	body := strings.TrimRight(content, "\n")
	out := make([]string, 0, 2)
	out = append(out, topTick)
	if body != "" {
		for line := range strings.SplitSeq(body, "\n") {
			out = append(out, bar+" "+line)
		}
	}
	out = append(out, bottomTick)
	return strings.Join(out, "\n")
}

// nextApprovalMode returns the next value in the manual → balanced → auto →
// manual cycle. Empty input is treated as manual so cycling from the zero value
// lands on a real first state.
func nextApprovalMode(m client.NeoApprovalMode) client.NeoApprovalMode {
	switch m {
	case client.NeoApprovalModeManual:
		return client.NeoApprovalModeBalanced
	case client.NeoApprovalModeBalanced:
		return client.NeoApprovalModeAuto
	case client.NeoApprovalModeAuto:
		return client.NeoApprovalModeManual
	}
	return client.NeoApprovalModeManual
}

// nextPermissionMode flips between default and read-only. Anything else
// (including empty) collapses back to default.
func nextPermissionMode(m client.NeoPermissionMode) client.NeoPermissionMode {
	if m == client.NeoPermissionModeReadOnly {
		return client.NeoPermissionModeDefault
	}
	return client.NeoPermissionModeReadOnly
}

// recallInput replaces the textarea contents with a recalled prompt and parks
// the cursor at the end, ready to edit or resend.
func (m *Model) recallInput(s string) {
	m.textInput.SetValue(s)
	m.textInput.MoveToEnd()
}

// historyPrev steps to an older prompt. It returns true when it handled the
// key (so the caller swallows it). The first step out of the live draft
// stashes that draft in historyDraft so a later Down can restore it.
func (m *Model) historyPrev() bool {
	if len(m.history) == 0 {
		return false
	}
	if m.historyIdx == 0 {
		return true // already at the oldest entry; pin here
	}
	if m.historyIdx == len(m.history) {
		m.historyDraft = m.textInput.Value()
	}
	m.historyIdx--
	m.recallInput(m.history[m.historyIdx])
	return true
}

// historyNext steps toward newer prompts; past the newest it restores the
// saved draft. It returns false when not currently navigating so Down falls
// through to the textarea's own cursor movement.
func (m *Model) historyNext() bool {
	if m.historyIdx >= len(m.history) {
		return false
	}
	m.historyIdx++
	if m.historyIdx == len(m.history) {
		m.recallInput(m.historyDraft)
	} else {
		m.recallInput(m.history[m.historyIdx])
	}
	return true
}

// renderIndented word-wraps content (ANSI-safe) to termWidth minus the
// 2-space transcript gutter, or returns un-wrapped if the width is too
// small to wrap into. URLs in the post-wrap output are wrapped in OSC 8
// escapes so terminals that support hyperlinks render them as clickable.
// We linkify after wrapping so a URL split across lines simply stays
// non-clickable rather than producing a broken escape sequence.
func renderIndented(style lipgloss.Style, termWidth int, content string) string {
	if termWidth <= 4 {
		return "  " + linkifyURLs(style.Render(content))
	}
	return "  " + linkifyURLs(style.Width(termWidth-2).Render(content))
}

// NewModel creates a new TUI Model.
func NewModel(cfg ModelConfig) Model {
	initialWidth := cfg.InitialWidth
	if initialWidth <= 0 {
		initialWidth = 80
	}
	initialHeight := 24

	ti := textarea.New()
	ti.Placeholder = "Send a message..."
	ti.CharLimit = 4096
	ti.ShowLineNumbers = false
	// DynamicHeight starts the input at one visible line and grows it as the
	// user adds newlines, capped at MaxHeight so a long paste scrolls inside
	// the textarea rather than pushing scrollback off-screen.
	ti.DynamicHeight = true
	ti.MinHeight = 1
	// MaxHeight=0 disables both the visual cap and atContentLimit, so the
	// textarea grows with content. CharLimit is the real upper bound.
	ti.MaxHeight = 0
	// textarea defaults to height 6 and only recalculates inside SetWidth.
	// Force the first frame to 1 line so there's no gap above the welcome
	// banner before the WindowSizeMsg handler fires.
	ti.SetHeight(1)
	// First line carries the prompt chevron; subsequent lines indent to keep
	// continuation lines visually aligned under the chevron.
	ti.SetPromptFunc(2, func(info textarea.PromptInfo) string {
		if info.LineNumber == 0 {
			return "❯ "
		}
		return "  "
	})
	// textarea.New defaults to dark styles; reselect for the detected background
	// so the input box (cursor line, text) reads on a light terminal too.
	styles := textarea.DefaultStyles(cfg.HasDarkBackground)
	styles.Focused.Prompt = promptStyle
	styles.Blurred.Prompt = promptStyle
	placeholder := placeholderStyle(cfg.HasDarkBackground)
	styles.Focused.Placeholder = placeholder
	styles.Blurred.Placeholder = placeholder
	ti.SetStyles(styles)
	// Keep Enter as submit. Shift+Enter / Alt+Enter need kitty keyboard
	// protocol; Ctrl+J (== LF) is the portable fallback. Trailing backslash
	// + Enter inserts a newline via the bare-Enter branch of Update.
	ti.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter", "alt+enter", "ctrl+j"),
		key.WithHelp("shift+enter / alt+enter / ctrl+j", "newline"),
	)
	ti.Focus()

	sp := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6"))),
	)

	m := Model{
		welcome: welcomeModel{
			org:       cfg.Org,
			workDir:   cfg.WorkDir,
			username:  cfg.Username,
			version:   cfg.Version,
			termWidth: initialWidth,
			greeting:  pickGreeting(cfg.Username),
		},
		textInput:         ti,
		eventCh:           cfg.EventCh,
		outCh:             cfg.OutCh,
		busy:              cfg.Busy,
		spinner:           sp,
		width:             initialWidth,
		height:            initialHeight,
		hasDarkBackground: cfg.HasDarkBackground,
		messageSent:       cfg.MessageSent,
		taskCreated:       cfg.TaskCreated,
		approvalMode:      cfg.InitialApprovalMode,
		permissionMode:    cfg.InitialPermissionMode,
		overlay:           newOverlayModel(initialWidth, initialHeight),
	}
	m.textInput.SetWidth(max(m.liveWidth(), 3))
	if cfg.InitialPrompt != "" {
		// Render the initial-prompt block now so tests can find it via
		// findBlockKind. The actual scrollback emission happens on the first
		// WindowSizeMsg, when we have the real terminal width. Seed the block
		// directly rather than via commitBlock: its printlnBlock side effect
		// would flip hasEmittedScrollback before anything is actually printed,
		// giving the welcome banner a stray leading blank line.
		m.stageBlock(block{kind: blockUserMessage, raw: cfg.InitialPrompt})
		m.pendingUserEchoes = append(m.pendingUserEchoes, cfg.InitialPrompt)
	}
	for _, event := range cfg.History {
		m.applyHistoryEvent(event)
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
	cmds := []tea.Cmd{waitForEvent(m.eventCh), textarea.Blink}
	if m.busy {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		firstSize := !m.sizeReceived
		m.sizeReceived = true
		width := msg.Width
		if firstSize && width < minUsableTUIWidth && m.width >= minUsableTUIWidth {
			width = m.width
		}
		height := msg.Height
		if height <= 0 {
			height = m.height
		}
		m.width = width
		m.height = height
		safeWidth := m.liveWidth()
		m.welcome.termWidth = safeWidth
		// SetWidth accounts for the prompt width registered via SetPromptFunc.
		m.textInput.SetWidth(max(safeWidth, 3))
		m.overlay.SetSize(m.width, m.height)
		if m.overlayActive {
			m.overlay.Refresh(m.toolHistory)
		}

		m.rebuildMarkdownRenderer()
		// Re-render every block at the new width. Live blocks (busy / streaming /
		// open pulumi op) get reflected in View() on the next draw; committed
		// blocks already in scrollback don't reflow — only the initial-prompt
		// block, queued by NewModel before the width was known, is committed
		// here for the first time below.
		for i := range m.blocks {
			m.renderBlock(&m.blocks[i])
		}
		if firstSize {
			rendered := m.committedScrollback()
			// 50ms covers ~3 ticks at bubbletea's 60Hz default — see
			// firstFlushReadyMsg for why we defer at all.
			cmds = append(cmds, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
				return firstFlushReadyMsg{rendered: rendered}
			}))
		}

	case firstFlushReadyMsg:
		cmds = append(cmds, m.printlnBlock(msg.rendered))

	case tea.ResumeMsg:
		// Resuming from a Ctrl+Z suspend (via `fg`): bubbletea repaints only the
		// live frame, but the committed transcript was emitted to scrollback via
		// tea.Println and isn't redrawn. Clear the viewport and re-emit the
		// committed transcript so the session reads continuously after `fg`.
		// Reset hasEmittedScrollback so the re-emit skips its leading blank
		// line, matching the initial flush. Sequence keeps the clear ahead of
		// the print.
		m.hasEmittedScrollback = false
		return m, tea.Sequence(tea.ClearScreen, m.printlnBlock(m.committedScrollback()))

	case ctrlCDisarmMsg:
		// Stale tick: the user already pressed another key (gen still
		// matches but ctrlCArmed=false) or re-armed (gen advanced). Either
		// way, leave the current state alone.
		if msg.gen == m.ctrlCArmGen && m.ctrlCArmed {
			m.ctrlCArmed = false
		}
		return m, nil

	case approvalDebounceTickMsg:
		// Trailing-edge debounce: dispatch the PATCH only if the user hasn't
		// pressed Ctrl+A again since this tick was scheduled. A newer press
		// advances approvalDebounceGen, so a stale tick is silently dropped
		// here. taskCreated is guaranteed by the Ctrl+A handler that
		// scheduled this tick.
		if msg.gen == m.approvalDebounceGen {
			next := m.approvalMode
			m.sendOut(outboundEvent{
				update: &client.UpdateNeoTaskOptions{ApprovalMode: &next},
			})
		}
		return m, nil

	case permissionDebounceTickMsg:
		if msg.gen == m.permissionDebounceGen {
			next := m.permissionMode
			m.sendOut(outboundEvent{
				update: &client.UpdateNeoTaskOptions{PermissionMode: &next},
			})
		}
		return m, nil

	case tea.KeyPressMsg:
		keyStr := msg.String()
		// While the overlay is open, ctrl+o / esc / ctrl+c / ctrl+d all close
		// it; scroll keys go to the viewport; everything else is swallowed so
		// it can't leak into the hidden input bar. Closing on ctrl+c/d means a
		// reflexive "abort" tap dismisses the overlay rather than killing the
		// session — the user can press ctrl+c again from the inline view to
		// quit. Alt-screen toggling happens in View() via tea.View.AltScreen.
		if m.overlayActive {
			switch keyStr {
			case "ctrl+o", "esc", "ctrl+c", "ctrl+d":
				m.overlayActive = false
				return m, nil
			case "up", "down", "pgup", "pgdown", "home", "end":
				return m, m.overlay.Update(msg)
			default:
				return m, nil
			}
		}
		// Ctrl+D mirrors Ctrl+C: same arm/quit gate, same cancel-when-busy
		// semantics. Two bindings is friendlier than picking one and forcing
		// users to discover it.
		if keyStr == "ctrl+c" || keyStr == "ctrl+d" {
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
				return m, tea.Batch(cancelCmd, disarmCmd)
			}
			return m, disarmCmd
		}
		// Any other key disarms the second-press-to-exit prompt. Keeps the
		// "two presses in a row" semantics tight: a stray keystroke between
		// presses goes back to needing two presses again. The pending tick
		// will fire later but no-op because ctrlCArmed is already false.
		m.ctrlCArmed = false

		// Ctrl+Z suspends via standard Unix job control (SIGTSTP); resume with
		// `fg`. Bubbletea restores the terminal around the stop, so raw mode is
		// fine. Not gated on busy — suspending mid-turn is the point, since
		// cancellation isn't instant.
		if keyStr == "ctrl+z" {
			return m, tea.Suspend
		}

		// Ctrl+O opens the overlay. Sits above the busy/approval gates so
		// users can peek mid-turn. Closing is handled by the overlayActive
		// branch above. Alt-screen toggling happens in View() via the
		// tea.View.AltScreen field.
		if keyStr == "ctrl+o" {
			m.overlayActive = true
			m.overlay.SetSize(m.width, m.height)
			m.overlay.Refresh(m.toolHistory)
			return m, nil
		}

		// Shift+Tab toggles plan mode. The toggle must run before the approval
		// and busy guards so users can flip the indicator at any point in the
		// pre-task window, even while the startup spinner is up. It also has to
		// intercept the key before textarea.Update sees it, since textarea
		// otherwise treats Shift+Tab as a keypress with no visible effect.
		if keyStr == "shift+tab" {
			if m.messageSent {
				// Plan mode is task-level on the wire and gets snapshotted at
				// the moment the first message is sent. A post-send toggle
				// would be misleading — it could not affect the task.
				return m, m.appendWarningBlock(
					"Plan mode is task-level — start a new `pulumi neo` session to change it.")
			}
			m.planMode = !m.planMode
			return m, nil
		}

		// Ctrl+A cycles the approval mode (manual → balanced → auto → manual).
		// Pre-message the value just updates the local snapshot; post-message
		// each press updates local state immediately but the PATCH against the
		// live task is debounced via approvalDebounceTickMsg so rapid mashing
		// collapses to one UpdateNeoTask carrying the final value (the cloud
		// would otherwise see a noisy burst).
		//
		// The window between messageSent and taskCreated is a separate race:
		// the dispatcher has no taskID to PATCH against yet and would silently
		// drop the update. Swallow the keypress in that window so the status
		// bar can't get out of sync with the cloud — the user can press again
		// once the task URL arrives.
		if keyStr == "ctrl+a" {
			if m.messageSent && !m.taskCreated {
				return m, nil
			}
			m.approvalMode = nextApprovalMode(m.approvalMode)
			if m.messageSent {
				return m, m.scheduleApprovalDebounce()
			}
			return m, nil
		}

		// Ctrl+R toggles the permission mode (default ↔ read-only). Same
		// pre/post-send semantics — and the same create-task race window and
		// debounce mechanics — as the approval-mode cycle above.
		if keyStr == "ctrl+r" {
			if m.messageSent && !m.taskCreated {
				return m, nil
			}
			m.permissionMode = nextPermissionMode(m.permissionMode)
			if m.messageSent {
				return m, m.schedulePermissionDebounce()
			}
			return m, nil
		}

		// ESC clears a non-empty textarea; with an empty box it asks the
		// agent to abort. The cancelling flag overrides the spinner label
		// until the backend acknowledges via cancelled / error / a new
		// final assistant_message. Skipped during a pending approval — the
		// agent is already paused for us there.
		if keyStr == "esc" {
			if m.textInput.Value() != "" {
				m.textInput.Reset()
				return m, nil
			}
			if m.busy && !m.pendingApproval && !m.cancelling {
				m.sendOut(outboundEvent{event: apitype.AgentUserEventCancel{Type: userEventUserCancel}})
				m.cancelling = true
				return m, m.showBusy("Cancelling...", shimmerVerb)
			}
			return m, nil
		}

		// Handled before the busy check because the agent is intentionally
		// paused here waiting for the user.
		if m.pendingApproval {
			if keyStr == "enter" {
				text := strings.TrimSpace(m.textInput.Value())
				if m.pendingIsQuestion {
					if text == "" {
						return m, nil
					}
					m.sendOut(outboundEvent{
						event: apitype.AgentUserEventUserConfirmation{
							Type:       userEventUserConfirmation,
							ApprovalID: m.pendingApprovalID,
							Approved:   false,
							Message:    text,
						},
						planMode: m.planMode,
					})
					m.clearPendingPrompt()
					answerCmd := m.commitBlock(block{kind: blockAnswerSubmitted, raw: text})
					return m, tea.Batch(answerCmd, m.showBusy(thinkingLabel, shimmerVerb))
				}
				approved := isAffirmative(text)
				denialMsg := ""
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
				// Approving a plan exits plan mode server-side (the PlanModeTracker
				// stops gating writes), so mirror that locally. Denial leaves the
				// mode on — the agent will re-plan and gate-out again on the next
				// exit_plan_mode call.
				if wasPlanApproval && approved {
					m.planMode = false
				}
				m.clearPendingPrompt()
				choiceCmd := m.commitBlock(block{
					kind:     blockApprovalChoice,
					approved: approved,
					raw:      denialMsg,
				})
				if approved {
					return m, tea.Batch(choiceCmd, m.showBusy(thinkingLabel, shimmerVerb))
				}
				return m, choiceCmd
			}
			var tiCmd tea.Cmd
			m.textInput, tiCmd = m.textInput.Update(msg)
			return m, tiCmd
		}

		if keyStr == "enter" {
			if m.busy {
				// Agent is mid-turn — leave the typed text in the input so
				// the user can send it after the next UITaskIdle.
				return m, nil
			}
			raw := m.textInput.Value()
			// Backslash-Enter: a trailing `\` rewrites this Enter from submit
			// to newline, so users on terminals that can't distinguish
			// Shift+Enter still have a way to add a line.
			if stripped, ok := strings.CutSuffix(raw, "\\"); ok {
				m.textInput.SetValue(stripped)
				m.textInput.InsertRune('\n')
				return m, nil
			}
			text := strings.TrimSpace(raw)
			// Typing `quit` or `exit` and pressing Enter cleanly closes the
			// session, complementing Ctrl+C / Ctrl+D for users who reach for
			// shell-style commands first. Strict whole-input match so messages
			// that merely contain the word ("quit the deploy") still send.
			if strings.EqualFold(text, "quit") || strings.EqualFold(text, "exit") {
				return m, tea.Quit
			}
			if text != "" {
				m.textInput.Reset()
				sent := m.sendOut(outboundEvent{
					event: apitype.AgentUserEventUserMessage{
						Type:    userEventUserMessage,
						Content: text,
					},
					planMode:       m.planMode,
					approvalMode:   m.approvalMode,
					permissionMode: m.permissionMode,
				})
				if sent {
					// Render optimistically so the user sees their message in
					// the transcript before the server echoes it back. The
					// echo is reconciled against pendingUserEchoes in the
					// UIUserMessage handler to avoid duplicates.
					userCmd := m.appendUserMessageBlock(text)
					m.pendingUserEchoes = append(m.pendingUserEchoes, text)
					// Record the prompt for up/down history recall. Skip a
					// consecutive duplicate so mashing the same message
					// doesn't bloat the history.
					if n := len(m.history); n == 0 || m.history[n-1] != text {
						m.history = append(m.history, text)
					}
					m.historyIdx = len(m.history)
					m.historyDraft = ""
					// Freeze the plan-mode affordance: planMode has now been
					// committed to the dispatcher and any later Shift+Tab
					// would be a no-op on the server.
					m.messageSent = true
					return m, tea.Batch(userCmd, m.showBusy(thinkingLabel, shimmerVerb))
				}
			}
			return m, nil
		}

		// Up/Down recall prompt history when the cursor sits on the edge line
		// of the input — mirroring shell history. Mid-buffer they fall through
		// to the textarea's line movement. No footer hint is shown, by design.
		if keyStr == "up" && m.textInput.Line() == 0 {
			if m.historyPrev() {
				return m, nil
			}
		}
		if keyStr == "down" && m.textInput.Line() == m.textInput.LineCount()-1 {
			if m.historyNext() {
				return m, nil
			}
		}

		// Pass to text input for typing. The terminal's own scrollback is
		// what the user uses to look back, so pgup/pgdn aren't forwarded.
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		cmds = append(cmds, tiCmd)

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
		return m, cmd

	case UIAssistantMessage:
		if msg.Content != "" {
			cmds = append(cmds, m.commitBlock(block{kind: blockAssistantFinal, raw: msg.Content}))
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolStarted:
		m.toolHistory = appendToolStart(m.toolHistory, msg.Name, msg.Args)
		if m.overlayActive {
			m.overlay.Refresh(m.toolHistory)
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolProgress:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIToolCompleted:
		completeToolCall(m.toolHistory, msg.Name, msg.Result, msg.IsError)
		if m.overlayActive {
			m.overlay.Refresh(m.toolHistory)
		}
		cmds = append(cmds, m.commitBlock(toolCompletedBlock(msg.Name, msg.Args, msg.IsError)))
		// Keep the busy block alive across the inter-tool gap so the spinner
		// stays visible while the agent decides its next move.
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIError:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, m.commitBlock(block{kind: blockError, raw: msg.Message}))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIWarning:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, m.appendWarningBlock(msg.Message))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIReconnecting:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIReconnected:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UICancelled:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, m.commitBlock(block{kind: blockCancelled, raw: "Session cancelled."}))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UITaskIdle:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UISessionURL:
		// The welcome banner is already in scrollback by the time the task URL
		// arrives, so emit the URL as its own line rather than re-rendering.
		// OSC 8 wraps the URL so supporting terminals render it as clickable.
		m.welcome.consoleURL = msg.URL
		// CreateNeoTask sends UISessionURL immediately after taskID is set,
		// so this is the natural moment to lift the post-Enter toggle freeze.
		m.taskCreated = true
		cmds = append(cmds, m.printlnBlock("  "+inputHintStyle.Render("⟡ "+osc8Hyperlink(msg.URL, msg.URL))))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIUserMessage:
		// If the queue's front matches, this is the server echoing a message
		// we already rendered optimistically on submit — pop and skip.
		// Otherwise it came from another client (e.g. the web UI) and we
		// render it normally.
		if len(m.pendingUserEchoes) > 0 && m.pendingUserEchoes[0] == msg.Content {
			m.pendingUserEchoes = m.pendingUserEchoes[1:]
		} else {
			cmds = append(cmds, m.appendUserMessageBlock(msg.Content))
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIAwaitingApprovals:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIContextCompression:
		cmds = append(cmds, m.applyBusyForEvent(msg))
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
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiResource:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			m.blocks[idx].pulumi.addResource(msg.Op, msg.URN, msg.Type, msg.Status)
			m.renderBlock(&m.blocks[idx])
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
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
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIPulumiEnd:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			st := m.blocks[idx].pulumi
			st.counts = msg.Counts
			st.elapsed = msg.Elapsed
			st.err = msg.Err
			st.done = true
			m.renderBlock(&m.blocks[idx])
			// done==true flips it from live to committed; emit to scrollback.
			if rendered := m.blocks[idx].rendered; rendered != "" {
				cmds = append(cmds, m.printlnBlock(rendered))
			}
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UITodoList:
		// In plan mode, hold the list until plan_exit so the tasks land
		// inside the same block as the plan. Outside plan mode commit
		// immediately so status flips show up in scrollback.
		if len(msg.Items) > 0 {
			if m.planMode {
				m.pendingTodos = msg.Items
			} else {
				cmds = append(cmds, m.commitBlock(block{kind: blockTodoList, todos: msg.Items}))
			}
		}
		cmds = append(cmds, m.applyBusyForEvent(msg))
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIApprovalRequest:
		cmds = append(cmds, m.applyBusyForEvent(msg))
		m.pendingApproval = true
		m.pendingApprovalID = msg.ApprovalID
		m.pendingApprovalType = msg.ApprovalType
		m.pendingIsQuestion = false
		m.textInput.Placeholder = ""
		m.textInput.Reset()
		switch {
		case m.pendingApprovalType == approvalTypePlanExit:
			// The plan body is authored as markdown and lives in
			// PlanDescription; msg.Message is just a generic intro.
			planBlock := block{kind: blockApprovalPlan, raw: msg.PlanDescription, todos: m.pendingTodos}
			m.pendingTodos = nil
			cmds = append(cmds, m.commitBlock(planBlock))
			m.approvalPromptText = warningStyle.Render("Approve plan? [y to approve / reason to deny]:")
		case isAskUserToolName(msg.ToolName):
			m.pendingIsQuestion = true
			cmds = append(cmds, m.commitBlock(block{kind: blockQuestion, raw: msg.Message}))
			m.approvalPromptText = promptStyle.Render("Your answer:")
		default:
			cmds = append(cmds, m.commitBlock(block{kind: blockApprovalGeneral, raw: msg.Message}))
			m.approvalPromptText = warningStyle.Render("Approve? [y to approve / reason to deny]:")
		}
		cmds = append(cmds, waitForEvent(m.eventCh))

	case UIApprovalResolved:
		// Discriminator: pendingApproval is still true iff the cloud auto-
		// resolved this request server-side (under ApprovalMode=auto/balanced).
		// The manual path clears state locally on Enter before sending its
		// user_confirmation upstream, so when the cloud echoes that back here
		// pendingApproval is already false — and we no-op, which is correct.
		// The ID match guards against a stale resolved event arriving after
		// the user has already moved on to a different approval.
		if m.pendingApproval && msg.ApprovalID == m.pendingApprovalID {
			cmds = append(cmds, m.commitBlock(block{
				kind:           blockApprovalAuto,
				approved:       msg.Approved,
				autoIsQuestion: m.pendingIsQuestion,
			}))
			m.clearPendingPrompt()
		}
		cmds = append(cmds, waitForEvent(m.eventCh))

	default:
		// Pass unhandled messages to the textarea (e.g. blink).
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		cmds = append(cmds, tiCmd)
	}

	return m, tea.Batch(cmds...)
}

// View returns the rendered live frame: live blocks (busy spinner, in-flight
// streaming, in-flight pulumi op) above the input bar. Completed blocks aren't
// drawn here — they were committed to terminal scrollback via tea.Println as
// soon as they reached a terminal state. When the overlay is open the
// inline frame is suspended and the alt-screen overlay view is returned
// instead (with View.AltScreen=true so bubbletea v2 enters the alt buffer).
func (m Model) View() tea.View {
	if m.overlayActive {
		v := tea.NewView(m.overlay.View())
		v.AltScreen = true
		return v
	}
	return tea.NewView(m.viewString())
}

// viewString builds the live frame as a plain string. Kept separate from View
// so tests (and the View() wrapper) can compose its output without unpacking
// the tea.View struct.
func (m Model) viewString() string {
	hintText := "enter to send · shift+tab plan · ctrl+a auto-approval · " +
		"ctrl+r read-only · ctrl+o tool details · ctrl+c to quit"
	if m.busy {
		hintText = "agent is working · enter disabled · ctrl+o tool details · esc or ctrl+c to cancel"
	}
	hint := "  "
	chips := m.modeChips()
	if chips != "" {
		hint += chips
		hintText = " · " + hintText
	}
	if m.ctrlCArmed {
		// Override everything else: this is a transient prompt, the user just
		// pressed Ctrl+C and needs to see what a second press will do.
		hint = "  " + inputHintStyle.Render("Press Ctrl+C again to exit")
	} else {
		hint += inputHintStyle.Render(hintText)
	}

	// Lead with an empty line so the live frame (or, when idle, the prompt
	// itself) is always separated from the last block in scrollback by a
	// blank gap line. The busy spinner / streaming reply / in-flight pulumi
	// op deliberately sit directly above the prompt with no gap — they read
	// as part of the same input zone, and a chat-style spinner pinned to the
	// prompt is what users expect. The prompt and hint stay adjacent for
	// the same reason.
	parts := make([]string, 0, 5)
	parts = append(parts, "")
	if live := m.liveView(); live != "" {
		parts = append(parts, live)
	}
	if m.approvalPromptText != "" {
		parts = append(parts, "  "+m.approvalPromptText)
	}
	parts = append(parts, m.inputView(), hint)
	return strings.Join(parts, "\n")
}

func (m Model) inputView() string {
	ti := m.textInput
	ti.SetWidth(max(m.liveWidth(), 3))
	return ti.View()
}

func (m Model) prepareInitialScrollback(width, height int) (Model, string) {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	prepared := updated.(Model)
	rendered := prepared.committedScrollback()
	prepared.hasEmittedScrollback = true
	return prepared, rendered
}

// modeChips renders the status-bar chips for the three independent mode axes
// (plan, approval, permission). Default values are omitted so the status bar
// stays uncluttered until the user opts into something non-default.
func (m Model) modeChips() string {
	var chips []string
	if m.planMode {
		chips = append(chips, planAccentStyle.Render("⏸ plan mode"))
	}
	switch m.approvalMode {
	case client.NeoApprovalModeBalanced:
		chips = append(chips, planAccentStyle.Render("⚖ balanced"))
	case client.NeoApprovalModeAuto:
		chips = append(chips, planAccentStyle.Render("⚡ auto-approve"))
	case client.NeoApprovalModeManual:
		// Manual is the default — no chip, the status bar stays uncluttered.
	}
	if m.permissionMode == client.NeoPermissionModeReadOnly {
		chips = append(chips, planAccentStyle.Render("⊘ read-only"))
	}
	return strings.Join(chips, " ")
}

// rebuildMarkdownRenderer constructs a new glamour renderer at the current
// live width and a style matching the terminal background. Glamour bakes both
// in at construction, so callers rebuild on resize. We pick the style
// explicitly rather than via glamour.WithAutoStyle, which queries the terminal
// in-band and would race bubbletea's input reader (see runNeo).
func (m *Model) rebuildMarkdownRenderer() {
	style := "dark"
	if !m.hasDarkBackground {
		style = "light"
	}
	if r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(m.liveWidth()-4),
	); err == nil {
		m.mdRenderer = r
	}
}

// liveWidth returns the width to use when rendering live-frame content:
// the terminal width minus a small cushion that keeps glamour, lipgloss,
// and the textarea off the wrap column.
func (m *Model) liveWidth() int {
	const (
		margin         = 4
		minUsableWidth = 40 // below this, give up the cushion to keep something visible
	)
	if m.width > minUsableWidth {
		return m.width - margin
	}
	return m.width
}

// liveView renders only the in-flight blocks that should occupy the live
// frame: busy spinner (always pinned at the bottom of the live region) and
// an open pulumi op block. The busy block's spinner glyph is read from
// m.spinner.View() at render time so the animation tracks the current frame
// without re-caching per block.
//
// We deliberately use strings.Join instead of lipgloss.JoinVertical:
// JoinVertical pads every line to the widest constituent's column count with
// trailing spaces, which makes resize-shrink wrap every line and desync the
// inline-renderer line accounting. strings.Join keeps each line at its
// natural length so only genuinely-wide content is at risk.
func (m *Model) liveView() string {
	var parts []string
	for _, b := range m.blocks {
		if !isLiveKind(b) {
			continue
		}
		if b.kind == blockBusy {
			parts = append(parts, "  "+m.spinner.View()+" "+shimmerLabel(b.label, b.shimmer, m.frame, m.hasDarkBackground))
		} else {
			parts = append(parts, b.rendered)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	// "\n\n" separator gives one blank line between live blocks (e.g. an
	// in-flight pulumi op stack and the busy spinner pinned beneath it) so
	// the live frame matches the breathing room used between committed blocks
	// in scrollback. Empty separator lines are safe under the inline-renderer
	// width constraint above — only wide lines wrap on resize.
	return strings.Join(parts, "\n\n")
}

// isLiveKind reports whether a block belongs in the live frame (true) or has
// been committed to terminal scrollback (false).
func isLiveKind(b block) bool {
	switch b.kind {
	case blockBusy:
		return true
	case blockPulumiOp:
		return b.pulumi != nil && !b.pulumi.done
	case blockToolComplete, blockAssistantFinal, blockError, blockWarning,
		blockCancelled, blockUserMessage, blockApprovalPlan,
		blockApprovalGeneral, blockApprovalChoice, blockApprovalAuto,
		blockTodoList, blockQuestion, blockAnswerSubmitted:
		return false
	}
	return false
}

func isCommittedKind(b block) bool { return !isLiveKind(b) }

func (m *Model) stageBlock(b block) {
	m.renderBlock(&b)
	m.appendBlock(b)
}

func (m *Model) applyHistoryEvent(ev UIEvent) {
	switch msg := ev.(type) {
	case UIAssistantMessage:
		if msg.Content != "" {
			m.stageBlock(block{kind: blockAssistantFinal, raw: msg.Content})
		}
		_ = m.applyBusyForEvent(msg)
		m.clearStaleHistoryApproval(msg)
	case UIToolStarted:
		m.toolHistory = appendToolStart(m.toolHistory, msg.Name, msg.Args)
		_ = m.applyBusyForEvent(msg)
	case UIToolProgress:
		_ = m.applyBusyForEvent(msg)
	case UIToolCompleted:
		completeToolCall(m.toolHistory, msg.Name, msg.Result, msg.IsError)
		m.stageBlock(toolCompletedBlock(msg.Name, msg.Args, msg.IsError))
		_ = m.applyBusyForEvent(msg)
	case UIError:
		_ = m.applyBusyForEvent(msg)
		m.stageBlock(block{kind: blockError, raw: msg.Message})
		m.clearStaleHistoryApproval(msg)
	case UIWarning:
		_ = m.applyBusyForEvent(msg)
		m.stageBlock(block{kind: blockWarning, raw: msg.Message})
	case UIReconnecting, UIReconnected, UIAwaitingApprovals, UIContextCompression:
		_ = m.applyBusyForEvent(msg)
	case UICancelled:
		_ = m.applyBusyForEvent(msg)
		m.stageBlock(block{kind: blockCancelled, raw: "Session cancelled."})
		m.clearStaleHistoryApproval(msg)
	case UITaskIdle:
		_ = m.applyBusyForEvent(msg)
		m.clearStaleHistoryApproval(msg)
	case UISessionURL:
		m.welcome.consoleURL = msg.URL
		m.taskCreated = true
	case UIUserMessage:
		m.stageBlock(block{kind: blockUserMessage, raw: msg.Content})
		_ = m.applyBusyForEvent(msg)
	case UIApprovalRequest:
		_ = m.applyBusyForEvent(msg)
		m.pendingApproval = true
		m.pendingApprovalID = msg.ApprovalID
		m.pendingApprovalType = msg.ApprovalType
		m.pendingIsQuestion = false
		m.textInput.Placeholder = ""
		m.textInput.Reset()
		switch {
		case m.pendingApprovalType == approvalTypePlanExit:
			m.stageBlock(block{kind: blockApprovalPlan, raw: msg.PlanDescription, todos: m.pendingTodos})
			m.pendingTodos = nil
			m.approvalPromptText = warningStyle.Render("Approve plan? [y to approve / reason to deny]:")
		case isAskUserToolName(msg.ToolName):
			m.pendingIsQuestion = true
			m.stageBlock(block{kind: blockQuestion, raw: msg.Message})
			m.approvalPromptText = promptStyle.Render("Your answer:")
		default:
			m.stageBlock(block{kind: blockApprovalGeneral, raw: msg.Message})
			m.approvalPromptText = warningStyle.Render("Approve? [y to approve / reason to deny]:")
		}
	case UIApprovalResolved:
		if m.pendingApproval && msg.ApprovalID == m.pendingApprovalID {
			m.stageBlock(block{
				kind:           blockApprovalAuto,
				approved:       msg.Approved,
				autoIsQuestion: m.pendingIsQuestion,
			})
			m.clearPendingPrompt()
		}
	case UIPulumiStart:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx < 0 {
			m.stageBlock(block{kind: blockPulumiOp, pulumi: &pulumiBlockState{
				toolName:      msg.ToolName,
				stackName:     msg.StackName,
				isPreview:     msg.IsPreview,
				resourceByURN: map[string]int{},
			}})
		}
		_ = m.applyBusyForEvent(msg)
	case UIPulumiResource:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			m.blocks[idx].pulumi.addResource(msg.Op, msg.URN, msg.Type, msg.Status)
			m.renderBlock(&m.blocks[idx])
		}
		_ = m.applyBusyForEvent(msg)
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
		_ = m.applyBusyForEvent(msg)
	case UIPulumiEnd:
		if idx := m.findOpenPulumiBlock(msg.ToolName); idx >= 0 {
			st := m.blocks[idx].pulumi
			st.counts = msg.Counts
			st.elapsed = msg.Elapsed
			st.err = msg.Err
			st.done = true
			m.renderBlock(&m.blocks[idx])
		}
		_ = m.applyBusyForEvent(msg)
	case UITodoList:
		if len(msg.Items) > 0 {
			if m.planMode {
				m.pendingTodos = msg.Items
			} else {
				m.stageBlock(block{kind: blockTodoList, todos: msg.Items})
			}
		}
		_ = m.applyBusyForEvent(msg)
	}
}

func toolCompletedBlock(name string, args json.RawMessage, isError bool) block {
	marker := toolOKMarker
	if isError {
		marker = toolErrMarker
	}
	return block{
		kind:     blockToolComplete,
		rendered: "  " + marker + " " + styledToolLabel(name, args),
	}
}

func (m *Model) clearStaleHistoryApproval(ev UIEvent) {
	if !m.pendingApproval || !isFinalUIEvent(ev) {
		return
	}
	m.clearPendingPrompt()
}

// commitBlock renders b, appends it to m.blocks, and returns a tea.Cmd that
// prints the rendered string as new terminal scrollback. Returns nil when the
// block renders empty (e.g. an empty assistant final from a hand-off).
func (m *Model) commitBlock(b block) tea.Cmd {
	m.renderBlock(&b)
	m.appendBlock(b)
	if b.rendered == "" {
		return nil
	}
	return m.printlnBlock(b.rendered)
}

// committedScrollback returns the welcome banner followed by every committed
// block's rendered text, in transcript order, joined with the same blank-line
// separator printlnBlock puts between incremental prints — i.e. everything
// that belongs in terminal scrollback, as one string ready for a single
// atomic tea.Println. Used for the initial flush and to re-emit the
// transcript after a suspend/resume.
func (m Model) committedScrollback() string {
	out := []string{m.welcome.View()}
	for _, b := range m.blocks {
		if isCommittedKind(b) && b.rendered != "" {
			out = append(out, b.rendered)
		}
	}
	return strings.Join(out, "\n\n")
}

// printlnBlock emits rendered to scrollback, prepending a blank line so each
// committed block has visual breathing room from whatever came before. The
// very first emission (the welcome banner) skips the leading newline so the
// session doesn't open with empty space above the banner. Every other
// scrollback emission in the TUI must go through this helper rather than
// calling tea.Println directly so spacing stays uniform across block kinds.
func (m *Model) printlnBlock(rendered string) tea.Cmd {
	if !m.hasEmittedScrollback {
		m.hasEmittedScrollback = true
		return tea.Println(rendered)
	}
	return tea.Println("\n" + rendered)
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
	case UIReconnecting:
		return "Reconnecting...", shimmerVerb, true
	case UIReconnected:
		return thinkingLabel, shimmerVerb, true
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

func (m *Model) appendWarningBlock(msg string) tea.Cmd {
	return m.commitBlock(block{kind: blockWarning, raw: msg})
}

func (m *Model) appendUserMessageBlock(content string) tea.Cmd {
	return m.commitBlock(block{kind: blockUserMessage, raw: content})
}

// clearPendingPrompt resets all pendingApproval/pendingIsQuestion state and
// returns the text input to its default "send a message" appearance.
func (m *Model) clearPendingPrompt() {
	m.pendingApproval = false
	m.pendingApprovalID = ""
	m.pendingApprovalType = ""
	m.pendingIsQuestion = false
	m.approvalPromptText = ""
	m.textInput.Placeholder = "Send a message..."
	m.textInput.Reset()
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

// scheduleApprovalDebounce advances the approval-debounce generation and
// returns a tea.Cmd that posts an approvalDebounceTickMsg after
// modeToggleDebounce. The Update handler dispatches the PATCH only if the
// tick's gen still matches the current one — a subsequent Ctrl+A press
// advances the gen and silently retires the stale tick.
func (m *Model) scheduleApprovalDebounce() tea.Cmd {
	m.approvalDebounceGen++
	gen := m.approvalDebounceGen
	return tea.Tick(modeToggleDebounce, func(time.Time) tea.Msg {
		return approvalDebounceTickMsg{gen: gen}
	})
}

// schedulePermissionDebounce is the permission-mode counterpart to
// scheduleApprovalDebounce.
func (m *Model) schedulePermissionDebounce() tea.Cmd {
	m.permissionDebounceGen++
	gen := m.permissionDebounceGen
	return tea.Tick(modeToggleDebounce, func(time.Time) tea.Msg {
		return permissionDebounceTickMsg{gen: gen}
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
// width and markdown renderer. blockApprovalChoice and blockApprovalAuto are
// the two kinds that still render when raw is empty (their state is carried
// by b.approved / b.autoIsQuestion instead).
func (m *Model) renderBlock(b *block) {
	if b.kind == blockApprovalChoice {
		m.renderApprovalChoice(b)
		return
	}
	if b.kind == blockApprovalAuto {
		m.renderApprovalAuto(b)
		return
	}
	if b.kind == blockPulumiOp {
		b.rendered = m.renderPulumiBlock(b.pulumi)
		return
	}
	if b.kind == blockTodoList {
		b.rendered = renderHeaderedBlock(todoListHeader, renderTodoLines(b.todos))
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
		b.rendered = m.renderUserMessage(b.raw)
	case blockAssistantFinal:
		b.rendered = m.renderAssistantFinal(m.renderMarkdown(b.raw))
	case blockApprovalPlan:
		header := planAccentStyle.Render("⏺ Proposed plan")
		body := m.renderMarkdown(b.raw)
		// Fold any todos buffered during plan mode into the plan body. A
		// blank line separates them from the plan markdown so glamour's
		// last paragraph doesn't visually run into the Tasks header.
		if len(b.todos) > 0 {
			body = strings.TrimRight(body, "\n") + "\n\nTasks:\n" + renderTodoLines(b.todos)
		}
		b.rendered = renderHeaderedBlock(header, body)
	case blockApprovalGeneral:
		header := warningStyle.Render("⚠ Approval required")
		b.rendered = renderHeaderedBlock(header, m.wrapPlain(b.raw))
	case blockQuestion:
		// Cyan promptStyle (matches the ❯ input prompt) so the header reads
		// as an input affordance, not a warning.
		header := promptStyle.Render("◆ Question")
		b.rendered = renderHeaderedBlock(header, m.wrapPlain(b.raw))
	case blockAnswerSubmitted:
		answered := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✎ Answered")
		b.rendered = renderIndented(lipgloss.NewStyle(), m.width, answered+" — "+b.raw)
	case blockBusy, blockToolComplete:
		// No raw: blockBusy renders live from label, blockToolComplete is
		// pre-styled at event time.
	case blockApprovalChoice, blockApprovalAuto, blockPulumiOp, blockTodoList:
		// Unreachable: handled by early returns above.
	}
}

// renderTodoLines formats todos as one ASCII checkbox per line, sorted by
// Index so the agent's intended ordering survives JSON round-tripping.
func renderTodoLines(items []UITodoItem) string {
	if len(items) == 0 {
		return ""
	}
	sorted := append([]UITodoItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })
	lines := make([]string, 0, len(sorted))
	for _, it := range sorted {
		var marker, content string
		switch it.Status {
		case "completed":
			marker, content = "[x]", todoCompletedStyle.Render(it.Content)
		case "in_progress":
			marker, content = "[~]", todoActiveStyle.Render(it.Content)
		default:
			marker, content = "[ ]", it.Content
		}
		lines = append(lines, marker+" "+content)
	}
	return strings.Join(lines, "\n")
}

// renderApprovalAuto renders a feedback block committed when the cloud
// auto-resolves a pending approval/question under ApprovalMode=auto or
// balanced. The verb depends on what was asked (approval vs ask-user) and
// whether the cloud reported ok=true; today auto-deny doesn't exist on the
// cloud side but we render it anyway in case that changes.
func (m *Model) renderApprovalAuto(b *block) {
	verb := "Auto-approved"
	switch {
	case !b.approved:
		verb = "Auto-denied"
	case b.autoIsQuestion:
		verb = "Auto-answered"
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	b.rendered = "  " + style.Render("⚡ "+verb)
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

// renderUserMessage renders an echoed user message: the cyan-bold ❯ prefix
// marks it as user input, the content renders plain so it reads on any
// background. Continuation lines indent two spaces so they sit under the
// message body, not under the prompt glyph.
func (m *Model) renderUserMessage(content string) string {
	prefix := promptStyle.Render("❯") + " " // visible width 2
	wrap := m.liveWidth() - 2
	if wrap < 4 {
		return prefix + content
	}
	lines := strings.Split(wordwrap.String(content, wrap), "\n")
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(lines[0])
	for _, line := range lines[1:] {
		sb.WriteString("\n  ")
		sb.WriteString(line)
	}
	return sb.String()
}

// wrapPlain word-wraps non-markdown text to the safe live width (m.liveWidth)
// so streaming text never sits at the terminal wrap boundary. We use
// reflow's wordwrap (which only inserts \n at word boundaries) instead of
// lipgloss.Style.Width(W).Render, which also pads every line to W with
// trailing spaces — those full-width lines wrap on resize-shrink and desync
// the inline-renderer line accounting.
func (m *Model) wrapPlain(text string) string {
	w := m.liveWidth()
	if w <= 4 {
		return linkifyURLs(text)
	}
	return linkifyURLs(wordwrap.String(text, w-4))
}

// renderMarkdown renders text through glamour, falling back to plain text.
// URLs in the rendered output are wrapped in OSC 8 escapes so terminals that
// support hyperlinks render them as clickable.
func (m *Model) renderMarkdown(text string) string {
	if m.mdRenderer == nil {
		return linkifyURLs(text)
	}
	rendered, err := m.mdRenderer.Render(text)
	if err != nil {
		return linkifyURLs(text)
	}
	return linkifyURLs(strings.TrimRight(rendered, "\n"))
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

// finalMarker returns the ⏺ glyph for a final assistant message, colored
// to contrast with the terminal background (white on dark, black on light).
func (m *Model) finalMarker() string {
	fg := lipgloss.LightDark(m.hasDarkBackground)(lipgloss.Color("0"), lipgloss.Color("15"))
	return lipgloss.NewStyle().Foreground(fg).Render("⏺")
}

// renderAssistantFinal renders a final assistant message with a circle marker.
func (m *Model) renderAssistantFinal(rendered string) string {
	trimmed := strings.TrimLeft(rendered, "\n ")
	if trimmed == "" {
		return ""
	}
	firstLine, rest, _ := strings.Cut(trimmed, "\n")
	return renderHeaderedBlock(m.finalMarker()+" "+firstLine, rest)
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
