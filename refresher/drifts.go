package refresher

import (
	"fmt"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func calcDrift(event engine.Event, step engine.StepEventMetadata){
	var outputDiff *resource.ObjectDiff
	if event.Type == engine.ResourcePreEvent {
		step = event.Payload().(engine.ResourcePreEventPayload).Metadata

	} else if event.Type == engine.ResourceOutputsEvent {
		step = event.Payload().(engine.ResourceOutputsEventPayload).Metadata

	}
	var outs resource.PropertyMap
	if step.New == nil || step.New.Outputs == nil {
		outs = make(resource.PropertyMap)
	} else {
		outs = step.New.Outputs
	}

	if step.Old != nil && step.Old.Outputs != nil {
		outputDiff = step.Old.Outputs.Diff(outs, resource.IsInternalPropertyKey)

	if outputDiff.Updates != nil {
		for key, val := range  outputDiff.Updates {
			fmt.Println(key)
			fmt.Println(val)
		}

	}


	}
}