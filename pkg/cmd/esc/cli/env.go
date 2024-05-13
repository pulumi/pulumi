// Copyright 2023, Pulumi Corporation.

package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type envCommand struct {
	esc *escCommand

	envNameFlag string
}

func newEnvCmd(esc *escCommand) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage environments",
		Long: "Manage environments\n" +
			"\n" +
			"An environment is a named collection of possibly-secret, possibly-dynamic data.\n" +
			"Each environment has a definition and may be opened in order to access its contents.\n" +
			"Opening an environment may involve generating new dynamic data.\n" +
			"\n" +
			"To begin working with environments, run the `env init` command:\n" +
			"\n" +
			"    env init\n" +
			"\n" +
			"This will prompt you to create a new environment to hold secrets and configuration.\n" +
			"\n" +
			"For more information, please visit the project page: https://www.pulumi.com/docs/esc",

		Args: cmdutil.NoArgs,
	}

	env := &envCommand{esc: esc}

	cmd.PersistentFlags().StringVar(&env.envNameFlag, "env", "", "The name of the environment to operate on.")

	cmd.AddCommand(newEnvInitCmd(env))
	cmd.AddCommand(newEnvEditCmd(env))
	cmd.AddCommand(newEnvGetCmd(env))
	cmd.AddCommand(newEnvDiffCmd(env))
	cmd.AddCommand(newEnvSetCmd(env))
	cmd.AddCommand(newEnvLogCmd(env))
	cmd.AddCommand(newEnvVersionCmd(env))
	cmd.AddCommand(newEnvLsCmd(env))
	cmd.AddCommand(newEnvRmCmd(env))
	cmd.AddCommand(newEnvOpenCmd(env))
	cmd.AddCommand(newEnvRunCmd(env))

	return cmd
}

func (cmd *envCommand) getEnvName(args []string) (org, env, revisionOrTag string, rest []string, err error) {
	if cmd.envNameFlag == "" {
		if len(args) == 0 {
			return "", "", "", nil, fmt.Errorf("no environment name specified")
		}
		cmd.envNameFlag, args = args[0], args[1:]
	}

	orgName, envName, hasOrgName := strings.Cut(cmd.envNameFlag, "/")
	if !hasOrgName {
		orgName, envName = cmd.esc.account.DefaultOrg, orgName
	}

	envName, revisionOrTag, _ = strings.Cut(envName, ":")

	return orgName, envName, revisionOrTag, args, nil
}

func sortEnvironmentDiagnostics(diags []client.EnvironmentDiagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		di, dj := diags[i], diags[j]
		if di.Range == nil {
			if dj.Range == nil {
				return di.Summary < dj.Summary
			}
			return true
		}
		if dj.Range == nil {
			return false
		}
		if di.Range.Environment != dj.Range.Environment {
			return di.Range.Environment < dj.Range.Environment
		}
		if di.Range.Begin.Line != dj.Range.Begin.Line {
			return di.Range.Begin.Line < dj.Range.Begin.Line
		}
		return di.Range.Begin.Column < dj.Range.Begin.Column
	})
}

func (cmd *envCommand) writeYAMLEnvironmentDiagnostics(
	out io.Writer,
	envName string,
	yaml []byte,
	diags []client.EnvironmentDiagnostic,
) error {
	width, color := 0, false
	if file, ok := out.(*os.File); ok {
		w, _, err := term.GetSize(int(file.Fd()))
		if err != nil {
			w = 0
		}
		width, color = w, cmd.esc.colors != colors.Never
	}

	files := map[string]*hcl.File{envName: {Bytes: yaml}}
	writer := hcl.NewDiagnosticTextWriter(out, files, uint(width), color)

	sortEnvironmentDiagnostics(diags)

	for _, d := range diags {
		var subject *hcl.Range
		if d.Range != nil {
			subject = &hcl.Range{
				Filename: d.Range.Environment,
				Start: hcl.Pos{
					Line:   d.Range.Begin.Line,
					Column: d.Range.Begin.Column,
					Byte:   d.Range.Begin.Byte,
				},
				End: hcl.Pos{
					Line:   d.Range.End.Line,
					Column: d.Range.End.Column,
					Byte:   d.Range.End.Byte,
				},
			}
		}
		err := writer.WriteDiagnostic(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  d.Summary,
			Subject:  subject,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *envCommand) writePropertyEnvironmentDiagnostics(out io.Writer, diags []client.EnvironmentDiagnostic) error {
	sortEnvironmentDiagnostics(diags)

	var b strings.Builder
	for _, d := range diags {
		b.Reset()

		if d.Range != nil {
			fmt.Fprintf(&b, "%v%v:", colors.Red, d.Range.Environment)
			if d.Range.Begin.Line != 0 {
				fmt.Fprintf(&b, "%v:%v:", d.Range.Begin.Line, d.Range.Begin.Column)
			}
			fmt.Fprintf(&b, " ")
		}
		fmt.Fprintln(&b, d.Summary)

		fmt.Fprint(out, cmd.esc.colors.Colorize(b.String()))
	}

	return nil
}
