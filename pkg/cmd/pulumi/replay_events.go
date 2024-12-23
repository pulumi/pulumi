// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newReplayEventsCmd() *cobra.Command {
	var preview bool

	var jsonDisplay bool
	var diffDisplay bool
	var showConfig bool
	var showReplacementSteps bool
	var showSames bool
	var showReads bool
	var suppressOutputs bool
	var suppressProgress bool
	var debug bool

	var delay time.Duration
	var period time.Duration

	cmd := &cobra.Command{
		Use:   "replay-events [kind] [events-file]",
		Short: "Replay events from a prior update, refresh, or destroy",
		Long: "Replay events from a prior update, refresh, or destroy.\n" +
			"\n" +
			"This command is used to replay events emitted by a prior\n" +
			"invocation of the Pulumi CLI (e.g. `pulumi up --event-log [file]`).\n" +
			"\n" +
			"This command loads events from the indicated file and renders them\n" +
			"using either the progress view or the diff view.\n",
		Args:   cmdutil.ExactArgs(2),
		Hidden: !env.DebugCommands.Value(),
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			var action apitype.UpdateKind
			switch args[0] {
			case "update":
				if preview {
					action = apitype.PreviewUpdate
				} else {
					action = apitype.UpdateUpdate
				}
			case "refresh":
				action = apitype.RefreshUpdate
			case "destroy":
				action = apitype.DestroyUpdate
			case "import":
				action = apitype.ResourceImportUpdate
			default:
				return fmt.Errorf("unrecognized update kind '%v'", args[0])
			}

			displayType := display.DisplayProgress
			if diffDisplay {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           showConfig,
				ShowReplacementSteps: showReplacementSteps,
				ShowSameResources:    showSames,
				ShowReads:            showReads,
				SuppressOutputs:      suppressOutputs,
				SuppressProgress:     suppressProgress,
				IsInteractive:        cmdutil.Interactive(),
				Type:                 displayType,
				JSONDisplay:          jsonDisplay,
				Debug:                debug,
			}

			events, err := loadEvents(args[1])
			if err != nil {
				return fmt.Errorf("error reading events: %w", err)
			}

			eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

			if delay != 0 {
				time.Sleep(delay)
			}

			go display.ShowEvents(
				"replay", action, tokens.MustParseStackName("replay"), "replay", "",
				eventChannel, doneChannel, displayOpts, preview)

			for _, e := range events {
				eventChannel <- e
				if period != 0 {
					time.Sleep(period)
				}
			}
			<-doneChannel

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&preview, "preview", "p", false,
		"Must be set for events from a `pulumi preview`.")

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&jsonDisplay, "json", "j", false,
		"Serialize the preview diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().BoolVar(
		&showConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&showReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&showSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&showReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")
	cmd.PersistentFlags().BoolVar(
		&suppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVar(
		&suppressProgress, "suppress-progress", false,
		"Suppress display of periodic progress dots")

	cmd.PersistentFlags().DurationVar(&delay, "delay", time.Duration(0),
		"Delay display by the given duration. Useful for attaching a debugger.")
	cmd.PersistentFlags().DurationVar(&period, "period", time.Duration(0),
		"Delay each event by the given duration.")

	return cmd
}

func loadEvents(path string) ([]engine.Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening '%v': %w", path, err)
	}
	defer contract.IgnoreClose(f)

	var events []engine.Event
	dec := json.NewDecoder(f)
	for {
		var jsonEvent apitype.EngineEvent
		if err = dec.Decode(&jsonEvent); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decoding event: %w", err)
		}

		event, err := display.ConvertJSONEvent(jsonEvent)
		if err != nil {
			return nil, fmt.Errorf("decoding event: %w", err)
		}
		events = append(events, event)
	}

	// If there are no events or if the event stream does not terminate with a cancel event,
	// synthesize one here.
	if len(events) == 0 || events[len(events)-1].Type != engine.CancelEvent {
		events = append(events, engine.NewCancelEvent())
	}

	return events, nil
}
