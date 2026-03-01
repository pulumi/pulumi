// Copyright 2016-2026, Pulumi Corporation.
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

package skills

import (
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
)

func NewSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage Pulumi agent skills",
		Long: "Manage Pulumi agent skills for AI assistants.\n" +
			"\n" +
			"Agent skills are packages of procedural knowledge that teach AI assistants\n" +
			"how to write Pulumi code, migrate from other tools, and follow best practices.\n" +
			"Skills work across platforms including Claude Code, Cursor, VS Code, and more.\n" +
			"\n" +
			"The skills family of commands provides a way to install and manage these skills\n" +
			"in your project's AI assistant configuration directories.",
	}

	constrictor.AttachArguments(cmd, constrictor.NoArgs)
	cmd.AddCommand(newSyncCmd())

	return cmd
}
