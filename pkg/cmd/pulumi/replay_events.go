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
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type ReplayEventsConfig struct {
	PulumiConfig

	Preview              bool
	JSON                 bool
	DisplayDiff          bool
	ShowConfig           bool
	ShowReplacementSteps bool
	ShowSames            bool
	ShowReads            bool
	SuppressOutputs      bool
	SuppressProgress     bool
	Debug                bool
	Delay                time.Duration
	Period               time.Duration
}

func newReplayEventsCmd() *cobra.Command {
	var config ReplayEventsConfig

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
		Hidden: !hasDebugCommands(),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			var action apitype.UpdateKind
			switch args[0] {
			case "update":
				if config.Preview {
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
			if config.DisplayDiff {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           config.ShowConfig,
				ShowReplacementSteps: config.ShowReplacementSteps,
				ShowSameResources:    config.ShowSames,
				ShowReads:            config.ShowReads,
				SuppressOutputs:      config.SuppressOutputs,
				SuppressProgress:     config.SuppressProgress,
				IsInteractive:        cmdutil.Interactive(),
				Type:                 displayType,
				JSONDisplay:          config.JSON,
				Debug:                config.Debug,
			}

			events, err := loadEvents(args[1])
			if err != nil {
				return fmt.Errorf("error reading events: %w", err)
			}

			eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

			if config.Delay != 0 {
				time.Sleep(config.Delay)
			}

			go display.ShowEvents(
				"replay", action, tokens.MustParseStackName("replay"), "replay", "",
				eventChannel, doneChannel, displayOpts, config.Preview)

			for _, e := range events {
				eventChannel <- e
				if config.Period != 0 {
					time.Sleep(config.Period)
				}
			}
			<-doneChannel

			return nil
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&config.Preview, "preview", "p", false,
		"Must be set for events from a `pulumi preview`.")

	cmd.PersistentFlags().BoolVarP(
		&config.Debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().BoolVar(
		&config.DisplayDiff, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.Flags().BoolVarP(
		&config.JSON, "json", "j", false,
		"Serialize the preview diffs, operations, and overall output as JSON")
	cmd.PersistentFlags().BoolVar(
		&config.ShowConfig, "show-config", false,
		"Show configuration keys and variables")
	cmd.PersistentFlags().BoolVar(
		&config.ShowReplacementSteps, "show-replacement-steps", false,
		"Show detailed resource replacement creates and deletes instead of a single step")
	cmd.PersistentFlags().BoolVar(
		&config.ShowSames, "show-sames", false,
		"Show resources that needn't be updated because they haven't changed, alongside those that do")
	cmd.PersistentFlags().BoolVar(
		&config.ShowReads, "show-reads", false,
		"Show resources that are being read in, alongside those being managed directly in the stack")
	cmd.PersistentFlags().BoolVar(
		&config.SuppressOutputs, "suppress-outputs", false,
		"Suppress display of stack outputs (in case they contain sensitive values)")
	cmd.PersistentFlags().BoolVar(
		&config.SuppressProgress, "suppress-progress", false,
		"Suppress display of periodic progress dots")

	cmd.PersistentFlags().DurationVar(&config.Delay, "delay", time.Duration(0),
		"Delay display by the given duration. Useful for attaching a debugger.")
	cmd.PersistentFlags().DurationVar(&config.Period, "period", time.Duration(0),
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
