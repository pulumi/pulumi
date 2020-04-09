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

package ciutil

import (
	"os"
)

// DetectVars detects and returns the CI variables for the current environment.
// Not all fields of the `Vars` struct are applicable to every CI system,
// and may be left blank.
func DetectVars() Vars {
	if os.Getenv("PULUMI_DISABLE_CI_DETECTION") != "" {
		return Vars{Name: ""}
	}

	var v Vars
	system := detectSystem()
	if system == nil {
		return v
	}
	// Detect the vars for the respective CI system and
	v = system.DetectVars()

	return v
}
