// Copyright 2024, Pulumi Corporation.

package style

import (
	"io"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/muesli/termenv"
)

func some[T any](v T) *T {
	return &v
}

func Glamour(w io.Writer, options ...glamour.TermRendererOption) (*glamour.TermRenderer, error) {
	opts := []glamour.TermRendererOption{
		glamour.WithStyles(Default()),
		glamour.WithColorProfile(Profile(w)),
	}
	opts = append(opts, options...)
	return glamour.NewTermRenderer(opts...)
}

func Profile(w io.Writer) termenv.Profile {
	return termenv.NewOutput(w).Profile
}

func Default() ansi.StyleConfig {
	if termenv.HasDarkBackground() {
		return Dark
	}
	return Light
}
