// Copyright 2016-2018, Pulumi Corporation.
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

package backend

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// Applier applies the changes specified by this update operation against the target stack. If dryRun is true,
// a preview is performed, rather than an actual update, and in any case events are written to the events channel.
type Applier func(ctx context.Context, kind apitype.UpdateKind, stack Stack, op UpdateOperation,
	dryRun, persist bool, events chan<- engine.Event) (engine.ResourceChanges, error)

var (
	updateTextMap = map[apitype.UpdateKind]struct {
		previewText string
		text        string
	}{
		apitype.PreviewUpdate: {"update of", "Previewing"},
		apitype.UpdateUpdate:  {"update of", "Updating"},
		apitype.RefreshUpdate: {"refresh of", "Refreshing"},
		apitype.DestroyUpdate: {"destroy of", "Destroying"},
		apitype.ImportUpdate:  {"import to", "Importing into"},
	}
)

func ActionLabel(kind apitype.UpdateKind, dryRun bool) string {
	v := updateTextMap[kind]
	contract.Assert(v.previewText != "")
	contract.Assert(v.text != "")

	if dryRun {
		return "Previewing " + v.previewText
	}

	return v.text
}

type response string

const (
	yes     response = "yes"
	no      response = "no"
	details response = "details"
)

func PreviewThenPrompt(ctx context.Context, kind apitype.UpdateKind, stack Stack,
	op UpdateOperation, apply Applier) (engine.ResourceChanges, error) {
	// create a channel to hear about the update events from the engine. this will be used so that
	// we can build up the diff display in case the user asks to see the details of the diff

	// Note that eventsChannel is not closed in a `defer`. It is generally unsafe to do so, since defers run during
	// panics and we can't know whether or not we were in the middle of writing to this channel when the panic occurred.
	//
	// Instead of using a `defer`, we manually close `eventsChannel` on every exit of this function.
	eventsChannel := make(chan engine.Event)

	var events []engine.Event
	go func() {
		// pull the events from the channel and store them locally
		for e := range eventsChannel {
			if e.Type == engine.ResourcePreEvent ||
				e.Type == engine.ResourceOutputsEvent ||
				e.Type == engine.SummaryEvent {

				events = append(events, e)
			}
		}
	}()

	// Perform the update operations, passing true for dryRun, so that we get a preview.
	changes := engine.ResourceChanges(nil)
	if !op.Opts.SkipPreview {
		c, err := apply(ctx, kind, stack, op, true /*dryRun*/, false /*persist*/, eventsChannel)
		if err != nil {
			close(eventsChannel)
			return c, err
		}
		changes = c
	}

	// If there are no changes, or we're auto-approving or just previewing, we can skip the confirmation prompt.
	if op.Opts.AutoApprove || kind == apitype.PreviewUpdate {
		close(eventsChannel)
		return changes, nil
	}

	// Otherwise, ensure the user wants to proceed.
	err := confirmBeforeUpdating(kind, stack, events, op.Opts)
	close(eventsChannel)
	return changes, err
}

// confirmBeforeUpdating asks the user whether to proceed. A nil error means yes.
func confirmBeforeUpdating(kind apitype.UpdateKind, stack Stack,
	events []engine.Event, opts UpdateOptions) error {
	for {
		var response string

		surveycore.DisableColor = true
		surveycore.QuestionIcon = ""
		surveycore.SelectFocusIcon = opts.Display.Color.Colorize(colors.BrightGreen + ">" + colors.Reset)

		choices := []string{string(yes), string(no)}

		// For non-previews, we can also offer a detailed summary.
		if !opts.SkipPreview {
			choices = append(choices, string(details))
		}

		var previewWarning string
		if opts.SkipPreview {
			previewWarning = colors.SpecWarning + " without a preview" + colors.BrightWhite
		}

		// Create a prompt. If this is a refresh, we'll add some extra text so it's clear we aren't updating resources.
		prompt := "\b" + opts.Display.Color.Colorize(
			colors.BrightWhite+fmt.Sprintf("Do you want to perform this %s%s?",
				kind, previewWarning)+colors.Reset)
		if kind == apitype.RefreshUpdate {
			prompt += "\n" +
				opts.Display.Color.Colorize(colors.BrightYellow+
					"No resources will be modified as part of this refresh; just your stack's state will be."+
					colors.Reset)
		}

		// Now prompt the user for a yes, no, or details, and then proceed accordingly.
		if err := survey.AskOne(&survey.Select{
			Message: prompt,
			Options: choices,
			Default: string(no),
		}, &response, nil); err != nil {
			return errors.Wrapf(err, "confirmation cancelled, not proceeding with the %s", kind)
		}

		if response == string(no) {
			return errors.Errorf("confirmation declined, not proceeding with the %s", kind)
		}

		if response == string(yes) {
			return nil
		}

		if response == string(details) {
			diff := createDiff(kind, events, opts.Display)
			_, err := os.Stdout.WriteString(diff + "\n\n")
			contract.IgnoreError(err)
			continue
		}
	}
}

func PreviewThenPromptThenExecute(ctx context.Context, kind apitype.UpdateKind, stack Stack,
	op UpdateOperation, apply Applier) (engine.ResourceChanges, error) {
	// Preview the operation to the user and ask them if they want to proceed.
	changes, err := PreviewThenPrompt(ctx, kind, stack, op, apply)
	if err != nil || kind == apitype.PreviewUpdate {
		return changes, err
	}

	// Now do the real operation. We don't care about the events it issues, so just pass a nil channel along.
	return apply(ctx, kind, stack, op, false /*dryRun*/, true /*persist*/, nil /*events*/)
}

func createDiff(updateKind apitype.UpdateKind, events []engine.Event, displayOpts display.Options) string {
	buff := &bytes.Buffer{}

	seen := make(map[resource.URN]engine.StepEventMetadata)
	displayOpts.SummaryDiff = true

	for _, e := range events {
		msg := display.RenderDiffEvent(updateKind, e, seen, displayOpts)
		if msg != "" {
			if e.Type == engine.SummaryEvent {
				msg = "\n" + msg
			}

			_, err := buff.WriteString(msg)
			contract.IgnoreError(err)
		}
	}

	return strings.TrimSpace(buff.String())
}
