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

package policyx

import pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

type EnforcementLevel int32

const (
	// Displayed to users, but does not block deployment.
	EnforcementLevelAdvisory EnforcementLevel = EnforcementLevel(pulumirpc.EnforcementLevel_ADVISORY)
	// Stops deployment, cannot be overridden.
	EnforcementLevelMandatory EnforcementLevel = EnforcementLevel(pulumirpc.EnforcementLevel_MANDATORY)
	// Disabled policies do not run during a deployment.
	EnforcementLevelDisabled EnforcementLevel = EnforcementLevel(pulumirpc.EnforcementLevel_DISABLED)
	// Remediated policies actually fixes problems instead of issuing diagnostics
	EnforcementLevelRemediate EnforcementLevel = EnforcementLevel(pulumirpc.EnforcementLevel_REMEDIATE)
)

// ConfigSchema represents the configuration schema for a policy.
type ConfigSchema struct {
	// The policy's configuration properties, this should be a map of property names to json schema
	// definitions.
	Properties map[string]map[string]any `json:"properties"`

	// The configuration properties that are required.
	Required []string `json:"required"`
}
