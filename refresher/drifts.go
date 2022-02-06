package refresher

import (
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func CalcDrift(step engine.StepEventMetadata) []map[string]interface{} {

	var drifts []map[string]interface{}
	var outputDiff *resource.ObjectDiff
	var outs resource.PropertyMap
	if step.New == nil || step.New.Outputs == nil {
		outs = make(resource.PropertyMap)
	} else {
		outs = step.New.Outputs
	}

	if step.Old != nil && step.Old.Outputs != nil {
		outputDiff = step.Old.Outputs.Diff(outs, resource.IsInternalPropertyKey)

		if outputDiff != nil {
			for key, val := range outputDiff.Updates {
				if val.Array == nil && val.Object == nil {
					drifts = append(drifts, map[string]interface{}{
						"keyName":       key,
						"providerValue": val.New.V,
						"iacValue":      val.Old.V,
					})
				}
			}
		}
	}
	return drifts

}
