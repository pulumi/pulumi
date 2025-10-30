package display

import display "github.com/pulumi/pulumi/sdk/v3/pkg/backend/display"

// CopilotErrorSummaryMetadata contains metadata about a Copilot error summary.
type CopilotErrorSummaryMetadata = display.CopilotErrorSummaryMetadata

// ExplainFailureLink returns the link that will open Copilot and trigger it to explain the failure on the given
// permalink.
func ExplainFailureLink(permalink string) string {
	return display.ExplainFailureLink(permalink)
}

// RenderCopilotErrorSummary renders a Copilot error summary to the console.
func RenderCopilotErrorSummary(summary *CopilotErrorSummaryMetadata, err error, opts Options, permalink string) {
	display.RenderCopilotErrorSummary(summary, err, opts, permalink)
}

func PrintCopilotLink(out io.Writer, opts Options, permalink string) {
	display.PrintCopilotLink(out, opts, permalink)
}

// RenderCopilotThinking displays a "Thinking..." message.
func RenderCopilotThinking(opts Options) {
	display.RenderCopilotThinking(opts)
}

// FormatCopilotSummary formats a Copilot summary for display.
func FormatCopilotSummary(summary string, opts Options) string {
	return display.FormatCopilotSummary(summary, opts)
}

