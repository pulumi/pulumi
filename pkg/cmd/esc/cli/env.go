// Copyright 2023, Pulumi Corporation.

package cli

import (
	"context"
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

type ambiguousIdentifierError struct {
	legacyRef environmentRef
	ref       environmentRef
}

func (e ambiguousIdentifierError) Error() string {
	return fmt.Sprintf(
		"ambiguous path provided\n\nEnvironments found at both '%s' and '%s'.\nPlease specify the full path as <org-name>/<project-name>/<env-name>",
		e.ref.String(),
		e.legacyRef.String(),
	)
}

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
	cmd.AddCommand(newEnvCloneCmd(env))
	cmd.AddCommand(newEnvEditCmd(env))
	cmd.AddCommand(newEnvGetCmd(env))
	cmd.AddCommand(newEnvDiffCmd(env))
	cmd.AddCommand(newEnvSetCmd(env))
	cmd.AddCommand(newEnvVersionCmd(env))
	cmd.AddCommand(newEnvLsCmd(env))
	cmd.AddCommand(newEnvTagCmd((env)))
	cmd.AddCommand(newEnvRmCmd(env))
	cmd.AddCommand(newEnvOpenCmd(env))
	cmd.AddCommand(newEnvRunCmd(env))

	return cmd
}

type environmentRef struct {
	orgName     string
	projectName string
	envName     string
	version     string

	isUsingLegacyID  bool
	hasAmbiguousPath bool
}

func (r *environmentRef) Id() string {
	s := fmt.Sprintf("%s/%s", r.projectName, r.envName)

	if r.version != "" {
		s = fmt.Sprintf("%s@%s", s, r.version)
	}
	return s
}

func (r *environmentRef) String() string {
	s := r.Id()

	if r.orgName != "" {
		s = fmt.Sprintf("%s/%s", r.orgName, s)
	}
	return s
}

func (cmd *envCommand) parseRef(refStr string) environmentRef {
	var orgName, projectName, envNameAndVersion string

	hasAmbiguousPath := false
	orgName = cmd.esc.account.DefaultOrg
	projectName = client.DefaultProject
	isUsingLegacyID := false

	parts := strings.Split(refStr, "/")

	switch l := len(parts); {
	case l == 1:
		// <environment-name>
		envNameAndVersion = parts[0]

		isUsingLegacyID = true
	case l == 2:
		// <project-name>/<env-name> or <org-name>/<env-name>
		// We assume the former, and this will be disambiguated later.
		projectName = parts[0]
		envNameAndVersion = parts[1]

		hasAmbiguousPath = true
	case l >= 3:
		// <org-name>/<project-name>/<environment-name>
		orgName = parts[0]
		projectName = parts[1]
		envNameAndVersion = strings.Join(parts[2:], "")
	}

	envName, version, hasSep := strings.Cut(envNameAndVersion, "@")
	if !hasSep {
		envName, version, _ = strings.Cut(envNameAndVersion, ":")
	}

	return environmentRef{
		orgName:          orgName,
		projectName:      projectName,
		envName:          envName,
		version:          version,
		isUsingLegacyID:  isUsingLegacyID,
		hasAmbiguousPath: hasAmbiguousPath,
	}
}

// getEnvRef returns an environment reference corresponding to the given ref string
// and a bool indicating if the environment reference is relative.
//
// If `refString` is only a version (i.e. "@123") and a non-nil environmentRef `rel` is provided,
// the returned environment reference is "relative" and will default to the provided environmentRef's values
func (cmd *envCommand) getEnvRef(refString string, rel *environmentRef) (environmentRef, bool) {
	envRef := cmd.parseRef(refString)

	isRelative := false
	// If refString is only a version, copy fields from `rel`
	if rel != nil && envRef.envName == "" && envRef.version != "" {
		envRef.orgName = rel.orgName
		envRef.projectName = rel.projectName
		envRef.envName = rel.envName

		isRelative = true
	}

	return envRef, isRelative
}

// Get an environment reference when creating a new environment
// If the given path is ambiguous, we need to make additional API calls to disambiguate
func (cmd *envCommand) getNewEnvRef(
	ctx context.Context,
	args []string,
) (environmentRef, []string, error) {
	if cmd.envNameFlag == "" {
		if len(args) == 0 {
			return environmentRef{}, nil, fmt.Errorf("no environment name specified")
		}
		cmd.envNameFlag, args = args[0], args[1:]
	}

	ref, isRelative := cmd.getEnvRef(cmd.envNameFlag, nil)

	if !ref.hasAmbiguousPath {
		if !strings.Contains(cmd.envNameFlag, "/") && !isRelative {
			cmd.printDeprecatedNameMessage(cmd.envNameFlag, ref)
		}

		return ref, args, nil
	}

	// Check if project at <org-name>/<project-name> exists. Assume not if listing environments errors
	allEnvs, _ := cmd.listEnvironments(ctx, "", "")
	existsProject := false
	for _, e := range allEnvs {
		if strings.EqualFold(e.Project, ref.projectName) {
			existsProject = true
			break
		}
	}

	// Check if user is part of the organization from legacy path pattern <org-name>/default/<environment-name>
	legacyRef := environmentRef{
		orgName:          ref.projectName,
		projectName:      client.DefaultProject,
		envName:          ref.envName,
		version:          ref.version,
		isUsingLegacyID:  true,
		hasAmbiguousPath: ref.hasAmbiguousPath,
	}

	existsLegacyPath := false
	_, orgs, _, _ := cmd.esc.client.GetPulumiAccountDetails(ctx)
	for _, org := range orgs {
		if strings.EqualFold(legacyRef.orgName, org) {
			existsLegacyPath = true
			break
		}
	}

	if !existsProject && existsLegacyPath {
		cmd.printDeprecatedNameMessage(cmd.envNameFlag, legacyRef)
		return legacyRef, args, nil
	}

	return ref, args, nil
}

// Get an environment reference for an existing environment
// If the given path is ambiguous, we need to make additional API calls to disambiguate
func (cmd *envCommand) getExistingEnvRef(
	ctx context.Context,
	args []string,
) (environmentRef, []string, error) {
	if cmd.envNameFlag == "" {
		if len(args) == 0 {
			return environmentRef{}, nil, fmt.Errorf("no environment name specified")
		}
		cmd.envNameFlag, args = args[0], args[1:]
	}

	envRef, err := cmd.getExistingEnvRefWithRelative(ctx, cmd.envNameFlag, nil)

	return envRef, args, err
}

func (cmd *envCommand) getExistingEnvRefWithRelative(
	ctx context.Context,
	refString string,
	rel *environmentRef,
) (environmentRef, error) {
	ref, isRelative := cmd.getEnvRef(refString, rel)

	if !ref.hasAmbiguousPath {
		if !strings.Contains(refString, "/") && !isRelative {
			cmd.printDeprecatedNameMessage(refString, ref)
		}

		return ref, nil
	}

	// Check <org-name>/<project-name>/<environment-name>
	exists, _ := cmd.esc.client.EnvironmentExists(ctx, ref.orgName, ref.projectName, ref.envName)

	// Check legacy path <org-name>/default/<environment-name>
	legacyRef := environmentRef{
		orgName:          ref.projectName,
		projectName:      client.DefaultProject,
		envName:          ref.envName,
		version:          ref.version,
		isUsingLegacyID:  true,
		hasAmbiguousPath: ref.hasAmbiguousPath,
	}

	existsLegacyPath, _ := cmd.esc.client.EnvironmentExists(
		ctx,
		legacyRef.orgName,
		legacyRef.projectName,
		legacyRef.envName,
	)

	// Require unambiguous path if both paths exist
	if exists && existsLegacyPath {
		return ref, ambiguousIdentifierError{
			legacyRef: legacyRef,
			ref:       ref,
		}
	}

	if existsLegacyPath {
		cmd.printDeprecatedNameMessage(refString, legacyRef)
		return legacyRef, nil
	}

	return ref, nil
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

func (cmd *envCommand) printDeprecatedNameMessage(name string, ref environmentRef) {
	msg := fmt.Sprintf(
		"%sWarning: Referring to an environment name ('%s') without a project is deprecated.\nPlease use '%s/%s' or '%s' instead.%s",
		colors.SpecWarning, name, ref.orgName, ref.Id(), ref.Id(), colors.Reset)
	fmt.Fprintln(cmd.esc.stderr, cmd.esc.colors.Colorize(msg))
}
