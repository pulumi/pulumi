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

package cmd

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
)

func newRunCmd() *cobra.Command {
	var analyzers []string
	var color colorFlag
	var configVars []string
	var debug bool
	var diffDisplay bool
	var message string
	var nonInteractive bool
	var parallel int
	var secretVars []string
	var stack string

	var cmd = &cobra.Command{
		Use:   "run",
		Short: "Run a Pulumi program anonymously",
		Long: "Run a Pulumi program anonymously.\n" +
			"\n" +
			"This command will stand up a new stack, deploy resources into it, and block the CLI while\n" +
			"the resulting program executes.  Any logs emitted while be printed to the console.  When you\n" +
			"are done, hit ^C, and the program will halt and the stack will be torn down and deleted.",
		Args: cmdutil.NoArgs,
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			// Grab the options we'll use for the update, mainly just whether to do interactive displays.
			interactive := isInteractive(nonInteractive)
			opts, err := updateFlagsToOptions(interactive, true, true)
			if err != nil {
				return err
			}
			opts.Engine = engine.UpdateOptions{
				Analyzers: analyzers,
				Parallel:  parallel,
				Debug:     debug,
			}
			opts.Display = backend.DisplayOptions{
				Color:         color.Colorization(),
				IsInteractive: interactive,
				DiffDisplay:   diffDisplay,
				Debug:         debug,
			}

			// Ensure there's a project for us to deploy.
			proj, root, err := readProject()
			if err != nil {
				return err
			}

			// Create a new stack.  If the stack name wasn't given, we will pick a random one.
			b, s, err := createTempStack(proj, stack)
			if err != nil {
				return err
			}
			defer func() {
				if _, err := removeStack(s, true); err != nil {
					cmdutil.Diag().Warningf(
						diag.Message("", "failed to remove stack; exiting anyway: %v"), err)
				}
			}()

			// Parse and set configuration variables.
			ps, err := workspace.DetectProjectStack(s.Name().StackName())
			for i, vars := range [][]string{configVars, secretVars} {
				for _, kv := range vars {
					ix := strings.IndexRune(kv, '=')
					if ix == -1 {
						return errors.Errorf("invalid configuration flag; expected '<key>=<value>': %s\n", kv)
					}

					key, err := parseConfigKey(kv[:ix])
					if err != nil {
						return errors.Wrap(err, "invalid configuration key")
					}

					val := kv[ix+1:]
					if secret := i == 1; secret {
						crypter, err := b.GetStackCrypter(s.Name())
						if err != nil {
							return err
						}
						enc, err := crypter.EncryptValue(val)
						if err != nil {
							return err
						}
						ps.Config[key] = config.NewSecureValue(enc)
					} else {
						ps.Config[key] = config.NewValue(val)
					}
				}
			}
			if err = workspace.SaveProjectStack(s.Name().StackName(), ps); err != nil {
				return err
			}

			// Grab some update metadata to use for updates and destroys.
			m, err := getUpdateMetadata(message, root)
			if err != nil {
				return errors.Wrap(err, "gathering environment metadata")
			}

			// Ensure we destroy the stack before exiting.
			defer func() {
				if _, err := s.Destroy(commandContext(), proj, root, m, opts, cancellationScopes); err != nil {
					cmdutil.Diag().Warningf(
						diag.Message("", "failed to destroy stack; removal may fail: %v"), err)
				}
			}()

			// Perform an update to stand up the initial stack.
			if _, err := s.Update(commandContext(), proj, root, m, opts, cancellationScopes); err != nil {
				return err
			}

			// Now, tail the logs.  Keep doing this until ^C is pressed, and then tear down the stack.
			now := time.Now()
			return showLogs(s, &now, nil, true)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&debug, "debug", "d", false,
		"Print detailed debugging output during resource operations")
	cmd.PersistentFlags().StringVarP(
		&message, "message", "m", "",
		"Optional message to associate with the update operation")

	// Flags for engine.UpdateOptions.
	cmd.PersistentFlags().StringSliceVar(
		&analyzers, "analyzer", []string{},
		"Run one or more analyzers as part of this update")
	cmd.PersistentFlags().VarP(
		&color, "color", "c", "Colorize output. Choices are: always, never, raw, auto")
	cmd.PersistentFlags().StringArrayVar(
		&configVars, "config", nil, "Set configuration variables of the form '<key>=<value>'")
	cmd.PersistentFlags().BoolVar(
		&diffDisplay, "diff", false,
		"Display operation as a rich diff showing the overall change")
	cmd.PersistentFlags().BoolVar(
		&nonInteractive, "non-interactive", false, "Disable interactive mode")
	cmd.PersistentFlags().IntVarP(
		&parallel, "parallel", "p", 0,
		"Allow P resource operations to run in parallel at once (<=1 for no parallelism)")
	cmd.PersistentFlags().StringArrayVar(
		&secretVars, "secret", nil, "Set secret encrypted configuration variables of the form '<key>=<value>'")
	cmd.PersistentFlags().StringVarP(
		&stack, "stack", "s", "", "Use a specific stack name, rather than auto-generating one")

	return cmd
}

func createTempStack(proj *workspace.Project, stack string) (backend.Backend, backend.Stack, error) {
	b, err := currentBackend()
	if err != nil {
		return nil, nil, err
	}

	if stack == "" {
		rs, err := randomStackName(proj)
		if err != nil {
			return nil, nil, errors.Wrap(err, "generating random stack name")
		}
		stack = rs
	}

	stackRef, err := b.ParseStackReference(stack)
	if err != nil {
		return nil, nil, err
	}

	s, err := createStack(b, stackRef, nil)
	return b, s, err
}

func randomStackName(proj *workspace.Project) (string, error) {
	hash := make([]byte, 8)
	if _, err := rand.Read(hash); err != nil {
		return "", err
	}
	return fmt.Sprintf("run-%s-%s", proj.Name, hex.EncodeToString(hash)), nil
}
