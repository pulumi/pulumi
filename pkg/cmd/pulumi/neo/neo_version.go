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

package neo

import (
	"fmt"

	"github.com/blang/semver"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// neoUpgradeMessage returns a user-facing upgrade message when the build is
// older than the service-advertised minimum, or "" otherwise. Missing or
// unparseable versions fall through silently so a dev build (empty
// version.Version) or a garbled service response can't lock users out.
func neoUpgradeMessage(caps apitype.Capabilities, currentVersion string) string {
	if caps.NeoCLIMode == nil || caps.NeoCLIMode.MinCLIVersion == "" {
		return ""
	}
	required, err := semver.ParseTolerant(caps.NeoCLIMode.MinCLIVersion)
	if err != nil {
		return ""
	}
	current, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return ""
	}
	if current.GTE(required) {
		return ""
	}
	return fmt.Sprintf(
		"`pulumi neo` requires Pulumi CLI %s or newer; you are running %s.\n"+
			"To upgrade, see https://www.pulumi.com/docs/install/",
		required, current)
}
