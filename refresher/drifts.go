package refresher

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

func CalcDrift(step engine.StepEventMetadata) ([]map[string]interface{} , error){
	var merr *multierror.Error

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
				providerType := fmt.Sprintf("%T", val.New.Mappable())
				var providerValue string

				if providerType != "string" {
					providerJson, err := json.Marshal( val.New.Mappable())
					if err != nil {
						merr = multierror.Append(merr, err)
						continue
					}
					providerValue = string(providerJson)
				} else {
					providerValue = fmt.Sprintf("%v", val.New.Mappable())
				}

				iacType := fmt.Sprintf("%T", val.Old.Mappable())
				var iacValue string

				if iacType != "string" {
					iacJson, err := json.Marshal( val.Old.Mappable())
					if err != nil {
						merr = multierror.Append(merr, err)
						continue
					}
					iacValue = string(iacJson)
				} else {
					iacValue = fmt.Sprintf("%v", val.Old.Mappable())
				}

				drifts = append(drifts, map[string]interface{}{
					"keyName":       key,
					"providerValue": providerValue,
					"iacValue":      iacValue,
				})

			}
			for key, val := range outputDiff.Adds {
				drifts = append(drifts, map[string]interface{}{
					"keyName":       key,
					"providerValue": "undefined",
					"iacValue":      fmt.Sprintf("%v", val.Mappable()),
				})

			}
		}
	}
	return drifts, nil

}
