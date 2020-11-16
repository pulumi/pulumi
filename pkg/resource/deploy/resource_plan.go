package deploy

import "github.com/pulumi/pulumi/sdk/v2/go/common/resource"

// A ResourcePlan represents the planned goal state and resource operations for a single resource. The operations are
// ordered.
type ResourcePlan struct {
	Goal *resource.Goal
	Ops  []StepOp
}

// Partial returns true if the plan is partial (i.e. its inputs properties contain unknown values).
func (rp *ResourcePlan) Partial() bool {
	return rp.Goal.Properties.ContainsUnknowns()
}

func (rp *ResourcePlan) completeInputs(programInputs resource.PropertyMap) resource.PropertyMap {
	// Find all unknown properties and replace them with their resolved values.
	plannedObject := resource.NewObjectProperty(rp.Goal.Properties.DeepCopy())
	programObject := resource.NewObjectProperty(programInputs)
	for _, path := range plannedObject.FindUnknowns() {
		if v, ok := path.Get(programObject); ok {
			path.Set(plannedObject, v)
		} else {
			path.Delete(plannedObject)
		}
	}
	return plannedObject.ObjectValue()
}
