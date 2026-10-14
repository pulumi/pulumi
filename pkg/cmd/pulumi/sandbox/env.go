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

package sandbox

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	sandboxrules "github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers/sandbox"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// emulatorSettings are the stack config keys that pick the region or project clients use with each emulator, the
// variables they set, and the value used when neither the stack nor the environment sets one.
var emulatorSettings = map[string]struct {
	configKey string
	envVars   []string
	fallback  string
}{
	"aws": {"aws:region", []string{"AWS_REGION", "AWS_DEFAULT_REGION"}, "us-east-1"},
	"gcp": {"gcp:project", []string{"GOOGLE_CLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT"}, "floci-local"},
}

func newSandboxEnvCmd() *cobra.Command {
	var shell, stackName string
	cmd := &cobra.Command{
		Use:   "env [aws|gcp]",
		Short: "Print environment variables that point cloud CLIs and SDKs at the local emulators",
		Long: "Print environment variables that point cloud CLIs and SDKs at the local emulators.\n" +
			"\n" +
			"Evaluate the output in your shell to use tools such as the AWS CLI against the same\n" +
			"emulators your sandbox stacks deploy to:\n" +
			"\n" +
			"    eval $(pulumi sandbox env)\n" +
			"    aws s3 ls\n" +
			"\n" +
			"The variables unset any credentials for the real clouds, such as AWS_PROFILE. When run\n" +
			"in a project, the region (aws:region) and project (gcp:project) come from the current\n" +
			"stack's config; otherwise from your environment, or the emulator's default.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"aws", "gcp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			emulators := Emulators
			if len(args) == 1 {
				emulator, ok := LookupEmulator(args[0])
				if !ok {
					return fmt.Errorf("unknown emulator %q; choose aws or gcp", args[0])
				}
				emulators = []Emulator{emulator}
			}
			format, ok := shellFormats[shell]
			if !ok {
				return fmt.Errorf("unsupported shell %q; choose bash, fish or powershell", shell)
			}

			stackConfig := loadStackConfig(cmd.Context(), stackName)
			vars, err := sandboxEnv(cmd.Context(), emulators, stackConfig, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			format.write(cmd.OutOrStdout(), vars)
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "bash", "The shell to print commands for: bash, fish or powershell")
	cmd.Flags().StringVarP(&stackName, "stack", "s", "",
		"The stack whose region and project to use (defaults to the current stack, if any)")
	return cmd
}

// loadStackConfig returns the config of the named or current stack, or nil when there's no project or stack.
func loadStackConfig(ctx context.Context, stackName string) config.Map {
	ws := pkgWorkspace.Instance
	proj, _, err := ws.ReadProject("")
	if err != nil {
		return nil
	}
	sink := diag.DefaultSink(io.Discard, io.Discard, diag.FormatOptions{Color: colors.Never})
	s, err := cmdStack.RequireStack(ctx, sink, ws, cmdBackend.DefaultLoginManager, stackName, cmdStack.LoadOnly,
		display.Options{Color: cmdutil.GetGlobalColorization()}, "")
	if err != nil || s == nil {
		return nil
	}
	ps, err := cmdStack.LoadProjectStack(ctx, sink, proj, s, "")
	if err != nil {
		return nil
	}
	return ps.Config
}

// sandboxEnv returns the variables for the given emulators. An empty value means the variable is unset. Emulators
// that aren't running are reported to warn, but their variables are still returned.
func sandboxEnv(
	ctx context.Context, emulators []Emulator, stackConfig config.Map, warn io.Writer,
) (map[string]string, error) {
	mode, err := sandboxrules.New(sandboxrules.Options{})
	if err != nil {
		return nil, err
	}

	vars := map[string]string{}
	for _, emulator := range emulators {
		floci, err := NewFloci(emulator, io.Discard)
		if err != nil {
			return nil, err
		}
		if !floci.Healthy(ctx) {
			fmt.Fprintf(warn, "warning: %s isn't running at %s; it starts the next time a sandbox stack needs it, "+
				"or start it with `floci %s start`\n", floci.DisplayName, floci.Endpoint, floci.CLICommand)
		}

		emulatorVars, err := mode.EmulatorEnv(emulator.Name, floci.Endpoint)
		if err != nil {
			return nil, err
		}
		maps.Copy(vars, emulatorVars)

		if setting, ok := emulatorSettings[emulator.Name]; ok {
			value := configValue(stackConfig, setting.configKey)
			for _, name := range setting.envVars {
				if value == "" {
					value = os.Getenv(name)
				}
			}
			if value == "" {
				value = setting.fallback
			}
			for _, name := range setting.envVars {
				vars[name] = value
			}
		}
	}
	return vars, nil
}

func configValue(cfg config.Map, key string) string {
	if cfg == nil {
		return ""
	}
	v, ok, err := cfg.Get(config.MustParseKey(key), false)
	if err != nil || !ok || v.Secure() {
		return ""
	}
	value, err := v.Value(config.NewBlindingDecrypter())
	if err != nil {
		return ""
	}
	return value
}

type shellFormat struct {
	set, unset func(name, value string) string
	comment    string
}

var shellFormats = map[string]shellFormat{
	"bash": {
		set:     func(name, value string) string { return fmt.Sprintf("export %s=%s", name, quotePOSIX(value)) },
		unset:   func(name, _ string) string { return "unset " + name },
		comment: "# Run: eval $(pulumi sandbox env)",
	},
	"fish": {
		set:     func(name, value string) string { return fmt.Sprintf("set -gx %s %s", name, quoteFish(value)) },
		unset:   func(name, _ string) string { return "set -e " + name },
		comment: "# Run: pulumi sandbox env --shell fish | source",
	},
	"powershell": {
		set: func(name, value string) string {
			return fmt.Sprintf("$env:%s = '%s'", name, strings.ReplaceAll(value, "'", "''"))
		},
		unset: func(name, _ string) string {
			return fmt.Sprintf("Remove-Item Env:%s -ErrorAction SilentlyContinue", name)
		},
		comment: "# Run: pulumi sandbox env --shell powershell | Invoke-Expression",
	},
}

func (f shellFormat) write(w io.Writer, vars map[string]string) {
	names := make([]string, 0, len(vars))
	for name := range vars {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if value := vars[name]; value == "" {
			fmt.Fprintln(w, f.unset(name, value))
		} else {
			fmt.Fprintln(w, f.set(name, value))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, f.comment)
}

func quotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func quoteFish(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, "'", `\'`).Replace(s) + "'"
}
