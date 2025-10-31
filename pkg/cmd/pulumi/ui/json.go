package ui

import ui "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/ui"

// MakeJSONString turns the given value into a JSON string. If multiline is true, the JSON will be formatted with
// indentation and a trailing newline.
func MakeJSONString(v any, multiline bool) (string, error) {
	return ui.MakeJSONString(v, multiline)
}

// PrintJSON simply prints out some object, formatted as JSON, using standard indentation.
func PrintJSON(v any) error {
	return ui.PrintJSON(v)
}

// FprintJSON simply prints out some object, formatted as JSON, using standard indentation.
func FprintJSON(w io.Writer, v any) error {
	return ui.FprintJSON(w, v)
}

