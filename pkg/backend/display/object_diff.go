package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

func PrintObject(b *bytes.Buffer, props resource.PropertyMap, planning bool, indent int, op display.StepOp, prefix bool, truncateOutput bool, debug bool, showSecrets bool) {
	display.PrintObject(b, props, planning, indent, op, prefix, truncateOutput, debug, showSecrets)
}

func PrintResourceReference(b *bytes.Buffer, resRef resource.ResourceReference, planning bool, indent int, op display.StepOp, prefix bool, debug bool) {
	display.PrintResourceReference(b, resRef, planning, indent, op, prefix, debug)
}

func PrintObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff, include []resource.PropertyKey, planning bool, indent int, summary bool, truncateOutput bool, debug bool, showSecrets bool, hidden []resource.PropertyPath) {
	display.PrintObjectDiff(b, diff, include, planning, indent, summary, truncateOutput, debug, showSecrets, hidden)
}

