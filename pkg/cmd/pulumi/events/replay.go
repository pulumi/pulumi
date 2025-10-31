package events

import events "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/events"

func NewReplayEventsCmd() *cobra.Command {
	return events.NewReplayEventsCmd()
}

