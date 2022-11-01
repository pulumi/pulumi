// Copyright 2016-2018, Pulumi Corporation.
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

package display

import (
	"strings"
	"unicode/utf8"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func columnHeader(msg string) string {
	return colors.Underline + colors.BrightBlue + msg + colors.Reset
}

func messagePadding(message string, maxLength, extraPadding int) string {
	extraWhitespace := maxLength - utf8.RuneCountInString(message)
	contract.Assertf(extraWhitespace >= 0, "Neg whitespace. %v %s", maxLength, message)

	// Place two spaces between all columns (except after the first column).  The first
	// column already has a ": " so it doesn't need the extra space.
	extraWhitespace += extraPadding

	return strings.Repeat(" ", extraWhitespace)
}

// Gets the padding necessary to prepend to a column in order to keep it aligned in the terminal.
func columnPadding(uncolorizedColumns []string, columnIndex int, maxColumnLengths []int) string {
	extraWhitespace := " "
	if columnIndex >= 0 && len(maxColumnLengths) > 0 {
		column := uncolorizedColumns[columnIndex]
		maxLength := maxColumnLengths[columnIndex]
		extraWhitespace = messagePadding(column, maxLength, 2)
	}
	return extraWhitespace
}

// Gets the fully padded message to be shown.  The message will always include the ID of the
// status, then some amount of optional padding, then some amount of msgWithColors, then the
// suffix.  Importantly, if there isn't enough room to display all of that on the terminal, then
// the msg will be truncated to try to make it fit.
func renderRow(colorizedColumns, uncolorizedColumns []string, maxColumnLengths []int) string {
	var row strings.Builder
	for i := 0; i < len(colorizedColumns); i++ {
		row.WriteString(columnPadding(uncolorizedColumns, i-1, maxColumnLengths))
		row.WriteString(colorizedColumns[i])
	}
	return row.String()
}
