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
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/muesli/termenv"
)

type Stylist struct {
	profile termenv.Profile
}

func NewStylist(profile termenv.Profile) *Stylist {
	return &Stylist{profile: profile}
}

func (st *Stylist) Sprintf(rules ansi.StylePrimitive, s string, args ...any) string {
	out := st.profile.String(fmt.Sprintf(s, args...))

	if rules.Upper != nil && *rules.Upper {
		out = termenv.String(strings.ToUpper(s))
	}
	if rules.Lower != nil && *rules.Lower {
		out = termenv.String(strings.ToLower(s))
	}
	if rules.Color != nil {
		out = out.Foreground(st.profile.Color(*rules.Color))
	}
	if rules.BackgroundColor != nil {
		out = out.Background(st.profile.Color(*rules.BackgroundColor))
	}
	if rules.Underline != nil && *rules.Underline {
		out = out.Underline()
	}
	if rules.Bold != nil && *rules.Bold {
		out = out.Bold()
	}
	if rules.Italic != nil && *rules.Italic {
		out = out.Italic()
	}
	if rules.CrossedOut != nil && *rules.CrossedOut {
		out = out.CrossOut()
	}
	if rules.Overlined != nil && *rules.Overlined {
		out = out.Overline()
	}
	if rules.Inverse != nil && *rules.Inverse {
		out = out.Reverse()
	}
	if rules.Blink != nil && *rules.Blink {
		out = out.Blink()
	}

	return out.String()
}

func (st *Stylist) Fprintf(w io.Writer, rules ansi.StylePrimitive, s string, args ...any) (int, error) {
	return fmt.Fprint(w, st.Sprintf(rules, s, args...))
}
