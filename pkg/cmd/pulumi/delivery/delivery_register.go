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

package delivery

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	pulumicmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/operations"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	sdkconfig "github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	registerEnvVar  = "PULUMI_DELIVERY_REGISTER"
	sourceConfigKey = "delivery:source"
)

// pipelineSource is the value of the `delivery:source` stack config.
type pipelineSource struct {
	URL       string
	Directory string
}

// registerHiddenFlags are flags of `pulumi up` that a registration never uses. They still work.
var registerHiddenFlags = []string{
	"plan",
	"strict",
}

// registerPrepare runs before the update. It returns a function that reports the result once the update
// has finished.
type registerPrepare func(cmd *cobra.Command, stack, configFile string) (func(io.Writer), error)

func newDeliveryRegisterCmd() *cobra.Command {
	return newRegisterCmd(operations.NewUpCmd(), prepareRegistration)
}

// newRegisterCmd wraps up so that the command accepts every flag of `pulumi up` and runs it in
// registration mode.
func newRegisterCmd(up *cobra.Command, prepare registerPrepare) *cobra.Command {
	cmd := up
	cmd.Use = "register"
	cmd.Aliases = nil
	cmd.SuggestFor = nil
	cmd.Short = "[PREVIEW] Register the Pulumi Delivery pipeline that a program declares"
	cmd.Long = "[PREVIEW] Register the Pulumi Delivery pipeline that the program on this stack declares.\n" +
		"\n" +
		"This command registers the pipeline by running the Pulumi program and observing the pipeline\n" +
		"resources that it allocates. It stores them in Pulumi Cloud as the pipeline definition and\n" +
		"starts no run.\n" +
		"\n" +
		"The pipeline needs to know where its program lives. When the stack configuration has no\n" +
		"`delivery:source` value, this command detects the repository URL and the project directory\n" +
		"from the git checkout, and writes them to the stack configuration file. Commit that file so\n" +
		"that runs can find the program. The pipeline tracks the default branch of the repository; set\n" +
		"`delivery:source.branch` to track another branch."
	constrictor.AttachArguments(cmd, constrictor.NoArgs)

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if strings.HasPrefix(f.Name, "remote") || f.Name == "suppress-stream-logs" ||
			slices.Contains(registerHiddenFlags, f.Name) {
			f.Hidden = true
		}
	})

	runUp := up.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		stack, err := cmd.Flags().GetString("stack")
		if err != nil {
			return err
		}
		configFile, err := cmd.Flags().GetString("config-file")
		if err != nil {
			return err
		}
		report, err := prepare(cmd, stack, configFile)
		if err != nil {
			return err
		}
		if err := registerOutcome(runUp(cmd, nil)); err != nil {
			return err
		}
		report(cmd.OutOrStdout())
		return nil
	}
	return cmd
}

// prepareRegistration checks the project and backend, writes the pipeline source to the stack configuration
// when it is missing, and sets the registration variable. Plugins started by the update inherit the process
// environment.
func prepareRegistration(cmd *cobra.Command, stack, configFile string) (func(io.Writer), error) {
	ctx := cmd.Context()
	ws := pkgWorkspace.Instance
	opts := display.Options{Color: cmdutil.GetGlobalColorization()}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current working directory: %w", err)
	}
	project, projectDir, err := ws.ReadProject(cwd)
	if err != nil {
		if errors.Is(err, workspace.ErrProjectNotFound) {
			return nil, errors.New("`pulumi delivery register` must be run in a project directory")
		}
		return nil, err
	}
	if _, err := cmdBackend.CurrentBackend(
		ctx, ws, cmdBackend.DefaultLoginManager, project, opts); err != nil {
		return nil, err
	}

	s, err := cmdStack.RequireStack(ctx, cmdutil.Diag(), ws, cmdBackend.DefaultLoginManager,
		stack, cmdStack.LoadOnly, opts, configFile)
	if err != nil {
		return nil, err
	}
	if err := cmdStack.RejectConfigFlagsOnManagedStack(s, nil, configFile); err != nil {
		return nil, err
	}
	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, errors.New("`pulumi delivery register` requires the Pulumi Cloud backend; run `pulumi login`")
	}
	cloudBe, ok := cloudStack.Backend().(httpstate.Backend)
	if !ok {
		return nil, errors.New("`pulumi delivery register` requires the Pulumi Cloud backend; run `pulumi login`")
	}

	ps, err := cmdStack.LoadProjectStack(ctx, cmdutil.Diag(), project, s, configFile)
	if err != nil {
		return nil, err
	}
	src, err := writeSourceIfMissing(ps, func() (pipelineSource, error) { return detectSource(projectDir) },
		func(ps *workspace.ProjectStack) error { return cmdStack.SaveProjectStack(ctx, s, ps, configFile) })
	if err != nil {
		return nil, err
	}
	if src != nil {
		where := "the stack configuration file"
		if configFile != "" {
			where = configFile
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s (%s) to %s; commit it.\n", sourceConfigKey, src, where)
	}

	if err := os.Setenv(registerEnvVar, "true"); err != nil {
		return nil, err
	}

	stackRef := cloudStack.Ref()
	proj, _ := stackRef.Project()
	id := client.StackIdentifier{Owner: cloudStack.OrgName(), Project: string(proj), Stack: stackRef.Name()}
	consoleURL := client.CloudConsoleURL(cloudBe.CloudURL(), id.Owner, "delivery", id.Project, id.Stack.String())
	return func(w io.Writer) { printRegistered(w, id, consoleURL) }, nil
}

// writeSourceIfMissing sets the detected source in ps and saves it when ps has no `delivery:source` value.
// It returns the source it wrote, or nil when the value was already set.
func writeSourceIfMissing(
	ps *workspace.ProjectStack,
	detect func() (pipelineSource, error),
	save func(*workspace.ProjectStack) error,
) (*pipelineSource, error) {
	_, set, err := ps.Config.Get(sdkconfig.MustMakeKey("delivery", "source"), false)
	if err != nil || set {
		return nil, err
	}
	src, err := detect()
	if err != nil {
		return nil, err
	}
	if err := setSource(ps.Config, src); err != nil {
		return nil, err
	}
	if err := save(ps); err != nil {
		return nil, fmt.Errorf("writing %s: %w", sourceConfigKey, err)
	}
	return &src, nil
}

func (s pipelineSource) String() string {
	out := s.URL
	if s.Directory != "" {
		out += ", directory " + s.Directory
	}
	return out
}

// registerOutcome maps the result of the update to the result of the command. An update that ends with
// resources awaiting is how a registration normally finishes.
func registerOutcome(err error) error {
	if err == nil || pulumicmd.ExitCodeFor(err) == pulumicmd.ExitAwaitError {
		return nil
	}
	return err
}

func printRegistered(w io.Writer, id client.StackIdentifier, consoleURL string) {
	fmt.Fprintf(w, "Registered the pipeline of %s.\n%s\n", id, consoleURL)
}

// setSource sets the source in the stack configuration, as `pulumi config set --path` does.
func setSource(cfg sdkconfig.Map, src pipelineSource) error {
	for _, kv := range [][2]string{{"url", src.URL}, {"directory", src.Directory}} {
		if kv[1] == "" {
			continue
		}
		key := sdkconfig.MustMakeKey("delivery", "source."+kv[0])
		if err := cfg.Set(key, sdkconfig.NewTypedValue(kv[1], sdkconfig.TypeString), true); err != nil {
			return fmt.Errorf("writing %s: %w", sourceConfigKey, err)
		}
	}
	return nil
}

func sourceNotDetectedError(reason error) error {
	return fmt.Errorf("could not detect the pipeline source from git (%w); set the `%s` config:\n"+
		"  pulumi config set --path %s.url https://github.com/<org>/<repo>\n"+
		"  pulumi config set --path %s.directory <path of the project in the repository>",
		reason, sourceConfigKey, sourceConfigKey, sourceConfigKey)
}

// detectSource reads the source of the pipeline from the git checkout that contains projectDir.
func detectSource(projectDir string) (pipelineSource, error) {
	repo, err := gitutil.GetGitRepository(projectDir)
	if err != nil {
		return pipelineSource{}, sourceNotDetectedError(err)
	}
	if repo == nil {
		return pipelineSource{}, sourceNotDetectedError(errors.New("no git repository found"))
	}
	remote, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		return pipelineSource{}, sourceNotDetectedError(err)
	}
	remoteURL, err := normalizeRemoteURL(remote)
	if err != nil {
		return pipelineSource{}, sourceNotDetectedError(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return pipelineSource{}, sourceNotDetectedError(err)
	}
	dir, err := relativeDirectory(wt.Filesystem().Root(), projectDir)
	if err != nil {
		return pipelineSource{}, sourceNotDetectedError(err)
	}
	return pipelineSource{URL: remoteURL, Directory: dir}, nil
}

var scpRemote = regexp.MustCompile(`^(?:[^@/:]+@)?([^:/]+):(.+)$`)

// normalizeRemoteURL turns a git remote URL into an https URL without a `.git` suffix.
func normalizeRemoteURL(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	var host, path string
	if m := scpRemote.FindStringSubmatch(remote); m != nil && !strings.Contains(remote, "://") {
		host, path = m[1], m[2]
	} else {
		u, err := url.Parse(remote)
		if err != nil || u.Hostname() == "" {
			return "", fmt.Errorf("unsupported remote URL %q", remote)
		}
		host, path = u.Hostname(), u.Path
		if u.Scheme == "http" || u.Scheme == "https" {
			host = u.Host
		}
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if path == "" {
		return "", fmt.Errorf("unsupported remote URL %q", remote)
	}
	return "https://" + host + "/" + path, nil
}

// relativeDirectory returns the slash-separated path of dir inside root, or "" when dir is root.
func relativeDirectory(root, dir string) (string, error) {
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		dir = d
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}
