package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// PrettySpecs can be pretty-printed as human-readable strings for use in debugging output and error messages.
type PrettySpec = fuzzing.PrettySpec

// ColorFor accepts a string and hashes it to produce an RGB color. This is useful for making e.g. different URNs easy
// to identify by giving them unique colors when pretty-printing them.
func ColorFor(s string) *color.Color {
	return fuzzing.ColorFor(s)
}

// Colored generates a color from the given string and colors the string with it.
func Colored(t T) string {
	return fuzzing.Colored(t)
}

