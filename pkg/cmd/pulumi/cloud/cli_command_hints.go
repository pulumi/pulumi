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

package cloud

import (
	"fmt"
	"io"
)

var cliCommands = map[string]string{
	"ListPolicyIssues":                  "pulumi policy issue list",
	"GetPolicyIssue":                    "pulumi policy issue get",
	"GetPolicyComplianceResults":        "pulumi policy compliance list",
	"ListPolicyGroups":                  "pulumi policy group list",
	"GetPolicyGroup":                    "pulumi policy group get",
	"NewPolicyGroup":                    "pulumi policy group new",
	"BatchUpdatePolicyGroup":            "pulumi policy group edit",
	"DeletePolicyGroup":                 "pulumi policy group remove",
	"ListPolicyPacks_orgs":              "pulumi policy list",
	"GetPolicyPack":                     "pulumi policy get",
	"DeletePolicyPack_orgs_policypacks": "pulumi policy remove",
	"DeletePolicyPackVersion":           "pulumi policy remove",
	"CreatePolicyPack":                  "pulumi policy publish",
	"CompletePolicyPack":                "pulumi policy publish",
}

func writeCLICommandHint(w io.Writer, operationID string) {
	command, ok := cliCommands[operationID]
	if !ok {
		return
	}
	fmt.Fprintf(w, "%s has a dedicated command; run `%s` instead of `pulumi api`.\n", operationID, command)
}
