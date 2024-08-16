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
	"github.com/spf13/viper"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

//nolint:lll
type ReplayEventsArgs struct {
	Preview              bool "argsShort:\"p\" argsUsage:\"Must be set for events from a `pulumi preview`.\""
	JSON                 bool `args:"json" argsShort:"j" argsUsage:"Serialize the preview diffs, operations, and overall output as JSON"`
	DisplayDiff          bool `args:"diff" argsUsage:"Display operation as a rich diff showing the overall change"`
	ShowConfig           bool `argsUsage:"Show configuration keys and variables"`
	ShowReplacementSteps bool `argsUsage:"Show detailed resource replacement creates and deletes instead of a single step"`
	ShowSames            bool `argsUsage:"Show resources that needn't be updated because they haven't changed, alongside those that do"`
	ShowReads            bool `argsUsage:"Show resources that are being read in, alongside those being managed directly in the stack"`
	SuppressOutputs      bool `argsUsage:"Suppress display of stack outputs (in case they contain sensitive values)"`
	SuppressProgress     bool `argsUsage:"Suppress display of periodic progress dots"`
	Debug                bool `argsShort:"d" argsUsage:"Print detailed debugging output during resource operations"`

	// TODO hack/pulumirc duration arguments
	Delay  time.Duration `argsUsage:"Delay display by the given duration. Useful for attaching a debugger."`
	Period time.Duration `argsUsage:"Delay each event by the given duration."`
}

func newReplayEventsCmd(
	v *viper.Viper,
	parentPulumiCmd *cobra.Command,
) *cobra.Command {
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
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, cmdArgs []string) error {
			args := UnmarshalArgs[ReplayEventsArgs](v, cmd)

			var action apitype.UpdateKind
			switch cmdArgs[0] {
			case "update":
				if args.Preview {
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
				return fmt.Errorf("unrecognized update kind '%v'", cmdArgs[0])
			}

			displayType := display.DisplayProgress
			if args.DisplayDiff {
				displayType = display.DisplayDiff
			}

			displayOpts := display.Options{
				Color:                cmdutil.GetGlobalColorization(),
				ShowConfig:           args.ShowConfig,
				ShowReplacementSteps: args.ShowReplacementSteps,
				ShowSameResources:    args.ShowSames,
				ShowReads:            args.ShowReads,
				SuppressOutputs:      args.SuppressOutputs,
				SuppressProgress:     args.SuppressProgress,
				IsInteractive:        cmdutil.Interactive(),
				Type:                 displayType,
				JSONDisplay:          args.JSON,
				Debug:                args.Debug,
			}

			events, err := loadEvents(cmdArgs[1])
			if err != nil {
				return fmt.Errorf("error reading events: %w", err)
			}

			eventChannel, doneChannel := make(chan engine.Event), make(chan bool)

			if args.Delay != 0 {
				time.Sleep(args.Delay)
			}

			go display.ShowEvents(
				"replay", action, tokens.MustParseStackName("replay"), "replay", "",
				eventChannel, doneChannel, displayOpts, args.Preview)

			for _, e := range events {
				eventChannel <- e
				if args.Period != 0 {
					time.Sleep(args.Period)
				}
			}
			<-doneChannel

			return nil
		}),
	}

	parentPulumiCmd.AddCommand(cmd)
	BindFlags[ReplayEventsArgs](v, cmd)

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
