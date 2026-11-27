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

package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpCommandRef matches a command reference such as `pulumi stack tag list` in help text. The match stops at the
// first word that can't be a command name, such as a flag or a placeholder.
var helpCommandRef = regexp.MustCompile("`pulumi((?: [a-z][a-z0-9-]*)+)")

// TestHelpTextCommandReferencesExist checks that every `pulumi ...` command mentioned in help text exists, so that
// help text never tells people to run a command that fails with "unknown command".
func TestHelpTextCommandReferencesExist(t *testing.T) {
	t.Parallel()

	root, cleanup := NewPulumiCmd()
	defer cleanup()

	// unknownWord returns the first word of a command reference that isn't a command. Words after a command that
	// has no subcommands are arguments, so they aren't checked.
	unknownWord := func(words []string) string {
		cur := root
		for _, w := range words {
			if !cur.HasSubCommands() {
				return ""
			}
			var next *cobra.Command
			for _, sub := range cur.Commands() {
				if sub.Name() == w || sub.HasAlias(w) {
					next = sub
					break
				}
			}
			if next == nil {
				return w
			}
			cur = next
		}
		return ""
	}

	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, text := range []string{cmd.Short, cmd.Long} {
			for _, m := range helpCommandRef.FindAllStringSubmatch(text, -1) {
				if w := unknownWord(strings.Fields(m[1])); w != "" {
					t.Errorf("help for %q refers to `pulumi%s`, but %q is not a command", cmd.CommandPath(), m[1], w)
				}
			}
		}
		for _, sub := range cmd.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
