// Copyright 2025, Pulumi Corporation.
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

package cli

import (
	"fmt"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
)

const settingDeletionProtected settingName = "deletion-protected"

type DeletionProtectedSetting struct{}

func (s *DeletionProtectedSetting) KebabName() string {
	return "deletion-protected"
}

func (s *DeletionProtectedSetting) HelpText() string {
	return "Enable or disable deletion protection"
}

// ValidateValue accepts only "true" and "false" strings, unlike the general env {get,set} commands
// which parse YAML and accept broader boolean values like "yes", "no", "on", "off", etc.
// This restriction maintains compatibility while limiting the accepted subset to a well-defined
// interface that can be reliably parsed and validated.
func (s *DeletionProtectedSetting) ValidateValue(raw string) (bool, error) {
	if raw != "true" && raw != "false" {
		return false, fmt.Errorf("invalid value for %s: %s (expected true or false)", s.KebabName(), raw)
	}
	return raw == "true", nil
}

func (s *DeletionProtectedSetting) GetValue(settings *client.EnvironmentSettings) bool {
	return settings.DeletionProtected
}

func (s *DeletionProtectedSetting) SetValue(req *client.PatchEnvironmentSettingsRequest, value bool) {
	req.DeletionProtected = &value
}
