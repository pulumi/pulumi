package auto

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type LocalWorkspace struct {
	workDir    string
	pulumiHome *string
	program    pulumi.RunFunc
}

var settingsExtensions = []string{".yaml", ".yml", ".json"}

func (l *LocalWorkspace) ProjectSettings() (*workspace.Project, error) {
	for _, ext := range settingsExtensions {
		projectPath := filepath.Join(l.WorkDir(), fmt.Sprintf("pulumi%s", ext))
		if _, err := os.Stat(projectPath); err != nil {
			proj, err := workspace.LoadProject(projectPath)
			if err != nil {
				return nil, errors.Wrap(err, "found project settings, but failed to load")
			}
			return proj, nil
		}
	}
	return nil, errors.New("unable to find project settings in workspace")
}

func (l *LocalWorkspace) WriteProjectSettings(settings *workspace.Project) error {
	pulumiYamlPath := filepath.Join(l.WorkDir(), "pulumi.yaml")
	return settings.Save(pulumiYamlPath)
}

func (l *LocalWorkspace) StackSettings(fqsn string) (*workspace.ProjectStack, error) {
	name, err := GetStackFromFQSN(fqsn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load stack settings, invalid stack name")
	}
	for _, ext := range settingsExtensions {
		stackPath := filepath.Join(l.WorkDir(), fmt.Sprintf("pulumi.%s%s", name, ext))
		if _, err := os.Stat(stackPath); err != nil {
			proj, err := workspace.LoadProjectStack(stackPath)
			if err != nil {
				return nil, errors.Wrap(err, "found stack settings, but failed to load")
			}
			return proj, nil
		}
	}
	return nil, errors.Errorf("unable to find stack settings in workspace for %s", fqsn)
}

func (l *LocalWorkspace) WriteStackSettings(fqsn string, settings *workspace.ProjectStack) error {
	name, err := GetStackFromFQSN(fqsn)
	if err != nil {
		return errors.Wrap(err, "failed to save stack settings, invalid stack name")
	}
	stackYamlPath := filepath.Join(l.WorkDir(), fmt.Sprintf("pulumi.%s.yaml:", name))
	err = settings.Save(stackYamlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to save stack setttings for %s", fqsn)
	}
	return nil
}

func (l *LocalWorkspace) SerializeArgsForOp(fqsn string) ([]string, error) {
	// not utilized for LocalWorkspace
	return nil, nil
}

func (l *LocalWorkspace) PostOpCallback(fqsn string) error {
	// not utilized for LocalWorkspace
	return nil
}

func (l *LocalWorkspace) GetConfig(string, config.Key) (config.Value, error) {
	var val config.Value
	// pulumi config key --json --show-secrets
	return val, nil
}

func (l *LocalWorkspace) GetAllConfig(string) (config.Map, error) {
	// pulumi config --json --show-secrets
	return nil, nil
}

func (l *LocalWorkspace) SetConfig(string, config.Key, config.Value) error {
	// pulumi config set --json
	return nil
}

func (l *LocalWorkspace) SetAllConfig(string, config.Map) error {
	// for each, pulumi config set --json
	return nil
}

func (l *LocalWorkspace) RefreshConfig(string) (config.Map, error) {
	// pulumi config refresh --force
	// l.GetAllConfig
	return nil, nil
}

func (l *LocalWorkspace) WorkDir() string {
	return l.workDir
}

func (l *LocalWorkspace) PulumiHome() *string {
	return l.pulumiHome
}

func (l *LocalWorkspace) Stack() string {
	// pulumi stack ls --json, followed by filter for current == true
	// pulumi whoami, also read pulumi.yaml to find project
	return ""
}

func (l *LocalWorkspace) CreateStack(string) (Stack, error) {
	// validate fqsn
	// pulumi stack init fqsn
	return nil, nil
}

func (l *LocalWorkspace) SelectStack(string) (Stack, error) {
	// validate fqsn
	// pulumi stack init fqsn
	return nil, nil
}

//
func (l *LocalWorkspace) ListStacks(string) ([]string, error) {
	// pulumi stack ls --json
	// pulumi whoami, read pulumi.yaml for project
	return nil, nil
}

func (l *LocalWorkspace) InstallPlugin(string, string) error {
	// pulumi plugin install resource str str
	return nil
}

func (l *LocalWorkspace) RemovePlugin(string, string) error {
	// pulumi plugin rm
	return nil
}

func (l *LocalWorkspace) ListPlugins() ([]workspace.PluginInfo, error) {
	// pulumi plugin ls
	return nil, nil
}

func (l *LocalWorkspace) runPulumiCmdSync(args ...string) (string, string, int, error) { /*set work dir, set pulumi home*/
	var env []string
	if l.PulumiHome() != nil {
		env = append(env, *l.PulumiHome())
	}
	return runPulumiCommandSync(l.WorkDir(), env, args...)
}

func NewLocalWorkspace(opts ...LocalWorkspaceOption) (Workspace, error) {
	lwOpts := &localWorkspaceOptions{}
	// for merging options, last specified value wins
	for _, opt := range opts {
		opt.applyLocalWorkspaceOption(lwOpts)
	}

	var workDir string

	if lwOpts.WorkDir != "" {
		workDir = lwOpts.WorkDir
	} else {
		dir, err := ioutil.TempDir("", "pulumi_auto")
		if err != nil {
			return nil, errors.Wrap(err, "unable to create tmp directory for workspace")
		}
		workDir = dir
	}

	if lwOpts.Repo != nil {
		// now do the git clone
		projDir, err := setupGitRepo(workDir, lwOpts.Repo)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace, unable to enlist in git repo")
		}
		workDir = projDir
	}

	var program pulumi.RunFunc
	if lwOpts.Program != nil {
		program = lwOpts.Program
	}

	var pulumiHome *string
	if lwOpts.PulumiHome != nil {
		pulumiHome = lwOpts.PulumiHome
	}

	l := &LocalWorkspace{
		workDir:    workDir,
		program:    program,
		pulumiHome: pulumiHome,
	}

	if lwOpts.Project != nil {
		err := l.WriteProjectSettings(lwOpts.Project)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace, unable to save project settings")
		}
	}

	for fqsn, ss := range lwOpts.Stacks {
		err := l.WriteStackSettings(fqsn, &ss)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create workspace")
		}
	}

	return l, nil
}

type localWorkspaceOptions struct {
	// WorkDir is the directory to execute commands from and store state.
	// Defaults to a tmp dir.
	WorkDir string
	// Program is the Pulumi Program to execute. If none is supplied,
	// the program identified in $WORKDIR/pulumi.yaml will be used instead.
	Program pulumi.RunFunc
	// PulumiHome overrides the metadata directory for pulumi commands
	PulumiHome *string
	// Project is the project settings for the workspace
	Project *workspace.Project
	// Stacks is a map of [fqsn -> stack settings objects] to seed the workspace
	Stacks map[string]workspace.ProjectStack
	// Repo is a git repo with a Pulumi Project to clone into the WorkDir.
	Repo *GitRepo
}

type LocalWorkspaceOption interface {
	applyLocalWorkspaceOption(*localWorkspaceOptions)
}

type localWorkspaceOption func(*localWorkspaceOptions)

func (o localWorkspaceOption) applyLocalWorkspaceOption(opts *localWorkspaceOptions) {
	o(opts)
}

type GitRepo struct {
	// URL to clone git repo
	URL string
	// Optional path relative to the repo root specifying location of the pulumi program.
	// Specifying this option will update the Worspace's WorkDir accordingly.
	ProjectPath string
	// Optional branch to checkout
	Branch string
	// Optional commit to checkout
	CommitHash string
	// Optional function to execute after enlisting in the specified repo.
	Setup SetupFn
}

// SetupFn is a function to execute after enlisting in a git repo.
// It is called with a PATH containing the pulumi program post-enlistment.
type SetupFn func(string) error

// WorkDir is the directory to execute commands from and store state.
func WorkDir(workDir string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.WorkDir = workDir
	})
}

// Program is the Pulumi Program to execute.
func Program(program pulumi.RunFunc) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Program = program
	})
}

// PulumiHome overrides the metadata directory for pulumi commands
func PulumiHome(dir string) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.PulumiHome = &dir
	})
}

// Project sets project settings for the workspace
func Project(settings workspace.Project) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Project = &settings
	})
}

// Stacks is a list of stack settings objects to seed the workspace
func Stacks(settings map[string]workspace.ProjectStack) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Stacks = settings
	})
}

// GitURL is a git repo with a Pulumi Project to clone into the WorkDir.
func GitURL(gitRepo GitRepo) LocalWorkspaceOption {
	return localWorkspaceOption(func(lo *localWorkspaceOptions) {
		lo.Repo = &gitRepo
	})
}

func GetStackFromFQSN(fqsn string) (string, error) {
	if err := ValidateFullyQualifiedStackName(fqsn); err != nil {
		return "", err
	}
	return strings.Split(fqsn, "/")[2], nil
}

func ValidateFullyQualifiedStackName(fqsn string) error {
	parts := strings.Split(fqsn, "/")
	if len(parts) != 3 {
		return errors.Errorf(
			"invalid fully qualified stack name: %s, expected in the form 'org/project/stack'",
			fqsn,
		)
	}
	return nil
}
