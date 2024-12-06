// Copyright 2016-2024, Pulumi Corporation.
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

package stack

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackOutputCmd() *cobra.Command {
	var socmd stackOutputCmd
	cmd := &cobra.Command{
		Use:   "output [property-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Show a stack's output properties",
		Long: "Show a stack's output properties.\n" +
			"\n" +
			"By default, this command lists all output properties exported from a stack.\n" +
			"If a specific property-name is supplied, just that property's value is shown.",
		Run: cmd.RunCmdFunc(func(cmd *cobra.Command, args []string) error {
			return socmd.Run(cmd.Context(), args)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&socmd.jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().BoolVar(
		&socmd.shellOut, "shell", false, "Emit output as a shell script")
	cmd.PersistentFlags().StringVarP(
		&socmd.stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVar(
		&socmd.showSecrets, "show-secrets", false, "Display outputs which are marked as secret in plaintext")

	return cmd
}

type stackOutputCmd struct {
	stackName   string
	showSecrets bool
	jsonOut     bool
	shellOut    bool

	ws pkgWorkspace.Context
	OS string // defaults to runtime.GOOS

	// requireStack is a reference to the top-level requireStack function.
	// This is a field on stackOutputCmd so that we can replace it
	// from tests.
	requireStack func(
		ctx context.Context, ws pkgWorkspace.Context, lm cmdBackend.LoginManager,
		name string, lopt LoadOption, opts display.Options,
	) (backend.Stack, error)

	Stdout io.Writer // defaults to os.Stdout
}

func (cmd *stackOutputCmd) Run(ctx context.Context, args []string) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	requireStack := RequireStack
	if cmd.requireStack != nil {
		requireStack = cmd.requireStack
	}

	if cmd.ws == nil {
		cmd.ws = pkgWorkspace.Instance
	}

	osys := runtime.GOOS
	if cmd.OS != "" {
		osys = cmd.OS
	}

	stdout := io.Writer(os.Stdout)
	if cmd.Stdout != nil {
		stdout = cmd.Stdout
	}

	var outw stackOutputWriter
	if cmd.shellOut && cmd.jsonOut {
		return errors.New("only one of --json and --shell may be set")
	} else if cmd.jsonOut {
		outw = &jsonStackOutputWriter{W: stdout}
	} else if cmd.shellOut {
		outw = newShellStackOutputWriter(stdout, osys)
	} else {
		outw = &consoleStackOutputWriter{W: stdout}
	}

	// Fetch the current stack and its output properties.
	s, err := requireStack(
		ctx,
		cmd.ws,
		cmdBackend.DefaultLoginManager,
		cmd.stackName,
		LoadOnly,
		opts,
	)
	if err != nil {
		return err
	}
	snap, err := s.Snapshot(ctx, stack.DefaultSecretsProvider)
	if err != nil {
		return err
	}

	outputs, err := getStackOutputs(snap, cmd.showSecrets)
	if err != nil {
		return fmt.Errorf("getting outputs: %w", err)
	}
	if outputs == nil {
		outputs = make(map[string]interface{})
	}

	// If there is an argument, just print that property.  Else, print them all (similar to `pulumi stack`).
	if len(args) > 0 {
		name := args[0]
		v, has := outputs[name]
		if has {
			if err := outw.WriteOne(name, v); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("current stack does not have output property '%v'", name)
		}
	} else {
		if err := outw.WriteMany(outputs); err != nil {
			return err
		}
	}

	if cmd.showSecrets {
		Log3rdPartySecretsProviderDecryptionEvent(ctx, s, "", "pulumi stack output")
	}

	return nil
}

// stackOutputWriter writes one or more properties to stdout
// on behalf of 'pulumi stack output'.
type stackOutputWriter interface {
	WriteOne(name string, value interface{}) error
	WriteMany(outputs map[string]interface{}) error
}

// consoleStackOutputWriter writes human-readable stack output to stdout.
type consoleStackOutputWriter struct {
	W io.Writer
}

var _ stackOutputWriter = (*consoleStackOutputWriter)(nil)

func (w *consoleStackOutputWriter) WriteOne(_ string, v interface{}) error {
	_, err := fmt.Fprintf(w.W, "%v\n", stringifyOutput(v))
	return err
}

func (w *consoleStackOutputWriter) WriteMany(outputs map[string]interface{}) error {
	return fprintStackOutputs(w.W, outputs)
}

// jsonStackOutputWriter writes stack outputs as machine-parseable JSON.
type jsonStackOutputWriter struct {
	W io.Writer
}

var _ stackOutputWriter = (*jsonStackOutputWriter)(nil)

func (w *jsonStackOutputWriter) WriteOne(_ string, v interface{}) error {
	return ui.FprintJSON(w.W, v)
}

func (w *jsonStackOutputWriter) WriteMany(outputs map[string]interface{}) error {
	return ui.FprintJSON(w.W, outputs)
}

// newShellStackOutputWriter builds a stackOutputWriter
// that generates shell scripts for the given operating system.
func newShellStackOutputWriter(w io.Writer, os string) stackOutputWriter {
	switch os {
	case "windows":
		return &powershellStackOutputWriter{W: w}
	default:
		return &bashStackOutputWriter{W: w}
	}
}

// bashStackOutputWriter prints stack outputs as a bash shell script.
type bashStackOutputWriter struct {
	W io.Writer
}

var _ stackOutputWriter = (*bashStackOutputWriter)(nil)

func (w *bashStackOutputWriter) WriteOne(k string, v interface{}) error {
	s := shellquote.Join(stringifyOutput(v))
	_, err := fmt.Fprintf(w.W, "%v=%v\n", k, s)
	return err
}

func (w *bashStackOutputWriter) WriteMany(outputs map[string]interface{}) error {
	keys := slice.Prealloc[string](len(outputs))
	for v := range outputs {
		keys = append(keys, v)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := w.WriteOne(k, outputs[k]); err != nil {
			return err
		}
	}
	return nil
}

// powershellStackOutputWriter prints stack outputs as a powershell script.
type powershellStackOutputWriter struct {
	W io.Writer
}

var _ stackOutputWriter = (*powershellStackOutputWriter)(nil)

func (w *powershellStackOutputWriter) WriteOne(k string, v interface{}) error {
	// In Powershell, single-quoted strings are taken verbatim.
	// The only escaping necessary is to ' itself:
	// replace each instance with two to escape.
	s := strings.ReplaceAll(stringifyOutput(v), "'", "''")
	_, err := fmt.Fprintf(w.W, "$%v = '%v'\n", k, s)
	return err
}

func (w *powershellStackOutputWriter) WriteMany(outputs map[string]interface{}) error {
	keys := slice.Prealloc[string](len(outputs))
	for v := range outputs {
		keys = append(keys, v)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := w.WriteOne(k, outputs[k]); err != nil {
			return err
		}
	}
	return nil
}

func getStackOutputs(snap *deploy.Snapshot, showSecrets bool) (map[string]interface{}, error) {
	state, err := stack.GetRootStackResource(snap)
	if err != nil {
		return nil, err
	}

	if state == nil {
		return map[string]interface{}{}, nil
	}

	// massageSecrets will remove all the secrets from the property map, so it should be safe to pass a panic
	// crypter. This also ensure that if for some reason we didn't remove everything, we don't accidentally disclose
	// secret values!
	ctx := context.TODO()
	return stack.SerializeProperties(ctx, display.MassageSecrets(state.Outputs, showSecrets),
		config.NewPanicCrypter(), showSecrets)
}
