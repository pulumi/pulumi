package format

import format "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/hcl2/model/format"

// ExpressionGenerator is an interface that can be implemented in order to generate code for semantically-analyzed HCL2
// expressions using a Formatter.
type ExpressionGenerator = format.ExpressionGenerator

// Formatter is a convenience type that implements a number of common utilities used to emit source code. It implements
// the io.Writer interface.
type Formatter = format.Formatter

// NewFormatter creates a new emitter targeting the given io.Writer that will use the given ExpressionGenerator when
// generating code.
func NewFormatter(g ExpressionGenerator) *Formatter {
	return format.NewFormatter(g)
}

