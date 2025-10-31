package ui

import ui "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/ui"

// Quick and dirty utility function for printing to writers that we know will never fail.
func Fprintf(writer io.Writer, msg string, args ...any) {
	ui.Fprintf(writer, msg, args...)
}

