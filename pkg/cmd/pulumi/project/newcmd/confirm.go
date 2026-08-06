// Copyright 2026, Pulumi Corporation.
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

package newcmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

type field struct{ label, value string }

const (
	confirmYes    = "Yes, create the project"
	confirmChange = "Change these values"
)

// errConfirmationInterrupted is the friendly error surfaced when Ctrl-C is pressed at the guided
// confirmation select, mirroring declinedToChoose's mapping of the same interrupt at template
// selection (see errNoTemplateSelected in template.go).
var errConfirmationInterrupted = errors.New("no project created; please use `pulumi new` to start again")

// printFields renders aligned label/value rows: label column padded to the widest
// label, a colon, two spaces, then the value.
func printFields(w io.Writer, color colors.Colorization, indent string, fields []field) {
	width := 0
	for _, f := range fields {
		width = max(width, len(f.label))
	}
	for _, f := range fields {
		label := f.label + ":" + strings.Repeat(" ", width-len(f.label))
		fmt.Fprintln(w, indent+color.Colorize(colors.SpecSubHeadline+label+colors.Reset)+"  "+f.value)
	}
}

// confirmDefaults renders the settled values and asks whether to accept them.
// True means create the project as shown; false means fall through to the prompts.
func confirmDefaults(fields, configRows []field, opts display.Options, sel selectFunc) (bool, error) {
	w := opts.StdoutOrDefault()
	fmt.Fprintln(w)
	printFields(w, opts.Color, "", fields)
	if len(configRows) > 0 {
		fmt.Fprintln(w, opts.Color.Colorize(colors.SpecSubHeadline+"Config:"+colors.Reset))
		printFields(w, opts.Color, "  ", configRows)
	}
	i, err := sel("Do these look good?", []string{confirmYes, confirmChange}, opts)
	return i == 0, err
}
