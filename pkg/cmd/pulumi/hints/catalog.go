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

package hints

import "time"

// AgentSkills suggests Pulumi's agent skills to AI coding agents that create a new project. It may be shown before
// the first update, so that an agent sees it when it starts its first project.
var AgentSkills = Hint{
	ID:                "agent-skills",
	Cooldown:          30 * 24 * time.Hour,
	BeforeFirstUpdate: true,
	Message: "Pulumi agent skills help AI agents write and operate Pulumi programs. " +
		"Install them with `npx skills add pulumi/agent-skills --skill '*'`, " +
		"or in Claude Code with `/plugin marketplace add pulumi/agent-skills`.\n" +
		"Learn more: https://github.com/pulumi/agent-skills",
}

// DriftDetection suggests Pulumi Cloud's scheduled drift detection after a refresh finds drift.
var DriftDetection = Hint{
	ID:       "drift-detection",
	Cooldown: 7 * 24 * time.Hour,
	Message: "This refresh found drift. Pulumi Deployments can detect drift on a schedule and alert you, " +
		"or fix it automatically.\nLearn more: https://www.pulumi.com/docs/deployments/concepts/drift/",
	CloudOnly: true,
}
