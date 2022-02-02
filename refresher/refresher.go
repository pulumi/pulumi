package refresher

import (
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func (c *Client) TempRunner(b backend.Backend, stacks []backend.StackSummary) () {
	imports := []deploy.Import{}

	for _, be := range stacks {
		s, err := b.GetStack(c.Ctx, be.Name())

		if err != nil{
			fmt.Errorf("could not get stack. error=%w", err)
		}
		proj := workspace.Project{Name: "firefly", Runtime: workspace.NewProjectRuntimeInfo("go", nil)}

		up := backend.UpdateMetadata{
			Message:     "Firefly's Scan",
			Environment: nil,
		}
		updateOpts := backend.UpdateOperation{
			Proj:               &proj,
			Root:               "",
			Imports:            imports,
			M:                  &up,
			Opts:               c.Opts,
			SecretsManager:     nil,
			StackConfiguration: backend.StackConfiguration{},
			Scopes:             backend.CancellationScopeSource(cancellationScopeSource(0)),
		}
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

		a, result := s.Refresh(c.Ctx, updateOpts)
		_ = a
		_ = result

		fmt.Println("Done!!!")
	}
}

