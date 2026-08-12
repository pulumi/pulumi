// Copyright 2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package style

import (
	"io"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/muesli/termenv"
)

func ptr[T any](v T) *T {
	return &v
}

func Glamour(w io.Writer, options ...glamour.TermRendererOption) (*glamour.TermRenderer, error) {
	opts := make([]glamour.TermRendererOption, 0, 2+len(options))
	opts = append(opts, glamour.WithStyles(Default()), glamour.WithColorProfile(Profile(w)))
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
