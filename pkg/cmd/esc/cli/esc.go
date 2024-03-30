// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/cmd/esc/cli/version"
	"github.com/pulumi/esc/cmd/esc/cli/workspace"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

type Options struct {
	ParentPath string
	UserAgent  string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	Colors colors.Colorization

	Login           httpstate.LoginManager
	PulumiWorkspace workspace.PulumiWorkspace

	fs      escFS
	environ environ
	exec    cmdExec
	pager   pager

	newClient func(userAgent, backendURL, accessToken string, insecure bool) client.Client
}

type escCommand struct {
	fs      escFS
	environ environ
	exec    cmdExec
	pager   pager

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	command string
	colors  colors.Colorization

	login     httpstate.LoginManager
	workspace *workspace.Workspace

	userAgent string
	newClient func(userAgent, backendURL, accessToken string, insecure bool) client.Client
	client    client.Client
	account   workspace.Account
}

func newESC(opts *Options) *escCommand {
	fs := valueOrDefault(opts.fs, newFS())

	esc := &escCommand{
		fs:        fs,
		environ:   valueOrDefault(opts.environ, newEnviron()),
		exec:      valueOrDefault(opts.exec, newCmdExec()),
		pager:     valueOrDefault(opts.pager, newPager()),
		stdin:     valueOrDefault(opts.Stdin, io.Reader(os.Stdin)),
		stdout:    valueOrDefault(opts.Stdout, io.Writer(os.Stdout)),
		stderr:    valueOrDefault(opts.Stderr, io.Writer(os.Stderr)),
		command:   valueOrDefault(opts.ParentPath, "esc"),
		colors:    valueOrDefault(opts.Colors, cmdutil.GetGlobalColorization()),
		login:     valueOrDefault(opts.Login, httpstate.NewLoginManager()),
		workspace: workspace.New(fs, valueOrDefault(opts.PulumiWorkspace, workspace.DefaultPulumiWorkspace())),
		userAgent: valueOrDefault(opts.UserAgent, fmt.Sprintf("esc-cli/1 (%s; %s)", version.Version, runtime.GOOS)),
		newClient: opts.newClient,
	}
	if esc.newClient == nil {
		esc.newClient = client.New
	}

	return esc
}

// New creates a new ESC CLI instance.
func New(opts *Options) *cobra.Command {
	if opts == nil {
		opts = &Options{}
	}

	command := "esc"
	if opts.ParentPath != "" {
		command = opts.ParentPath + " esc"
	}

	cmd := &cobra.Command{
		Use:   "esc",
		Short: "Pulumi ESC command line",
		Long: fmt.Sprintf("Pulumi ESC - Manage environments, secrets, and configuration\n"+
			"\n"+
			"To begin working with Pulumi ESC, run the `%[1]s env init` command:\n"+
			"\n"+
			"    $ %[1]s env init\n"+
			"\n"+
			"This will prompt you to create a new environment to hold secrets and configuration.\n"+
			"\n"+
			"The most common commands from there are:\n"+
			"\n"+
			"    - %[1]s env get  : Get a property in an environment definition\n"+
			"    - %[1]s env set  : Set a property in an environment definition\n"+
			"    - %[1]s env edit : Edit an environment definition\n"+
			"    - %[1]s env ls   : List available environments\n"+
			"    - %[1]s run      : Run a command within the context of an environment\n"+
			"    - %[1]s open     : Open an environment and access its contents\n"+
			"\n"+
			"For more information, please visit the project page: https://www.pulumi.com/docs/esc", command),
	}

	esc := newESC(opts)

	env := newEnvCmd(esc)
	cmd.AddCommand(env)

	// Add top-level open/run aliases. We copy the commands to new struct values because
	// `AddCommand` mutates the command, modifying its parent, which can cause issues
	// with generated docs.
	openCmdCopy := *getCommand(env, "open")
	runCmdCopy := *getCommand(env, "run")
	cmd.AddCommand(&openCmdCopy)
	cmd.AddCommand(&runCmdCopy)

	cmd.AddCommand(newLoginCmd(esc))
	cmd.AddCommand(newLogoutCmd(esc))
	cmd.AddCommand(newVersionCmd(esc))
	cmd.AddCommand(newGenDocsCmd(cmd))

	return cmd
}

func getCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func valueOrDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}

func (esc *escCommand) confirmPrompt(prompt, name string) bool {
	if prompt != "" {
		fmt.Fprint(esc.stdout,
			esc.colors.Colorize(
				fmt.Sprintf("%s%s%s\n", colors.SpecAttention, prompt, colors.Reset)))
	}

	fmt.Fprint(esc.stdout,
		esc.colors.Colorize(
			fmt.Sprintf("%sPlease confirm that this is what you'd like to do by typing `%s%s%s`:%s ",
				colors.SpecAttention, colors.SpecPrompt, name, colors.SpecAttention, colors.Reset)))

	reader := bufio.NewReader(esc.stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line) == name
}
