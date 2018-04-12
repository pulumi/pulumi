// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package backend

import (
	"regexp"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Stack is a stack associated with a particular backend implementation.
type Stack interface {
	Name() tokens.QName         // this stack's name.
	Config() config.Map         // the current config map.
	Snapshot() *deploy.Snapshot // the latest deployment snapshot.
	Backend() Backend           // the backend this stack belongs to.

	// Update this stack.
	Update(proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error
	// Destroy this stack's resources.
	Destroy(proj *workspace.Project, root string,
		m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error

	Remove(force bool) (bool, error)                                  // remove this stack.
	GetLogs(query operations.LogQuery) ([]operations.LogEntry, error) // list log entries for this stack.
	ExportDeployment() (*apitype.UntypedDeployment, error)            // export this stack's deployment.
	ImportDeployment(deployment *apitype.UntypedDeployment) error     // import the given deployment into this stack.
}

// RemoveStack returns the stack, or returns an error if it cannot.
func RemoveStack(s Stack, force bool) (bool, error) {
	return s.Backend().RemoveStack(s.Name(), force)
}

// UpdateStack updates the target stack with the current workspace's contents (config and code).
func UpdateStack(s Stack, proj *workspace.Project, root string,
	m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error {
	return s.Backend().Update(s.Name(), proj, root, m, opts, displayOpts)
}

// DestroyStack destroys all of this stack's resources.
func DestroyStack(s Stack, proj *workspace.Project, root string,
	m UpdateMetadata, opts engine.UpdateOptions, displayOpts DisplayOptions) error {
	return s.Backend().Destroy(s.Name(), proj, root, m, opts, displayOpts)
}

// GetStackCrypter fetches the encrypter/decrypter for a stack.
func GetStackCrypter(s Stack) (config.Crypter, error) {
	return s.Backend().GetStackCrypter(s.Name())
}

// GetStackLogs fetches a list of log entries for the current stack in the current backend.
func GetStackLogs(s Stack, query operations.LogQuery) ([]operations.LogEntry, error) {
	return s.Backend().GetLogs(s.Name(), query)
}

// ExportStackDeployment exports the given stack's deployment as an opaque JSON message.
func ExportStackDeployment(s Stack) (*apitype.UntypedDeployment, error) {
	return s.Backend().ExportDeployment(s.Name())
}

// ImportStackDeployment imports the given deployment into the indicated stack.
func ImportStackDeployment(s Stack, deployment *apitype.UntypedDeployment) error {
	return s.Backend().ImportDeployment(s.Name(), deployment)
}

// GetStackTags returns the set of tags for the "current" stack, based on the environment
// and Pulumi.yaml file.
func GetStackTags() (map[apitype.StackTagName]string, error) {
	tags := make(map[apitype.StackTagName]string)

	// Tags based on the workspace's repository.
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}
	repo := w.Repository()
	if repo != nil {
		tags[apitype.GitHubOwnerNameTag] = repo.Owner
		tags[apitype.GitHubRepositoryNameTag] = repo.Name
	}

	// Tags based on Pulumi.yaml.
	projPath, err := workspace.DetectProjectPath()
	if err != nil {
		return nil, err
	}
	if projPath != "" {
		proj, err := workspace.LoadProject(projPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error loading project %q", projPath)
		}
		tags[apitype.ProjectNameTag] = proj.Name.String()
		tags[apitype.ProjectRuntimeTag] = proj.Runtime
		if proj.Description != nil {
			tags[apitype.ProjectDescriptionTag] = *proj.Description
		}
	}

	return tags, nil
}

// ValidateStackProperties validates the stack name and its tags to confirm they adhear to various
// naming and length restrictions.
func ValidateStackProperties(stack string, tags map[apitype.StackTagName]string) error {
	// Validate a name, used for both stack and project names since they use the same regex.
	validateName := func(ty, name string) error {
		// Must be alphanumeric, dash, or underscore. Must begin an alphanumeric. Must be between 3 and 100 chars.
		stackNameRE := regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-_]{1,98}[a-zA-Z0-9]$")
		if !stackNameRE.MatchString(name) {
			if len(name) < 3 || len(name) > 100 {
				return errors.Errorf("%s name must be between 3 and 100 characters long", ty)
			}
			first, last := name[0], name[len(name)-1]
			if first == '-' || first == '_' || last == '-' || last == '_' {
				return errors.Errorf("%s name begin and end with an alphanumeric", ty)
			}
			return errors.Errorf("%s name can only contain alphanumeric, hyphens, or underscores", ty)
		}
		return nil
	}
	if err := validateName("stack", stack); err != nil {
		return err
	}

	// Tags must all be shorter than a given length, and may have other tag-specific restrictions.
	// These values are enforced by the Pulumi Service, but we validate them here to have a better
	// error experience.
	const maxTagName = 40
	const maxTagValue = 256
	for t, v := range tags {
		switch t {
		case apitype.ProjectNameTag, apitype.ProjectRuntimeTag:
			if err := validateName("project", v); err != nil {
				return err
			}
		}

		if len(t) > maxTagName {
			return errors.Errorf("stack tag %q is too long (max length 40 characters)", t)
		}
		if len(v) > maxTagValue {
			return errors.Errorf("stack tag %q value is too long (max length 255 characters)", t)
		}
	}

	return nil
}
