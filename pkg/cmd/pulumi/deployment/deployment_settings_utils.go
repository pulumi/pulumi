// Copyright 2016, Pulumi Corporation.
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

package deployment

import (
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
)

func askForConfirmation(prompt string, color colors.Colorization, defaultValue bool, yes bool) bool {
	def := optNo
	if defaultValue {
		def = optYes
	}
	options := []string{optYes, optNo}
	response := ui.PromptUserSkippable(yes, prompt, options, def, color)
	return response == optYes
}
