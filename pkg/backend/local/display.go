// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package local

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	// "github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"

	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/progress"
	"github.com/docker/docker/pkg/streamformatter"
	"github.com/docker/docker/pkg/term"
)

// copied from: https://github.com/docker/cli/blob/master/cli/command/out.go
// replace with usage of that library when we can figure out hte right version story

type commonStream struct {
	fd         uintptr
	isTerminal bool
	state      *term.State
}

// FD returns the file descriptor number for this stream
func (s *commonStream) FD() uintptr {
	return s.fd
}

// IsTerminal returns true if this stream is connected to a terminal
func (s *commonStream) IsTerminal() bool {
	return s.isTerminal
}

// RestoreTerminal restores normal mode to the terminal
func (s *commonStream) RestoreTerminal() {
	if s.state != nil {
		term.RestoreTerminal(s.fd, s.state)
	}
}

// SetIsTerminal sets the boolean used for isTerminal
func (s *commonStream) SetIsTerminal(isTerminal bool) {
	s.isTerminal = isTerminal
}

type outStream struct {
	commonStream
	out io.Writer
}

func (o *outStream) Write(p []byte) (int, error) {
	return o.out.Write(p)
}

// SetRawTerminal sets raw mode on the input terminal
func (o *outStream) SetRawTerminal() (err error) {
	if os.Getenv("NORAW") != "" || !o.commonStream.isTerminal {
		return nil
	}
	o.commonStream.state, err = term.SetRawTerminalOutput(o.commonStream.fd)
	return err
}

// GetTtySize returns the height and width in characters of the tty
func (o *outStream) GetTtySize() (uint, uint) {
	if !o.isTerminal {
		return 0, 0
	}
	ws, err := term.GetWinsize(o.fd)
	if err != nil {
		if ws == nil {
			return 0, 0
		}
	}
	return uint(ws.Height), uint(ws.Width)
}

// NewOutStream returns a new OutStream object from a Writer
func newOutStream(out io.Writer) *outStream {
	fd, isTerminal := term.GetFdInfo(out)
	return &outStream{commonStream: commonStream{fd: fd, isTerminal: isTerminal}, out: out}
}

func writeDistributionProgress(outStream io.Writer, progressChan <-chan progress.Progress) {
	progressOutput := streamformatter.NewJSONStreamFormatter().NewProgressOutput(outStream, false)

	for prog := range progressChan {
		// fmt.Printf("Received progress")
		progressOutput.WriteProgress(prog)
	}
}

// DisplayEvents reads events from the `events` channel until it is closed, displaying each event as it comes in.
// Once all events have been read from the channel and displayed, it closes the `done` channel so the caller can
// await all the events being written.
func DisplayEvents(action string,
	events <-chan engine.Event, done chan<- bool, debug bool, opts backend.DisplayOptions) {
	// prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	// spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil)

	defer func() {
		// spinner.Reset()
		// ticker.Stop()
		done <- true
	}()

	_, stdout, _ := term.StdStreams()
	// _, isTerminal := term.GetFdInfo(stdout)

	pipeReader, pipeWriter := io.Pipe()
	progressChan := make(chan progress.Progress, 100)

	chanOutput := progress.ChanOutput(progressChan)

	go func() {
		writeDistributionProgress(pipeWriter, progressChan)
		pipeWriter.Close()
	}()

	// chanOutput.WriteProgress(progress.Progress{"Id"})

	go func() {
		for {
			select {
			// case <-ticker.C:
			// 	spinner.Tick()
			case event := <-events:
				fmt.Printf("Got event %v\n", event.Type)
				// spinner.Reset()

				out := os.Stdout
				if event.Type == engine.DiagEvent {
					payload := event.Payload.(engine.DiagEventPayload)
					if payload.Severity == diag.Error || payload.Severity == diag.Warning {
						out = os.Stderr
					}
				}

				msg := RenderEvent(event, debug, opts, chanOutput)
				if msg != "" && out != nil {
					fprintIgnoreError(out, msg)
				}

				if event.Type == engine.CancelEvent {
					fmt.Printf("Got cancel\n")
					close(progressChan)
					return
				}
			}
		}
	}()

	jsonmessage.DisplayJSONMessagesToStream(pipeReader, newOutStream(stdout), nil)
}

func RenderEvent(
	event engine.Event, debug bool,
	opts backend.DisplayOptions, chanOutput progress.Output) string {

	switch event.Type {
	case engine.CancelEvent:
		return ""
	case engine.PreludeEvent:
		return RenderPreludeEvent(event.Payload.(engine.PreludeEventPayload), opts, chanOutput)
	case engine.SummaryEvent:
		return RenderSummaryEvent(event.Payload.(engine.SummaryEventPayload), opts, chanOutput)
	case engine.ResourceOperationFailed:
		return RenderResourceOperationFailedEvent(event.Payload.(engine.ResourceOperationFailedPayload), opts, chanOutput)
	case engine.ResourceOutputsEvent:
		return RenderResourceOutputsEvent(event.Payload.(engine.ResourceOutputsEventPayload), opts, chanOutput)
	case engine.ResourcePreEvent:
		return RenderResourcePreEvent(event.Payload.(engine.ResourcePreEventPayload), opts, chanOutput)
	case engine.StdoutColorEvent:
		return RenderStdoutColorEvent(event.Payload.(engine.StdoutEventPayload), opts, chanOutput)
	case engine.DiagEvent:
		return RenderDiagEvent(event.Payload.(engine.DiagEventPayload), debug, opts, chanOutput)
	default:
		contract.Failf("unknown event type '%s'", event.Type)
		return ""
	}
}

var (
	newlineRegexp = regexp.MustCompile(`\r?\n`)
)

func trim(id string, msg string, opts backend.DisplayOptions) string {
	_, stdout, _ := term.StdStreams()
	fd, _ := term.GetFdInfo(stdout)
	size, _ := term.GetWinsize(fd)

	trimLength := (int(size.Width) - len(id)) - 5

	if trimLength <= 0 {
		return ""
	}

	msg = newlineRegexp.ReplaceAllString(msg, " ")

	for {
		newMsg := strings.Replace(msg, "  ", " ", -1)
		if newMsg == msg {
			break
		}

		msg = newMsg
	}

	msg = strings.TrimSpace(msg)

	if trimLength > len(msg) {
		trimLength = len(msg)
	}

	msg = msg[0:trimLength] + opts.Color.Colorize(colors.Reset)
	return msg
}

func RenderDiagEvent(
	payload engine.DiagEventPayload, debug bool,
	opts backend.DisplayOptions, chanOutput progress.Output) string {

	if payload.Severity == diag.Debug && !debug {
		return ""
	}

	msg := opts.Color.Colorize(payload.Message)

	if chanOutput != nil {
		id := "Diagnostics"
		chanOutput.WriteProgress(progress.Progress{
			ID:     id,
			Action: trim(id, msg, opts),
		})

		return ""
	}

	return msg
}

func RenderStdoutColorEvent(payload engine.StdoutEventPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {
	msg := opts.Color.Colorize(payload.Message)

	if chanOutput != nil {
		id := "Out"
		chanOutput.WriteProgress(progress.Progress{
			ID:     id,
			Action: trim(id, msg, opts),
		})

		return ""
	}

	return msg
}

func RenderSummaryEvent(event engine.SummaryEventPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {
	changes := event.ResourceChanges

	changeCount := 0
	for op, c := range changes {
		if op != deploy.OpSame {
			changeCount += c
		}
	}
	var kind string
	if event.IsPreview {
		kind = "previewed"
	} else {
		kind = "performed"
	}

	var changesLabel string
	if changeCount == 0 {
		kind = "required"
		changesLabel = "no"
	} else {
		changesLabel = strconv.Itoa(changeCount)
	}

	if changeCount > 0 || changes[deploy.OpSame] > 0 {
		kind += ":"
	}

	out := &bytes.Buffer{}
	fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vinfo%v: %v %v %v\n",
		colors.SpecInfo, colors.Reset, changesLabel, plural("change", changeCount), kind)))

	var planTo string
	if event.IsPreview {
		planTo = "to "
	}

	// Now summarize all of the changes; we print sames a little differently.
	for _, op := range deploy.StepOps {
		if op != deploy.OpSame {
			if c := changes[op]; c > 0 {
				opDescription := string(op)
				if !event.IsPreview {
					opDescription = op.PastTense()
				}
				fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("    %v%v %v %v%v%v\n",
					op.Prefix(), c, plural("resource", c), planTo, opDescription, colors.Reset)))
			}
		}
	}
	if c := changes[deploy.OpSame]; c > 0 {
		fprintfIgnoreError(out, "      %v %v unchanged\n", c, plural("resource", c))
	}

	// For actual deploys, we print some additional summary information
	if !event.IsPreview {
		if changeCount > 0 {
			fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vUpdate duration: %v%v\n",
				colors.SpecUnimportant, event.Duration, colors.Reset)))
		}
	}

	return out.String()
}

func RenderPreludeEvent(event engine.PreludeEventPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {
	out := &bytes.Buffer{}

	if opts.ShowConfig {
		fprintIgnoreError(out, opts.Color.Colorize(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset)))

		var keys []string
		for key := range event.Config {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fprintfIgnoreError(out, "    %v: %v\n", key, event.Config[key])
		}
	}

	action := "Previewing"
	if !event.IsPreview {
		action = "Performing"
	}

	fprintIgnoreError(out, opts.Color.Colorize(
		fmt.Sprintf("%v%v changes:%v\n", colors.SpecUnimportant, action, colors.Reset)))

	return out.String()
}

func RenderResourceOperationFailedEvent(
	payload engine.ResourceOperationFailedPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {

	// It's not actually useful or interesting to print out any details about
	// the resource state here, because we always assume that the resource state
	// is unknown if an error occurs.
	//
	// In the future, once we get more fine-grained error messages from providers,
	// we can provide useful diagnostics here.

	if chanOutput != nil {
		chanOutput.WriteProgress(progress.Progress{
			ID:     string(payload.Metadata.URN),
			Action: fmt.Sprintf("Operation failed! Status unknown."),
		})

		return ""
	}

	return ""
}

func RenderResourcePreEvent(
	payload engine.ResourcePreEventPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {

	out := &bytes.Buffer{}

	if shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata) {
		fprintIgnoreError(out, opts.Color.Colorize(payload.Summary))

		if !opts.Summary {
			fprintIgnoreError(out, opts.Color.Colorize(payload.Details))
		}

		fprintIgnoreError(out, opts.Color.Colorize(colors.Reset))
	}

	msg := out.String()

	if msg != "" && chanOutput != nil {
		id := string(payload.Metadata.URN)
		chanOutput.WriteProgress(progress.Progress{
			ID:     id,
			Action: trim(id, msg, opts),
		})

		return ""
	}

	return msg
}

func RenderResourceOutputsEvent(
	payload engine.ResourceOutputsEventPayload, opts backend.DisplayOptions, chanOutput progress.Output) string {

	out := &bytes.Buffer{}
	if (shouldShow(payload.Metadata, opts) || isRootStack(payload.Metadata)) && !opts.Summary {
		fprintIgnoreError(out, opts.Color.Colorize(payload.Text))
	}

	msg := out.String()

	if msg != "" && chanOutput != nil {
		id := string(payload.Metadata.URN)
		chanOutput.WriteProgress(progress.Progress{
			ID:     id,
			Action: trim(id, msg, opts),
		})

		return ""
	}

	return msg
}

// isRootStack returns true if the step pertains to the rootmost stack component.
func isRootStack(step engine.StepEventMetdata) bool {
	return step.URN.Type() == resource.RootStackType
}

// shouldShow returns true if a step should show in the output.
func shouldShow(step engine.StepEventMetdata, opts backend.DisplayOptions) bool {
	// For certain operations, whether they are tracked is controlled by flags (to cut down on superfluous output).
	if step.Op == deploy.OpSame {
		// If the op is the same, it is possible that the resource's metadata changed.  In that case, still show it.
		if step.Old.Protect != step.New.Protect {
			return true
		}
		return opts.ShowSames
	} else if step.Op == deploy.OpCreateReplacement || step.Op == deploy.OpDeleteReplaced {
		return opts.ShowReplacementSteps
	} else if step.Op == deploy.OpReplace {
		return !opts.ShowReplacementSteps
	}
	return true
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}

func fprintfIgnoreError(w io.Writer, format string, a ...interface{}) {
	_, err := fmt.Fprintf(w, format, a...)
	contract.IgnoreError(err)
}

func fprintIgnoreError(w io.Writer, a ...interface{}) {
	_, err := fmt.Fprint(w, a...)
	contract.IgnoreError(err)
}
