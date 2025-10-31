package diag

import diag "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/diag"

// PrintDiagnostics prints the given diagnostics to the given diagnostic sink.
func PrintDiagnostics(sink diag.Sink, diagnostics hcl.Diagnostics) {
	diag.PrintDiagnostics(sink, diagnostics)
}

