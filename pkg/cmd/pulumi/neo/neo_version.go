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

// checkNeoMinCLIVersion enforces the service-advertised minimum CLI version for `pulumi
// neo`. The service surfaces a Neo capability config in /api/capabilities when it has
// decided it cannot speak the neo protocol with older CLIs; we read that here and refuse
// to create a task if the local CLI is too old.
//
// Defensive parsing: if either version is missing or unparseable (e.g. a dev build with
// an empty version.Version, or a service that sent garbage), we let the operation proceed
// rather than blocking on something we can't reason about. The service still gets to
// reject the request itself if it really has to.
func checkNeoMinCLIVersion(caps apitype.Capabilities, currentVersion string) error {
	if caps.Neo == nil || caps.Neo.MinCLIVersion == "" {
		return nil
	}
	required, err := semver.ParseTolerant(caps.Neo.MinCLIVersion)
	if err != nil {
		return nil
	}
	current, err := semver.ParseTolerant(currentVersion)
	if err != nil {
		return nil
	}
	if current.GTE(required) {
		return nil
	}
	return fmt.Errorf(
		"pulumi neo requires CLI version %s or newer (you have %s); "+
			"please upgrade to use this feature: https://www.pulumi.com/docs/install/",
		required, current)
}
