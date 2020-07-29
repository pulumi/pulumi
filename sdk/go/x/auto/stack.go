// Package auto contains the Pulumi Automation API, the programmatic interface for driving Pulumi programs
// without the CLI.
// Generally this can be thought of as encapsulating the functionality of the CLI
// (`pulumi up`, `pulumi preview`, pulumi destroy`, `pulumi stack init`, etc.). This still requires a
// CLI binary to be installed and available on your $PATH. The Automation API
// can operate on programs in three forms:
//
// 1. Local, on-disk programs: a stand alone Pulumi program on your local filesystem.
// Specified via `StackSpec.Project.SourcePath`.
//
// 2. Remote programs: a git URL and subdirectory containing a pulumi program. Specified via `StackSpec.Project.Remote`
//
// 3. Inline: A pulumi program embedded within your Automation API program. Enables defining a single main()
// func that describes and drives a pulumi program. Specified via `StackSpec.Project.InlineSource`
//
// The Automation API provides a natural way to orchestrate multiple stacks,
// feeding the output of one stack as an input to the next as shown in the example below.
// The package can be used for a number of use cases:
//
// 	- Driving pulumi deployments within CI/CD workflows
//
// 	- Integration testing
//
// 	- Multi-stage deployments such as blue-green deployment patterns
//
// 	- Deployments involving application code like database migrations
//
// 	- Building higher level tools, custom CLIs over pulumi, etc.
package auto

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

// Stack is a deployable instance of a Pulumi program, containing it's own unique configuration.
// It defines a common set of operations that can be performed across all three stack types
// (SourceRoot, Remote, InlineSource).
//
// Lifecycle Operations:
//
// - Up: create or update resources.
//
// - Preview: dry-run execution to see a proposed set of changes.
//
// - Refresh: sync the state of your stack with that of the cloud providers.
//
// - Destroy: delete all resources in a stack.
//
// - Remove: delete a stack, all associated history, and configuration.
//
//
// Status Operations:
//
// - Summary: retrieve information about the last lifecycle operation's result.
//
// - Outputs: get both secret and plaintext stack outputs.
//
// - User: get the current authenticated Pulumi username.
//
//
// Config Operations:
//
// - SetConfig: upsert the specified plaintext config values.
//
// - SetScrets: upsert the specified secret configuration values (will be stored encrypted per stack settings).
//
type Stack interface {
	// -- Lifecycle

	// Up creates or updates the resources in a stack.
	// https://www.pulumi.com/docs/reference/cli/pulumi_up/
	Up() (UpResult, error)
	// Preview preforms a dry-run update to a stack, returning pending changes.
	// https://www.pulumi.com/docs/reference/cli/pulumi_preview/
	Preview() (PreviewResult, error)
	// Refresh refreshes the resources in a stack, reading the state of the world directly from cloud providers.
	// https://www.pulumi.com/docs/reference/cli/pulumi_refresh/
	Refresh() (RefreshResult, error)
	// Destroy deletes all resources in a stack.
	// https://www.pulumi.com/docs/reference/cli/pulumi_destroy/
	Destroy() (DestroyResult, error)
	// Remove removes a stack, its configuration, and all associated history
	// https://www.pulumi.com/docs/reference/cli/pulumi_stack_rm/
	Remove() error

	// -- Status

	// Summary returns information about the last update on the stack.
	Summary() (UpdateSummary, error)
	// Outputs returns the current plaintext and secret stack outputs.
	Outputs() (map[string]interface{}, map[string]interface{}, error)
	// User returns the current identity associated with the ambient $PULUMI_ACCESS_TOKEN.
	User() (string, error)

	// -- Config

	// SetConfig sets (upsert) the specified config values
	SetConfig(map[string]string) error
	// SetSecrets sets (upsert) the specified secret config values, encrypted per settings in
	// `Pulumi.<stack>.yaml` or `StackSpec.Overrides.ProjectStack`
	SetSecrets(map[string]string) error

	// -- marker method
	isStack()
}

// NewStack creates a Stack for deployment and other operations.
// It will select an existing matching stack if available before creating a new instance.
// It will merge configuration with existing Pulumi.yaml, Pulumi.<stack>.yaml
// if available in a `Source` or `Remote` project.
// Sets config and secret values if provided.
func NewStack(ss StackSpec) (Stack, error) {
	err := ss.validate()
	if err != nil {
		return nil, err
	}

	if ss.Project.Remote != nil {
		p, err := setupRemote(ss.Project.Remote)
		ss.Project.SourcePath = p
		if err != nil {
			return nil, errors.Wrap(err, "unable to enlist and setup remote")
		}
	} else if ss.Project.InlineSource != nil {
		projDir, err := ioutil.TempDir("", "auto")
		if err != nil {
			return nil, errors.Wrap(err, "unable to create tmpdir for inline source")
		}
		ss.Project.SourcePath = projDir
		ss.setInlineDefaults()
	}

	s := &stack{
		Name:         ss.Name,
		ProjectName:  ss.Project.Name,
		SourcePath:   ss.Project.SourcePath,
		InlineSource: ss.Project.InlineSource,
	}

	err = ss.writeProject()
	if err != nil {
		return nil, err
	}

	err = s.initOrSelectStack()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize or select stack")
	}

	err = ss.writeStack()
	if err != nil {
		return nil, err
	}
	var config map[string]string
	var secrets map[string]string
	if ss.Overrides != nil {
		config = ss.Overrides.Config
		secrets = ss.Overrides.Secrets
	}
	err = s.setConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "unable to set config")
	}

	err = s.setSecrets(secrets)
	if err != nil {
		return nil, errors.Wrap(err, "unable to set secrets")
	}

	return s, nil
}

// StackSpec is a description of a Pulumi stack
type StackSpec struct {
	// Name of the the stack
	Name string
	// Project is a description of the project to execute
	Project ProjectSpec
	// Overrides is an optional set of values to overwrite in pulumi.<stack>.yaml
	Overrides *StackOverrides
}

// ProjectSpec is a description of a Project project and corresponding source code
//
// Source code for a Pulumi Project is specified in one of three variants:
//
// 1. Local, on-disk programs: a stand alone Pulumi program on your local filesystem.
// Specified via `StackSpec.Project.SourcePath`.
//
// 2. Remote programs: a git URL and subdirectory containing a pulumi program. Specified via `StackSpec.Project.Remote`
//
// 3. Inline: A pulumi program embedded within your Automation API program. Enables defining a single main()
// func that describes and drives a pulumi program. Specified via `StackSpec.Project.InlineSource`
//
type ProjectSpec struct {
	// Name of the project
	Name string

	// (Local program) source directory containing the pulumi program
	SourcePath string

	// (Remote program) information to clone and setup a git hosted pulumi program
	Remote *RemoteArgs

	// (Inline Program) the Pulumi program as an inline function
	InlineSource pulumi.RunFunc

	// Optional set of values to upsert into existing pulumi.yaml
	Overrides *ProjectOverrides
}

type RemoteArgs struct {
	// Git URL used to fetch the repository.
	RepoURL string

	// Optional path relative to the repo root specifying location of the pulumi program
	ProjectPath *string
	// Optional branch to checkout
	Branch *string
	// Optional commit to checkout
	CommitHash *string
	// Optional function to execute after enlisting in the specified repo.
	Setup SetupFn
	// Optional directory to enlist repo, defaults to using ioutil.TempDir.
	WorkDir *string
}

// SetupFn is a function to execute after enlisting in a Stack's remote repo.
// It is called with a PATH containing the pulumi program post-enlistment.
type SetupFn func(string) error

// ProjectOverrides is an optional set of values to be merged with
// the existing pulumi.yaml
type ProjectOverrides struct {
	// Replace controls merge behavior with existing Pulumi.yaml files
	Replace bool
	Project *workspace.Project
}

// StackOverrides is an optional set of values to be merged with
// the existing pulumi.<stackName>.yaml
type StackOverrides struct {
	// Replace controls merge behavior with existing stack.yaml files.
	Replace bool
	// Config is an optional config bag to `pulumi config set`
	Config map[string]string
	// Secrets is an optional config bag to `pulumi config set --secret`
	Secrets map[string]string
	// TODO we should use a limited struct that prevents setting config directly
	// We want users to explicitly handle config/secrets through above param
	// ProjectStack is the optional set of overrides
	ProjectStack *workspace.ProjectStack
}

func (ss *StackSpec) validate() error {
	if ss.Name == "" {
		return errors.New("missing stack name")
	}
	if ss.Project.Name == "" {
		return errors.New("missing project name")
	}
	if ss.Project.Overrides != nil && ss.Project.Overrides.Project != nil && ss.Project.Overrides.Project.Name != "" {
		if ss.Project.Overrides.Project.Name.String() != ss.Project.Name {
			return errors.New("`Project.Name` does not match Name specified in `Project.Overrides`")
		}
	}
	if ss.Project.SourcePath == "" && ss.Project.Remote == nil && ss.Project.InlineSource == nil {
		return errors.New(
			"must specify one of `ProjectSpec.Source`, `ProjectSpec.Remote`, or `ProjectSpec.InlineSource`",
		)
	}
	if ss.Project.Remote != nil {
		if ss.Project.SourcePath != "" {
			return errors.New(
				"`ProjectSpec.Source` and `ProjectSpec.Remote` are mutually exclusive, but both were provided",
			)
		}
		if ss.Project.InlineSource != nil {
			return errors.New(
				"`ProjectSpec.InlineSource` and `ProjectSpec.Remote` are mutually exclusive, but both were provided",
			)
		}
		if ss.Project.Remote.RepoURL == "" {
			return errors.New("Missing git source URL `Project.Remote.RepoURL`")
		}
	}
	if ss.Project.InlineSource != nil {
		if ss.Project.SourcePath != "" {
			return errors.New(
				"`ProjectSpec.Source` and `ProjectSpec.InlineSource` are mutually exclusive, but both were provided",
			)
		}
	}
	return nil
}

// inline projects shouldn't require a pulumi.yaml from a user perspective
// but they do from an implementation perspective. Ensure the required fields
// are set on behalf of the user.
func (ss *StackSpec) setInlineDefaults() {
	if ss.Project.Overrides == nil {
		ss.Project.Overrides = &ProjectOverrides{}
	}

	if ss.Project.Overrides.Project == nil {
		ss.Project.Overrides.Project = &workspace.Project{}
	}
	ss.Project.Overrides.Project.Name = tokens.PackageName(ss.Project.Name)
	ss.Project.Overrides.Project.Runtime = workspace.NewProjectRuntimeInfo(
		"go", ss.Project.Overrides.Project.Runtime.Options(),
	)
}

type stack struct {
	Name         string
	ProjectName  string
	SourcePath   string
	InlineSource pulumi.RunFunc
}

func (s *stack) isStack() {}
