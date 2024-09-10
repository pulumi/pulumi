// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	aho_corasick "github.com/petar-dambovaliev/aho-corasick"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/spf13/cobra"
)

func newReplacer(secrets []string) aho_corasick.Replacer {
	// Ignore very short secrets (anything less than 3 characters). Such secrets have low entropy, and redacting them is
	// unlikely to be useful (in the case of _empty_ secrets, including them is harmful, as it causes the redactor
	// to insert "[secret]" after every character).
	d := 0
	for _, s := range secrets {
		if len(s) >= 3 {
			secrets[d], d = s, d+1
		}
	}
	secrets = secrets[:d]

	builder := aho_corasick.NewAhoCorasickBuilder(aho_corasick.Opts{
		MatchKind: aho_corasick.StandardMatch,
	})
	return aho_corasick.NewReplacer(builder.Build(secrets))
}

type redactor struct {
	w        io.Writer
	replacer aho_corasick.Replacer
	line     bytes.Buffer
}

func newRedactor(w io.Writer, replacer aho_corasick.Replacer) *redactor {
	return &redactor{w: w, replacer: replacer}
}

func (w *redactor) Write(b []byte) (int, error) {
	written := 0
	for {
		newline := bytes.IndexByte(b, '\n')
		if newline == -1 {
			n, err := w.line.Write(b)
			contract.IgnoreError(err)

			return written + n, nil
		}

		n := w.line.Len()
		_, err := w.line.Write(b[:newline+1])
		contract.IgnoreError(err)

		redacted := w.replacer.ReplaceAllFunc(w.line.String(), func(m aho_corasick.Match) (string, bool) {
			return "[secret]", true
		})

		if _, err = w.w.Write([]byte(redacted)); err != nil {
			w.line.Truncate(n)
			return written, err
		}
		w.line.Reset()

		b, written = b[newline+1:], written+newline+1
	}
}

func (w *redactor) Close() error {
	if w.line.Len() != 0 {
		rest := w.line.String()
		w.line.Reset()

		redacted := w.replacer.ReplaceAllFunc(rest, func(m aho_corasick.Match) (string, bool) {
			return "[secret]", true
		})

		_, err := w.w.Write([]byte(redacted))
		return err
	}
	return nil
}

func newEnvRunCmd(envcmd *envCommand) *cobra.Command {
	var interactive bool
	var duration time.Duration

	shell := valueOrDefault(filepath.Base(envcmd.esc.environ.Get("SHELL")), "sh")

	cmd := &cobra.Command{
		Use:   "run [<org-name>/][<project-name>/]<environment-name> [flags] -- [command]",
		Args:  cobra.ArbitraryArgs,
		Short: "Open the environment with the given name and run a command.",
		Long: fmt.Sprintf("Open the environment with the given name and run a command\n"+
			"\n"+
			"This command opens the environment with the given name and runs the given command.\n"+
			"If the opened environment contains a top-level 'environmentVariables' object, each\n"+
			"key-value pair in the object is made available to the command as an environment\n"+
			"variable. Note that commands are not run in a subshell, so environment variable\n"+
			"references in the command are not expanded by default. You should invoke the command\n"+
			"inside a shell if you need environment variable expansion:\n"+
			"\n"+
			"    run -- %[1]s -c '\"echo $MY_ENV_VAR\"'\n"+
			"\n"+
			"The command to run is assumed to be non-interactive by default and its output\n"+
			"streams are filtered to remove any secret values. Use the -i flag to run interactive\n"+
			"commands, which will disable filtering.\n"+
			"\n"+
			"It is not strictly required that you pass `--`. The `--` indicates that any\n"+
			"arguments that follow it should be treated as positional arguments instead of flags.\n"+
			"It is only required if the arguments to the command you would like to run include\n"+
			"flags of the form `--flag` or `-f`.\n", shell),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			if len(args) == 0 {
				return fmt.Errorf("no command specified")
			}
			command, err := envcmd.esc.exec.LookPath(args[0])
			if err != nil {
				return fmt.Errorf("resolving command: %w", err)
			}
			args = args[1:]

			env, diags, err := envcmd.openEnvironment(ctx, ref, duration)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			files, environ, secrets, err := envcmd.prepareEnvironment(env, PrepareOptions{})
			if err != nil {
				return err
			}
			defer envcmd.removeTemporaryFiles(files)

			envV := esc.NewValue(env.Properties)
			for i, v := range args {
				interp, diags := ast.Interpolate(v)
				if !diags.HasErrors() {
					var arg strings.Builder
					for _, p := range interp.Parts {
						arg.WriteString(p.Text)
						if p.Value != nil {
							path := make(resource.PropertyPath, len(p.Value.Accessors))
							for i, accessor := range p.Value.Accessors {
								switch accessor := accessor.(type) {
								case *ast.PropertyName:
									path[i] = accessor.Name
								case *ast.PropertySubscript:
									path[i] = accessor.Index
								default:
									contract.Failf("unexpected accessor of type %T", accessor)
								}
							}
							if val, ok := getEnvValue(envV, path); ok {
								str := val.ToString(false)
								if val.Secret {
									secrets = append(secrets, str)
								}

								arg.WriteString(str)
							}
						}
					}
					args[i] = arg.String()
				}
			}

			runCmd := exec.Command(command, args...)
			runCmd.Env = append(envcmd.esc.environ.Vars(), environ...)

			stdout, stderr := envcmd.esc.stdout, envcmd.esc.stderr
			if !interactive {
				replacer := newReplacer(secrets)
				redactedStdout, redactedStderr := newRedactor(stdout, replacer), newRedactor(stderr, replacer)
				defer contract.IgnoreClose(redactedStdout)
				defer contract.IgnoreClose(redactedStderr)
				stdout, stderr = redactedStdout, redactedStderr
			}

			runCmd.Stdin = envcmd.esc.stdin
			runCmd.Stdout = stdout
			runCmd.Stderr = stderr
			return envcmd.esc.exec.Run(runCmd)
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "true to treat the command as interactive and disable output filters")
	cmd.Flags().DurationVarP(&duration, "lifetime", "l", 2*time.Hour, "the lifetime of the opened environment")

	return cmd
}
