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

package updatecheck

import (
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// GetCLIMetadata returns a map of metadata about the given CLI command.
func GetCLIMetadata(cmd *cobra.Command, environ []string, args []string) map[string]string {
	if cmd == nil {
		return nil
	}

	command := cmd.CommandPath()

	var flags strings.Builder
	i := 0
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if i > 0 {
				flags.WriteRune(' ')
			}
			flags.WriteString("--" + f.Name)
			i++
		}
	})

	if command == "pulumi plugin run" {
		if len(args) > 0 {
			positionals := positionalArgs(cmd, args)
			if len(positionals) > 0 {
				command += " " + positionals[0]
			}
		}
	}

	if command == "pulumi do" {
		positionals := positionalArgs(cmd, args)
		// Include the resource/function token
		if len(positionals) > 0 {
			command += " " + positionals[0]
		}
		if len(positionals) > 1 {
			switch positionals[1] {
			case "create", "read", "patch", "delete", "list":
				command += " " + positionals[1]
			}
		}
	}

	metadata := map[string]string{
		"Command":     command,
		"Flags":       flags.String(),
		"Environment": pulumiEnvNames(environ),
	}

	return metadata
}

// pulumiEnvNames returns the names, not the values, of the PULUMI_-prefixed variables in environ.
func pulumiEnvNames(environ []string) string {
	names := []string{}
	for _, e := range environ {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "PULUMI_") {
			names = append(names, parts[0])
		}
	}
	return strings.Join(names, " ")
}

// positionalArgs extracts positional arguments from a raw argument list,
// filtering out flags. This is used for commands with DisableFlagParsing
// where args contains everything including flags. Known flags from the
// command are registered so pflag can skip over their values correctly;
// unknown flags are also tolerated.
func positionalArgs(cmd *cobra.Command, args []string) []string {
	fs := pflag.NewFlagSet("telemetry", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	// Register the command's own flags so pflag can correctly skip their
	// values (e.g. --package "foo" should not treat "foo" as positional).
	// Visit both Flags() and PersistentFlags() to cover all defined flags;
	// cobra merges them during Execute(), but we may run before that merge.
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		fs.AddFlag(f)
	})
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if fs.Lookup(f.Name) == nil {
			fs.AddFlag(f)
		}
	})
	// Silence any output from parsing errors.
	fs.SetOutput(io.Discard)
	_ = fs.Parse(args)
	return fs.Args()
}
