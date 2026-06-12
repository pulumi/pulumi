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

package org

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/pulumi/pulumi/pkg/v3/util/outputflag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// roleSingleRender formats a single role for the user. The action describes
// the verb the command performed (e.g. "Created", "Updated") and is shown in
// the default human-readable output.
type roleSingleRender func(w io.Writer, orgName, action string, role apitype.Role) error

func defaultRoleSingleOutput() outputflag.OutputFlag[roleSingleRender] {
	return outputflag.OutputFlag[roleSingleRender]{
		RenderForTerminal: renderRoleSingleText,
		RenderJSON:        renderRoleSingleJSON,
	}
}

// roleSingleEnvelope is the JSON shape emitted by single-role mutating commands
// (`new`, `edit`) when --output=json is set.
type roleSingleEnvelope struct {
	Organization string   `json:"organization"`
	Action       string   `json:"action"`
	Role         roleJSON `json:"role"`
}

func renderRoleSingleJSON(w io.Writer, orgName, action string, role apitype.Role) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(roleSingleEnvelope{
		Organization: orgName,
		Action:       action,
		Role:         toRoleJSON(role),
	})
}

func renderRoleSingleText(w io.Writer, _ /*orgName*/, action string, role apitype.Role) error {
	label := role.Name
	if label == "" {
		label = role.ID
	}
	fmt.Fprintf(w, "%s role %q (id: %s)\n", action, label, role.ID)
	if role.Description != "" {
		fmt.Fprintf(w, "  description: %s\n", role.Description)
	}
	if role.UXPurpose != "" {
		fmt.Fprintf(w, "  purpose: %s\n", role.UXPurpose)
	}
	fmt.Fprintf(w, "  version: %d\n", role.Version)
	return nil
}
