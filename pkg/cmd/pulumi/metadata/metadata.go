// Copyright 2024, Pulumi Corporation.
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

package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	multierror "github.com/hashicorp/go-multierror"
	"github.com/spf13/pflag"

	git "github.com/go-git/go-git/v5"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/constant"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/ciutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	declared "github.com/pulumi/pulumi/sdk/v3/go/common/util/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

// GetUpdateMetadata returns an UpdateMetadata object, with optional data about the environment
// performing the update.
func GetUpdateMetadata(
	msg, root, execKind, execAgent string, updatePlan bool, cfg backend.StackConfiguration, flags *pflag.FlagSet,
) (*backend.UpdateMetadata, error) {
	m := &backend.UpdateMetadata{
		Message:     msg,
		Environment: make(map[string]string),
	}

	addPulumiCLIMetadataToEnvironment(m.Environment, flags, os.Environ)

	if err := addGitMetadata(root, m); err != nil {
		logging.V(3).Infof("errors detecting git metadata: %s", err)
	}

	addCIMetadataToEnvironment(m.Environment)

	addExecutionMetadataToEnvironment(m.Environment, execKind, execAgent)

	addUpdatePlanMetadataToEnvironment(m.Environment, updatePlan)

	addEscMetadataToEnvironment(m.Environment, cfg.EnvironmentImports)

	return m, nil
}

// addPulumiCLIMetadataToEnvironment enriches updates with metadata to provide context about
// the system, environment, and options that an update is being performed with.
func addPulumiCLIMetadataToEnvironment(env map[string]string, flags *pflag.FlagSet, getEnviron func() []string) {
	// Pulumi Environment Variables (Name and Set/Truthiness Only)

	// Set up a map of all known bool environment variables.
	boolVars := map[string]struct{}{}
	for _, e := range declared.Variables() {
		_, ok := e.Value.(declared.BoolValue)
		if !ok {
			continue
		}
		boolVars[e.Name()] = struct{}{}
	}

	// Add all environment variables that start with "PULUMI_" and their set-ness or truthiness.
	for _, envvar := range getEnviron() {
		if !strings.HasPrefix(envvar, "PULUMI_") {
			// Not a Pulumi environment variable. Skip.
			continue
		}
		name, value, ok := strings.Cut(envvar, "=")
		if !ok {
			// Invalid environment variable. Skip.
			continue
		}
		flag := "set"
		if _, isBoolVar := boolVars[name]; isBoolVar {
			flag = "false"
			if cmdutil.IsTruthy(value) {
				flag = "true"
			}
		}

		// Only store the variable name and set-ness or truthiness not the value.
		env["pulumi.env."+name] = flag
	}

	// System information
	env["pulumi.version"] = version.Version
	env["pulumi.os"] = runtime.GOOS
	env["pulumi.arch"] = runtime.GOARCH

	// Guard against nil pointer dereference.
	if flags == nil {
		// No command was provided. Don't add flags.
		return
	}

	// Pulumi CLI Flags (Name Only)
	flags.Visit(func(f *pflag.Flag) {
		env["pulumi.flag."+f.Name] = func() string {
			truth, err := flags.GetBool(f.Name)
			switch {
			case err != nil:
				return "set" // not a bool flag
			case truth:
				return "true"
			default:
				return "false"
			}
		}()
	})
}

// addCIMetadataToEnvironment populates the environment metadata bag with CI/CD-related values.
func addCIMetadataToEnvironment(env map[string]string) {
	// Add the key/value pair to env, if there actually is a value.
	addIfSet := func(key, val string) {
		if val != "" {
			env[key] = val
		}
	}

	// Use our built-in CI/CD detection logic.
	vars := ciutil.DetectVars()
	if vars.Name == "" {
		return
	}
	env[backend.CISystem] = string(vars.Name)
	addIfSet(backend.CIBuildID, vars.BuildID)
	addIfSet(backend.CIBuildNumer, vars.BuildNumber)
	addIfSet(backend.CIBuildType, vars.BuildType)
	addIfSet(backend.CIBuildURL, vars.BuildURL)
	addIfSet(backend.CIPRHeadSHA, vars.SHA)
	addIfSet(backend.CIPRNumber, vars.PRNumber)
}

// addExecutionMetadataToEnvironment populates the environment metadata bag with execution-related values.
func addExecutionMetadataToEnvironment(env map[string]string, execKind, execAgent string) {
	// this comes from a hidden flag, so we restrict the set of allowed values
	switch execKind {
	case constant.ExecKindAutoInline:
		break
	case constant.ExecKindAutoLocal:
		break
	case constant.ExecKindCLI:
		break
	default:
		execKind = constant.ExecKindCLI
	}
	env[backend.ExecutionKind] = execKind
	if execAgent != "" {
		env[backend.ExecutionAgent] = execAgent
	}
}

// addUpdatePlanMetadataToEnvironment populates the environment metadata bag with update plan related values.
func addUpdatePlanMetadataToEnvironment(env map[string]string, updatePlan bool) {
	env[backend.UpdatePlan] = strconv.FormatBool(updatePlan)
}

// addEscMetadataToEnvironment populates the environment metadata bag with the ESC environments
// used as part of the stack update.
func addEscMetadataToEnvironment(env map[string]string, escEnvironments []string) {
	envs := make([]apitype.EscEnvironmentMetadata, len(escEnvironments))
	for i, s := range escEnvironments {
		envs[i] = apitype.EscEnvironmentMetadata{ID: s}
	}

	jsonData, err := json.Marshal(envs)
	if err != nil {
		return
	}

	env[backend.StackEnvironments] = string(jsonData)
}

// addGitMetadata populate's the environment metadata bag with Git-related values.
func addGitMetadata(projectRoot string, m *backend.UpdateMetadata) error {
	var allErrors *multierror.Error

	// Gather git-related data as appropriate. (Returns nil, nil if no repo found.)
	repo, err := gitutil.GetGitRepository(projectRoot)
	if err != nil {
		return fmt.Errorf("detecting Git repository: %w", err)
	}
	if repo == nil {
		// If we couldn't find a repository, see if explicit environment variables have been set to provide us with
		// metadata.
		addGitMetadataFromEnvironment(m)

		return nil
	}

	if err := addGitRemoteMetadataToMap(repo, projectRoot, m.Environment); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	if err := addGitCommitMetadata(repo, projectRoot, m); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	return allErrors.ErrorOrNil()
}

// addGitMetadataFromEnvironment retrieves Git-related metadata from environment variables in the case that a Git
// repository is not present and adds any available information to the given metadata object.
func addGitMetadataFromEnvironment(m *backend.UpdateMetadata) {
	if owner := os.Getenv("PULUMI_VCS_REPO_OWNER"); owner != "" {
		m.Environment[backend.VCSRepoOwner] = owner
	}
	if repo := os.Getenv("PULUMI_VCS_REPO_NAME"); repo != "" {
		m.Environment[backend.VCSRepoName] = repo
	}
	if kind := os.Getenv("PULUMI_VCS_REPO_KIND"); kind != "" {
		m.Environment[backend.VCSRepoKind] = kind
	}
	if root := os.Getenv("PULUMI_VCS_REPO_ROOT"); root != "" {
		m.Environment[backend.VCSRepoRoot] = root
	}

	// As in addGitCommitMetadata, which we'd call if we had a Git repository, we'll use any information provided by the
	// CI system (or appropriate environment variables) to populate remaining Git-related fields that we don't have
	// explicit values for.
	vars := ciutil.DetectVars()

	message := os.Getenv("PULUMI_GIT_COMMIT_MESSAGE")
	if message == "" {
		message = vars.CommitMessage
	}
	if message != "" {
		m.Message = gitCommitTitle(message)
	}

	headName := os.Getenv("PULUMI_GIT_HEAD_NAME")
	if headName == "" {
		headName = vars.BranchName
	}
	if headName != "" {
		// In the case of GitHeadName, when we auto-detect from a Git repository we do not set the value if it is "HEAD".
		// However, in the case that a value has been set explicitly, we will use it regardless of its value, since this is
		// probably what the user expects.
		m.Environment[backend.GitHeadName] = headName
	}

	if head := os.Getenv("PULUMI_GIT_HEAD"); head != "" {
		m.Environment[backend.GitHead] = head
	}
	if committer := os.Getenv("PULUMI_GIT_COMMITTER"); committer != "" {
		m.Environment[backend.GitCommitter] = committer
	}
	if committerEmail := os.Getenv("PULUMI_GIT_COMMITTER_EMAIL"); committerEmail != "" {
		m.Environment[backend.GitCommitterEmail] = committerEmail
	}
	if author := os.Getenv("PULUMI_GIT_AUTHOR"); author != "" {
		m.Environment[backend.GitAuthor] = author
	}
	if authorEmail := os.Getenv("PULUMI_GIT_AUTHOR_EMAIL"); authorEmail != "" {
		m.Environment[backend.GitAuthorEmail] = authorEmail
	}
}

// addGitRemoteMetadataToMap reads the given git repo and adds its metadata to the given map bag.
func addGitRemoteMetadataToMap(repo *git.Repository, projectRoot string, env map[string]string) error {
	var allErrors *multierror.Error

	// Get the remote URL for this repo.
	remoteURL, err := gitutil.GetGitRemoteURL(repo, "origin")
	if err != nil {
		return fmt.Errorf("detecting Git remote URL: %w", err)
	}
	if remoteURL == "" {
		return nil
	}

	// Check if the remote URL is a GitHub or a GitLab URL.
	if err := addVCSMetadataToEnvironment(remoteURL, env); err != nil {
		allErrors = multierror.Append(allErrors, err)
	}

	// Add the repository root path.
	tree, err := repo.Worktree()
	if err != nil {
		allErrors = multierror.Append(allErrors, fmt.Errorf("detecting VCS root: %w", err))
	} else {
		rel, err := filepath.Rel(tree.Filesystem.Root(), projectRoot)
		if err != nil {
			allErrors = multierror.Append(allErrors, fmt.Errorf("detecting project root: %w", err))
		} else if !strings.HasPrefix(rel, "..") {
			env[backend.VCSRepoRoot] = filepath.ToSlash(rel)
		}
	}

	return allErrors.ErrorOrNil()
}

func addVCSMetadataToEnvironment(remoteURL string, env map[string]string) error {
	// GitLab, Bitbucket, Azure DevOps etc. repo slug if applicable.
	// We don't require a cloud-hosted VCS, so swallow errors.
	vcsInfo, err := gitutil.TryGetVCSInfo(remoteURL)
	if err != nil {
		return fmt.Errorf("detecting VCS project information: %w", err)
	}
	env[backend.VCSRepoOwner] = vcsInfo.Owner
	env[backend.VCSRepoName] = vcsInfo.Repo
	env[backend.VCSRepoKind] = vcsInfo.Kind

	return nil
}

func addGitCommitMetadata(repo *git.Repository, repoRoot string, m *backend.UpdateMetadata) error {
	// When running in a CI/CD environment, the current git repo may be running from a
	// detached HEAD and may not have have the latest commit message. We fall back to
	// CI-system specific environment variables when possible.
	ciVars := ciutil.DetectVars()

	// Commit at HEAD
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("getting repository HEAD: %w", err)
	}

	hash := head.Hash()
	m.Environment[backend.GitHead] = hash.String()
	commit, commitErr := repo.CommitObject(hash)
	if commitErr != nil {
		return fmt.Errorf("getting HEAD commit info: %w", commitErr)
	}

	// If in detached head, will be "HEAD", and fallback to use value from CI/CD system if possible.
	// Otherwise, the value will be like "refs/heads/master".
	headName := head.Name().String()
	if headName == "HEAD" && ciVars.BranchName != "" {
		headName = ciVars.BranchName
	}
	if headName != "HEAD" {
		m.Environment[backend.GitHeadName] = headName
	}

	// If there is no message set manually, default to the Git commit's title.
	msg := strings.TrimSpace(commit.Message)
	if msg == "" && ciVars.CommitMessage != "" {
		msg = ciVars.CommitMessage
	}
	if m.Message == "" {
		m.Message = gitCommitTitle(msg)
	}

	// Store committer and author information.
	m.Environment[backend.GitCommitter] = commit.Committer.Name
	m.Environment[backend.GitCommitterEmail] = commit.Committer.Email
	m.Environment[backend.GitAuthor] = commit.Author.Name
	m.Environment[backend.GitAuthorEmail] = commit.Author.Email

	// If the worktree is dirty, set a bit, as this could be a mistake.
	isDirty, err := isGitWorkTreeDirty(repoRoot)
	if err != nil {
		return fmt.Errorf("checking git worktree dirty state: %w", err)
	}
	m.Environment[backend.GitDirty] = strconv.FormatBool(isDirty)

	return nil
}

// gitCommitTitle turns a commit message into its title, simply by taking the first line.
func gitCommitTitle(s string) string {
	if ixCR := strings.Index(s, "\r"); ixCR != -1 {
		s = s[:ixCR]
	}
	if ixLF := strings.Index(s, "\n"); ixLF != -1 {
		s = s[:ixLF]
	}
	return s
}

// isGitWorkTreeDirty returns true if the work tree for the current directory's repository is dirty.
func isGitWorkTreeDirty(repoRoot string) (bool, error) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return false, err
	}

	gitStatusCmd := exec.Command(gitBin, "status", "--porcelain", "-z")
	var anyOutput anyWriter
	var stderr bytes.Buffer
	gitStatusCmd.Dir = repoRoot
	gitStatusCmd.Stdout = &anyOutput
	gitStatusCmd.Stderr = &stderr
	if err = gitStatusCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = stderr.Bytes()
		}
		return false, fmt.Errorf("'git status' failed: %w", err)
	}

	return bool(anyOutput), nil
}

// anyWriter is an io.Writer that will set itself to `true` iff any call to `anyWriter.Write` is made with a
// non-zero-length slice. This can be used to determine whether or not any data was ever written to the writer.
type anyWriter bool

func (w *anyWriter) Write(d []byte) (int, error) {
	if len(d) > 0 {
		*w = true
	}
	return len(d), nil
}
