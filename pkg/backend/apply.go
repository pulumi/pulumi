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

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
)

// ApplierOptions is a bag of configuration settings for an Applier.
type ApplierOptions struct {
	// DryRun indicates if the update should not change any resource state and instead just preview changes.
	DryRun bool
	// ShowLink indicates if a link to the update persisted result can be displayed.
	ShowLink bool
}

// Applier applies the changes specified by this update operation against the target stack.
type Applier func(ctx context.Context, kind apitype.UpdateKind, stack Stack, op UpdateOperation,
	opts ApplierOptions, events chan<- engine.Event) (engine.ResourceChanges, result.Result)

func ActionLabel(kind apitype.UpdateKind, dryRun bool) string {
	v := updateTextMap[kind]
	contract.Assert(v.previewText != "")
	contract.Assert(v.text != "")

	if dryRun {
		return "Previewing " + v.previewText
	}

	return v.text
}

var updateTextMap = map[apitype.UpdateKind]struct {
	previewText string
	text        string
}{
	apitype.PreviewUpdate:        {"update", "Previewing"},
	apitype.UpdateUpdate:         {"update", "Updating"},
	apitype.RefreshUpdate:        {"refresh", "Refreshing"},
	apitype.DestroyUpdate:        {"destroy", "Destroying"},
	apitype.StackImportUpdate:    {"stack import", "Importing"},
	apitype.ResourceImportUpdate: {"import", "Importing"},
}

type response string

const (
	yes     response = "yes"
	no      response = "no"
	details response = "details"
)

func PreviewThenPrompt(ctx context.Context, kind apitype.UpdateKind, stack Stack,
	op UpdateOperation, apply Applier) (engine.ResourceChanges, result.Result) {
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
	// We perform the preview (DryRun), but don't display the cloud link since the
	// thing the user cares about would be the link to the actual update if they
	// confirm the prompt.
	opts := ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}

	changes, res := apply(ctx, kind, stack, op, opts, eventsChannel)
	if res != nil {
		close(eventsChannel)
		return changes, res
	}

	// If there are no changes, or we're auto-approving or just previewing, we can skip the confirmation prompt.
	if op.Opts.AutoApprove || kind == apitype.PreviewUpdate {
		close(eventsChannel)
		return changes, nil
	}

	// Otherwise, ensure the user wants to proceed.
	res = confirmBeforeUpdating(kind, stack, events, op.Opts)
	close(eventsChannel)
	return changes, res
}

// confirmBeforeUpdating asks the user whether to proceed. A nil error means yes.
func confirmBeforeUpdating(kind apitype.UpdateKind, stack Stack,
	events []engine.Event, opts UpdateOptions) result.Result {
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
			previewWarning = colors.SpecWarning + " without a preview" + colors.Bold
		}

		// Create a prompt. If this is a refresh, we'll add some extra text so it's clear we aren't updating resources.
		prompt := "\b" + opts.Display.Color.Colorize(
			colors.SpecPrompt+fmt.Sprintf("Do you want to perform this %s%s?",
				updateTextMap[kind].previewText, previewWarning)+colors.Reset)
		if kind == apitype.RefreshUpdate {
			prompt += "\n" +
				opts.Display.Color.Colorize(colors.SpecImportant+
					"No resources will be modified as part of this refresh; just your stack's state will be."+
					colors.Reset)
		}

		cmdutil.EndKeypadTransmitMode()

		// Now prompt the user for a yes, no, or details, and then proceed accordingly.
		if err := survey.AskOne(&survey.Select{
			Message: prompt,
			Options: choices,
			Default: string(no),
		}, &response, nil); err != nil {
			return result.FromError(errors.Wrapf(err, "confirmation cancelled, not proceeding with the %s", kind))
		}

		if response == string(no) {
			fmt.Printf("confirmation declined, not proceeding with the %s\n", kind)
			return result.Bail()
		}

		if response == string(yes) {
			return nil
		}

		if response == string(details) {
			diff := createDiff(kind, events, opts.Display)
			_, err := os.Stdout.WriteString(diff + "\n")
			contract.IgnoreError(err)
			continue
		}
	}
}

func PreviewThenPromptThenExecute(ctx context.Context, kind apitype.UpdateKind, stack Stack,
	op UpdateOperation, apply Applier) (engine.ResourceChanges, result.Result) {
	// Preview the operation to the user and ask them if they want to proceed.

	if !op.Opts.SkipPreview {
		changes, res := PreviewThenPrompt(ctx, kind, stack, op, apply)
		if res != nil || kind == apitype.PreviewUpdate {
			return changes, res
		}
	}

	// Perform the change (!DryRun) and show the cloud link to the result.
	// We don't care about the events it issues, so just pass a nil channel along.
	opts := ApplierOptions{
		DryRun:   false,
		ShowLink: true,
	}
	return apply(ctx, kind, stack, op, opts, nil /*events*/)
}

func createDiff(updateKind apitype.UpdateKind, events []engine.Event, displayOpts display.Options) string {
	buff := &bytes.Buffer{}

	seen := make(map[resource.URN]engine.StepEventMetadata)
	displayOpts.SummaryDiff = true

	for _, e := range events {
		msg := display.RenderDiffEvent(updateKind, e, seen, displayOpts)
		if msg != "" && e.Type != engine.SummaryEvent {
			_, err := buff.WriteString(msg)
			contract.IgnoreError(err)
		}
	}

	return strings.TrimSpace(buff.String())
}
