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

package do

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	backenddisplay "github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type displayedStep struct {
	Op           display.StepOp
	Old, New     *resource.State
	Diffs        []resource.PropertyKey
	DetailedDiff map[string]plugin.PropertyDiff
	Preview      bool
}

func (pc *packageCommand) runDisplayedStep(
	cmd *cobra.Command, step displayedStep, call func() (*resource.State, error),
) error {
	urn := step.urn()
	subject := step.subject()
	preview := (pc.dryrun || step.Preview) && step.Op != deploy.OpRead

	if pc.jsonOut && !step.Preview {
		state, err := call()
		if err != nil || state == nil {
			return tidyProviderError(err, urn, subject)
		}
		return pc.printResourceResult(cmd, state)
	}

	stderr := cmd.ErrOrStderr()
	opts := backenddisplay.Options{
		Color:               cmdutil.GetGlobalColorization(),
		IsInteractive:       cmdutil.Interactive(),
		Type:                backenddisplay.DisplayProgress,
		Stdin:               cmd.InOrStdin(),
		Stdout:              stderr,
		Stderr:              stderr,
		ShowSecrets:         pc.showSecrets,
		ShowReads:           true,
		ShowResourceChanges: preview || len(step.Diffs) > 0 || len(step.DetailedDiff) > 0,
		SuppressProgress:    true,
		SuppressStackRow:    true,
	}

	kind := apitype.UpdateUpdate
	if step.Op == deploy.OpDelete {
		kind = apitype.DestroyUpdate
	}

	events := make(chan engine.Event)
	done := make(chan bool)
	go backenddisplay.ShowEvents(
		string(step.Op), kind, doDisplayStack, doDisplayProject,
		"" /*permalink*/, events, done, opts, preview)

	metadata := step.metadata(pc.showSecrets)
	start := time.Now()
	events <- engine.NewEvent(engine.ResourcePreEventPayload{Metadata: metadata, Planning: preview})

	forward := func(ephemeral bool) diagForwarder {
		return func(sev diag.Severity, d *diag.Diag, args ...any) {
			prefix, msg := stringifyDiag(sev, d, args...)
			events <- engine.NewEvent(engine.DiagEventPayload{
				URN:       urn,
				Prefix:    logging.FilterString(prefix),
				Message:   logging.FilterString(tidyProviderMessage(msg, urn, subject)),
				Color:     colors.Raw,
				Severity:  sev,
				StreamID:  d.StreamID,
				Ephemeral: ephemeral,
			})
		}
	}
	pc.diagFwd.set(forward(false))
	pc.statusFwd.set(forward(true))

	result, err := call()
	pc.diagFwd.clear()
	pc.statusFwd.clear()
	if err != nil {
		err = tidyProviderError(err, urn, subject)
		events <- engine.NewEvent(engine.ResourceOperationFailedPayload{
			Metadata: metadata,
			Status:   resource.StatusOK,
			Steps:    1,
		})
	} else {
		if result != nil {
			metadata.New = engine.MakeStepEventStateMetadata(result, pc.showSecrets)
			metadata.Res = metadata.New
		}
		events <- engine.NewEvent(engine.ResourceOutputsEventPayload{Metadata: metadata, Planning: preview})
		if step.Op != deploy.OpRead {
			events <- engine.NewEvent(engine.SummaryEventPayload{
				IsPreview:       preview,
				Duration:        time.Since(start),
				ResourceChanges: display.ResourceChanges{step.Op: 1},
			})
		}
	}

	events <- engine.NewCancelEvent()
	<-done
	close(events)

	return err
}

func (s displayedStep) urn() resource.URN {
	if s.New != nil {
		return s.New.URN
	}
	return s.Old.URN
}

func (s displayedStep) subject() string {
	id := ""
	if s.Old != nil && s.Old.ID != "" {
		id = string(s.Old.ID)
	}
	if s.New != nil && s.New.ID != "" {
		id = string(s.New.ID)
	}
	if id == "" {
		return string(s.urn().Type())
	}
	return fmt.Sprintf("%s %q", s.urn().Type(), id)
}

var singleErrorOccurred = regexp.MustCompile(`(?s)1 error occurred:\s*\n\s*\* (.*?)\s*$`)

func tidyProviderMessage(msg string, urn resource.URN, subject string) string {
	msg = strings.ReplaceAll(msg, string(urn), subject)
	return singleErrorOccurred.ReplaceAllString(msg, "$1")
}

func tidyProviderError(err error, urn resource.URN, subject string) error {
	if err == nil {
		return nil
	}
	msg := tidyProviderMessage(err.Error(), urn, subject)
	if msg == err.Error() {
		return err
	}
	return errors.New(msg)
}

func (s displayedStep) metadata(showSecrets bool) engine.StepEventMetadata {
	oldMeta := engine.MakeStepEventStateMetadata(s.Old, showSecrets)
	newMeta := engine.MakeStepEventStateMetadata(s.New, showSecrets)
	resMeta := newMeta
	if resMeta == nil {
		resMeta = oldMeta
	}
	return engine.StepEventMetadata{
		Op:           s.Op,
		URN:          s.urn(),
		Type:         s.urn().Type(),
		Old:          oldMeta,
		New:          newMeta,
		Res:          resMeta,
		Diffs:        s.Diffs,
		DetailedDiff: s.DetailedDiff,
	}
}

func operationState(urn resource.URN, id resource.ID, inputs, outputs resource.PropertyMap) *resource.State {
	return &resource.State{
		Type:    urn.Type(),
		URN:     urn,
		Custom:  true,
		ID:      id,
		Inputs:  inputs,
		Outputs: outputs,
	}
}
