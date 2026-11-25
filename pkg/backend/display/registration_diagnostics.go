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

package display

import (
	"strings"
	"unicode"

	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func appendDiagnosticSourceContext(message string, context *diag.SourceContext) string {
	if context == nil || context.Rendered == "" {
		return message
	}
	message = strings.TrimRightFunc(message, unicode.IsSpace)
	// The diagnostic sink adds a reset after the message, which may itself end in newlines.
	if text, hasReset := strings.CutSuffix(message, colors.Reset); hasReset {
		message = strings.TrimRightFunc(text, unicode.IsSpace) + colors.Reset
	}
	return message + "\n\n" + context.Rendered + "\n\n"
}
