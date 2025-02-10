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

package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/texttheater/golang-levenshtein/levenshtein"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

const (
	// This file will be ignored when copying from the template cache to
	// a project directory.
	legacyPulumiTemplateManifestFile = ".pulumi.template.yaml"

	// pulumiLocalTemplatePathEnvVar is a path to the folder where templates are stored.
	// It is used in sandboxed environments where the classic template folder may not be writable.
	pulumiLocalTemplatePathEnvVar = "PULUMI_TEMPLATE_PATH"

	// pulumiLocalPolicyTemplatePathEnvVar is a path to the folder where policy templates are stored.
	// It is used in sandboxed environments where the classic template folder may not be writable.
	pulumiLocalPolicyTemplatePathEnvVar = "PULUMI_POLICY_TEMPLATE_PATH"
)

// These are variables instead of constants in order that they can be set using the `-X`
// `ldflag` at build time, if necessary.
var (
	// The Git URL for Pulumi program templates
	pulumiTemplateGitRepository = "https://github.com/pulumi/templates.git"
	// The branch name for the template repository
	pulumiTemplateBranch = "master"
	// The Git URL for Pulumi Policy Pack templates
	pulumiPolicyTemplateGitRepository = "https://github.com/pulumi/templates-policy.git"
	// The branch name for the policy pack template repository
	pulumiPolicyTemplateBranch = "master"
)

// TemplateKind describes the form of a template.
type TemplateKind int

const (
	// TemplateKindPulumiProject is a template for a Pulumi stack.
	TemplateKindPulumiProject TemplateKind = 0

	// TemplateKindPolicyPack is a template for a Policy Pack.
	TemplateKindPolicyPack TemplateKind = 1
)

// TemplateRepository represents a repository of templates.
type TemplateRepository struct {
	Root         string // The full path to the root directory of the repository.
	SubDirectory string // The full path to the sub directory within the repository.
	ShouldDelete bool   // Whether the root directory should be deleted.
}

// Delete deletes the template repository.
func (repo TemplateRepository) Delete() error {
	if repo.ShouldDelete {
		return os.RemoveAll(repo.Root)
	}
	return nil
}

// Templates lists the templates in the repository.
func (repo TemplateRepository) Templates() ([]Template, error) {
	path := repo.SubDirectory

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// If it's a file, look in its directory.
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	// See if there's a Pulumi.yaml in the directory.
	template, err := LoadTemplate(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	} else if err == nil {
		return []Template{template}, nil
	}

	// Otherwise, read all subdirectories to find the ones
	// that contain a Pulumi.yaml.
	infos, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []Template
	for _, info := range infos {
		if info.IsDir() {
			name := info.Name()

			// Ignore the .git directory.
			if name == GitDir {
				continue
			}

			template, err := LoadTemplate(filepath.Join(path, name))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				logging.V(2).Infof(
					"Failed to load template %s: %s",
					name, err,
				)
				result = append(result, Template{Name: name, Error: err})
			} else if err == nil {
				result = append(result, template)
			}
		}
	}
	return result, nil
}

// PolicyTemplates lists the policy templates in the repository.
func (repo TemplateRepository) PolicyTemplates() ([]PolicyPackTemplate, error) {
	path := repo.SubDirectory

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// If it's a file, look in its directory.
	if !info.IsDir() {
		path = filepath.Dir(path)
	}

	// See if there's a PulumiPolicy.yaml in the directory.
	template, err := LoadPolicyPackTemplate(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	} else if err == nil {
		return []PolicyPackTemplate{template}, nil
	}

	// Otherwise, read all subdirectories to find the ones
	// that contain a PulumiPolicy.yaml.
	infos, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []PolicyPackTemplate
	for _, info := range infos {
		if info.IsDir() {
			name := info.Name()

			// Ignore the .git directory.
			if name == GitDir {
				continue
			}

			template, err := LoadPolicyPackTemplate(filepath.Join(path, name))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				logging.V(2).Infof(
					"Failed to load template %s: %s",
					name, err,
				)
				result = append(result, PolicyPackTemplate{Name: name, Error: err})
			} else if err == nil {
				result = append(result, template)
			}
		}
	}
	return result, nil
}

// Template represents a project template.
type Template struct {
	Dir         string                                // The directory containing Pulumi.yaml.
	Name        string                                // The name of the template.
	Description string                                // Description of the template.
	Quickstart  string                                // Optional text to be displayed after template creation.
	Config      map[string]ProjectTemplateConfigValue // Optional template config.
	Error       error                                 // Non-nil if the template is broken.

	ProjectName        string // Name of the project.
	ProjectDescription string // Optional description of the project.
}

// Errored returns if the template has an error
func (t Template) Errored() bool {
	return t.Error != nil
}

// PolicyPackTemplate represents a Policy Pack template.
type PolicyPackTemplate struct {
	Dir         string // The directory containing PulumiPolicy.yaml.
	Name        string // The name of the template.
	Description string // Description of the template.
	Error       error  // Non-nil if the template is broken.
}

// Errored returns if the template has an error
func (t PolicyPackTemplate) Errored() bool {
	return t.Error != nil
}

// cleanupLegacyTemplateDir deletes an existing ~/.pulumi/templates directory if it isn't a git repository.
func cleanupLegacyTemplateDir(templateKind TemplateKind) error {
	templateDir, err := GetTemplateDir(templateKind)
	if err != nil {
		return err
	}

	// See if the template directory is a Git repository.
	repo, err := git.PlainOpen(templateDir)
	if err != nil {
		// If the repository doesn't exist, it's a legacy directory.
		// Delete the entire template directory and all children.
		if err == git.ErrRepositoryNotExists {
			return os.RemoveAll(templateDir)
		}

		return err
	}

	// The template directory is a Git repository. We want to make sure that it has the same remote as the one that
	// we want to pull from. If it doesn't have the same remote, we'll delete it, so that the clone later succeeds.
	// Select the appropriate remote
	var url string
	if templateKind == TemplateKindPolicyPack {
		url = pulumiPolicyTemplateGitRepository
	} else {
		url = pulumiTemplateGitRepository
	}
	remotes, err := repo.Remotes()
	if err != nil {
		return fmt.Errorf("getting template repo remotes: %w", err)
	}
	// If the repo exists and it doesn't have exactly one remote that matches our URL, wipe the templates directory.
	if len(remotes) != 1 || remotes[0] == nil || !strings.Contains(remotes[0].String(), url) {
		return os.RemoveAll(templateDir)
	}

	return nil
}

// IsTemplateURL returns true if templateNamePathOrURL starts with "https://" (SSL) or "git@" (SSH).
func IsTemplateURL(templateNamePathOrURL string) bool {
	// Normalize the provided URL so we can check its scheme. This will
	// correctly return false in the case where the URL doesn't parse cleanly.
	url, _, _ := gitutil.ParseGitRepoURL(templateNamePathOrURL)
	return strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "ssh://")
}

// isTemplateFileOrDirectory returns true if templateNamePathOrURL is the name of a valid file or directory.
func isTemplateFileOrDirectory(templateNamePathOrURL string) bool {
	_, err := os.Stat(templateNamePathOrURL)
	return err == nil
}

// RetrieveTemplates retrieves a "template repository" based on the specified name, path, or URL.
func RetrieveTemplates(ctx context.Context, templateNamePathOrURL string, offline bool,
	templateKind TemplateKind,
) (TemplateRepository, error) {
	if isZIPTemplateURL(templateNamePathOrURL) {
		return RetrieveZIPTemplates(templateNamePathOrURL)
	}
	if IsTemplateURL(templateNamePathOrURL) {
		return retrieveURLTemplates(ctx, templateNamePathOrURL, offline, templateKind)
	}
	if isTemplateFileOrDirectory(templateNamePathOrURL) {
		return retrieveFileTemplates(templateNamePathOrURL)
	}
	return retrievePulumiTemplates(ctx, templateNamePathOrURL, offline, templateKind)
}

// retrieveURLTemplates retrieves the "template repository" at the specified URL.
func retrieveURLTemplates(
	ctx context.Context, rawurl string, offline bool, templateKind TemplateKind,
) (TemplateRepository, error) {
	if offline {
		return TemplateRepository{}, fmt.Errorf("cannot use %s offline", rawurl)
	}

	var err error

	// Create a temp dir.
	var temp string
	if temp, err = os.MkdirTemp("", "pulumi-template-"); err != nil {
		return TemplateRepository{}, err
	}

	var fullPath string
	if fullPath, err = RetrieveGitFolder(ctx, rawurl, temp); err != nil {
		return TemplateRepository{}, fmt.Errorf("failed to retrieve git folder: %w", err)
	}

	return TemplateRepository{
		Root:         temp,
		SubDirectory: fullPath,
		ShouldDelete: true,
	}, nil
}

// retrieveFileTemplates points to the "template repository" at the specified location in the file system.
func retrieveFileTemplates(path string) (TemplateRepository, error) {
	return TemplateRepository{
		Root:         path,
		SubDirectory: path,
		ShouldDelete: false,
	}, nil
}

// retrievePulumiTemplates retrieves the "template repository" for Pulumi templates.
// Instead of retrieving to a temporary directory, the Pulumi templates are managed from
// ~/.pulumi/templates.
func retrievePulumiTemplates(
	ctx context.Context, templateName string, offline bool, templateKind TemplateKind,
) (TemplateRepository, error) {
	templateName = strings.ToLower(templateName)

	// Cleanup the template directory.
	if err := cleanupLegacyTemplateDir(templateKind); err != nil {
		return TemplateRepository{}, err
	}

	// Get the template directory.
	templateDir, err := GetTemplateDir(templateKind)
	if err != nil {
		return TemplateRepository{}, err
	}

	// Ensure the template directory exists.
	if err := os.MkdirAll(templateDir, 0o700); err != nil {
		return TemplateRepository{}, err
	}

	if !offline {
		// Clone or update the pulumi/templates repo.
		repo := pulumiTemplateGitRepository
		branch := plumbing.NewBranchReferenceName(pulumiTemplateBranch)
		if templateKind == TemplateKindPolicyPack {
			repo = pulumiPolicyTemplateGitRepository
			branch = plumbing.NewBranchReferenceName(pulumiPolicyTemplateBranch)
		}
		err := gitutil.GitCloneOrPull(ctx, repo, branch, templateDir, false /*shallow*/)
		if err != nil {
			return TemplateRepository{}, fmt.Errorf("cloning templates repo: %w", err)
		}
	}

	subDir := templateDir
	if templateName != "" {
		subDir = filepath.Join(subDir, templateName)

		// Provide a nicer error message when the template can't be found (dir doesn't exist).
		_, err := os.Stat(subDir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return TemplateRepository{}, newTemplateNotFoundError(templateDir, templateName)
			}
			contract.IgnoreError(err)
		}
	}

	return TemplateRepository{
		Root:         templateDir,
		SubDirectory: subDir,
		ShouldDelete: false,
	}, nil
}

// RetrieveGitFolder downloads the repo to path and returns the full path on disk.
func RetrieveGitFolder(ctx context.Context, rawurl string, path string) (string, error) {
	url, urlPath, err := gitutil.ParseGitRepoURL(rawurl)
	if err != nil {
		return "", err
	}

	ref, commit, subDirectory, err := gitutil.GetGitReferenceNameOrHashAndSubDirectory(url, urlPath)
	if err != nil {
		return "", fmt.Errorf("failed to get git ref: %w", err)
	}
	logging.V(10).Infof(
		"Attempting to fetch from %s at commit %s@%s for subdirectory '%s'",
		url, ref, commit, subDirectory)

	if ref != "" {
		// Different reference attempts to cycle through
		// We default to master then main in that order. We need to order them to avoid breaking
		// already existing processes for repos that already have a master and main branch.
		refAttempts := []plumbing.ReferenceName{plumbing.Master, plumbing.NewBranchReferenceName("main")}

		if ref != plumbing.HEAD {
			// If we have a non-default reference, we just use it
			refAttempts = []plumbing.ReferenceName{ref}
		}

		var cloneErrs []error
		for _, ref := range refAttempts {
			// Attempt the clone. If it succeeds, break
			err := gitutil.GitCloneOrPull(ctx, url, ref, path, true /*shallow*/)
			if err == nil {
				break
			}
			logging.V(10).Infof("Failed to clone %s@%s: %v", url, ref, err)
			cloneErrs = append(cloneErrs, fmt.Errorf("ref '%s': %w", ref, err))
		}
		if len(cloneErrs) == len(refAttempts) {
			return "", fmt.Errorf("failed to clone %s: %w", rawurl, errors.Join(cloneErrs...))
		}
	} else {
		if cloneErr := gitutil.GitCloneAndCheckoutCommit(ctx, url, commit, path); cloneErr != nil {
			logging.V(10).Infof("Failed to clone %s@%s: %v", url, commit, err)
			return "", fmt.Errorf("failed to clone and checkout %s(%s): %w", url, commit, cloneErr)
		}
	}

	// Verify the sub directory exists.
	fullPath := filepath.Join(path, filepath.FromSlash(subDirectory))
	logging.V(10).Infof("Cloned %s at commit %s@%s to %s", url, ref, commit, fullPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		logging.V(10).Infof("Failed to stat %s after cloning %s: %v", fullPath, url, err)
		return "", err
	}
	if !info.IsDir() {
		logging.V(10).Infof("%s was not a directory after cloning %s: %v", fullPath, url, err)
		return "", fmt.Errorf("%s is not a directory", fullPath)
	}

	return fullPath, nil
}

// LoadTemplate returns a template from a path.
func LoadTemplate(path string) (Template, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Template{}, err
	}
	if !info.IsDir() {
		return Template{}, fmt.Errorf("%s is not a directory", path)
	}

	// TODO handle other extensions like Pulumi.yml and Pulumi.json?
	proj, err := LoadProject(filepath.Join(path, "Pulumi.yaml"))
	if err != nil {
		return Template{}, err
	}

	template := Template{
		Dir:  path,
		Name: filepath.Base(path),

		ProjectName: proj.Name.String(),
	}
	if proj.Template != nil {
		template.Description = proj.Template.Description
		template.Quickstart = proj.Template.Quickstart
		template.Config = proj.Template.Config
	}
	if proj.Description != nil {
		template.ProjectDescription = *proj.Description
	}

	return template, nil
}

// CopyTemplateFilesDryRun does a dry run of copying a template to a destination directory,
// to ensure it won't overwrite any files.
func CopyTemplateFilesDryRun(sourceDir, destDir, projectName string) error {
	var existing []string
	if err := walkFiles(sourceDir, destDir, projectName,
		func(entry os.DirEntry, source string, dest string) error {
			if destInfo, statErr := os.Stat(dest); statErr == nil && !destInfo.IsDir() {
				existing = append(existing, filepath.Base(dest))
			}
			return nil
		}); err != nil {
		return err
	}

	if len(existing) > 0 {
		return newExistingFilesError(existing)
	}
	return nil
}

func toYAMLString(value string) (string, error) {
	byts, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(byts), nil
}

// CopyTemplateFiles does the actual copy operation to a destination directory.
func CopyTemplateFiles(
	sourceDir, destDir string, force bool, projectName string, projectDescription string,
) error {
	// Needed for stringifying numeric user-provided project names.
	yamlName, err := toYAMLString(projectName)
	if err != nil {
		return err
	}

	// Needed for escaping special characters in user-provided descriptions.
	yamlDescription, err := toYAMLString(projectDescription)
	if err != nil {
		return err
	}

	return walkFiles(sourceDir, destDir, projectName,
		func(entry os.DirEntry, source string, dest string) error {
			if entry.IsDir() {
				// Create the destination directory.
				if force {
					info, _ := os.Stat(dest)
					if info != nil && !info.IsDir() {
						os.Remove(dest)
					}
					// MkdirAll will not error out if dest is a directory that already exists
					return os.MkdirAll(dest, 0o700)
				}
				return os.Mkdir(dest, 0o700)
			}

			// Read the source file.
			b, err := os.ReadFile(source)
			if err != nil {
				return err
			}

			// Transform only if it isn't a binary file.
			result := b
			if !isBinary(b) {
				name, description := projectName, projectDescription
				if strings.HasSuffix(source, ".yaml") {
					// Use the sanitized project name and description for the Pulumi.yaml file.
					name = yamlName
					description = yamlDescription
				}
				transformed := transform(string(b), name, description)
				result = []byte(transformed)
			}

			// Originally we just wrote in 0600 mode, but
			// this does not preserve the executable bit.
			// With the new logic below, we try to be at
			// least as permissive as 0600 and whatever
			// permissions the source file or symlink had.
			var mode os.FileMode
			sourceStat, err := os.Lstat(source)
			if err != nil {
				return err
			}
			mode = sourceStat.Mode().Perm() | 0o600

			// Write to the destination file.
			err = writeAllBytes(dest, result, force, mode)
			if err != nil {
				// An existing file has shown up in between the dry run and the actual copy operation.
				if os.IsExist(err) {
					return newExistingFilesError([]string{filepath.Base(dest)})
				}
			}
			return err
		})
}

// LoadPolicyPackTemplate returns a Policy Pack template from a path.
func LoadPolicyPackTemplate(path string) (PolicyPackTemplate, error) {
	info, err := os.Stat(path)
	if err != nil {
		return PolicyPackTemplate{}, err
	}
	if !info.IsDir() {
		return PolicyPackTemplate{}, fmt.Errorf("%s is not a directory", path)
	}

	pack, err := LoadPolicyPack(filepath.Join(path, "PulumiPolicy.yaml"))
	if err != nil {
		return PolicyPackTemplate{}, err
	}
	policyPackTemplate := PolicyPackTemplate{
		Dir:  path,
		Name: filepath.Base(path),
	}
	if pack.Description != nil {
		policyPackTemplate.Description = *pack.Description
	}

	return policyPackTemplate, nil
}

// GetTemplateDir returns the directory in which templates on the current machine are stored.
func GetTemplateDir(templateKind TemplateKind) (string, error) {
	envVar := pulumiLocalTemplatePathEnvVar
	if templateKind == TemplateKindPolicyPack {
		envVar = pulumiLocalPolicyTemplatePathEnvVar
	}
	// Allow the folder we use to store templates to be overridden.
	dir := os.Getenv(envVar)
	if dir != "" {
		return dir, nil
	}

	// If Policy Pack template and there is no override, then return the classic policy template directory.
	if templateKind == TemplateKindPolicyPack {
		return GetPulumiPath(TemplatePolicyDir)
	}

	// Use the classic template directory if there is no override.
	return GetPulumiPath(TemplateDir)
}

// walkFiles is a helper that walks the directories/files in a source directory
// and performs an action for each item.
func walkFiles(sourceDir string, destDir string, projectName string,
	actionFn func(entry os.DirEntry, source string, dest string) error,
) error {
	contract.Requiref(sourceDir != "", "sourceDir", "must not be empty")
	contract.Requiref(destDir != "", "destDir", "must not be empty")
	contract.Requiref(actionFn != nil, "actionFn", "must not be nil")

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		source := filepath.Join(sourceDir, name)
		dest := filepath.Join(destDir, name)

		if entry.IsDir() {
			// Ignore the .git directory.
			if name == GitDir {
				continue
			}

			if err := actionFn(entry, source, dest); err != nil {
				return err
			}

			if err := walkFiles(source, dest, projectName, actionFn); err != nil {
				return err
			}
		} else {
			// Ignore the legacy template manifest.
			if name == legacyPulumiTemplateManifestFile {
				continue
			}

			// The file name may contain a placeholder for project name: replace it with the actual value.
			newDest := transform(dest, projectName, "")

			if err := actionFn(entry, source, newDest); err != nil {
				return err
			}
		}
	}

	return nil
}

// newExistingFilesError returns a new error from a list of existing file names
// that would be overwritten.
func newExistingFilesError(existing []string) error {
	contract.Assertf(len(existing) > 0, "called with no existing files")
	message := "creating this template will make changes to existing files:\n"
	for _, file := range existing {
		message = message + fmt.Sprintf("  overwrite   %s\n", file)
	}
	message = message + "\nrerun the command and pass --force to accept and create"
	return errors.New(message)
}

type TemplateNotFoundError struct{ msg string }

func (err TemplateNotFoundError) Error() string { return err.msg }

func (TemplateNotFoundError) Is(target error) bool {
	_, v := target.(TemplateNotFoundError)
	_, p := target.(*TemplateNotFoundError)
	return v || p
}

// newTemplateNotFoundError returns an error for when the template doesn't exist,
// offering distance-based suggestions in the error message.
func newTemplateNotFoundError(templateDir string, templateName string) TemplateNotFoundError {
	message := fmt.Sprintf("template '%s' not found", templateName)

	// Attempt to read the directory to offer suggestions.
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		contract.IgnoreError(err)
		return TemplateNotFoundError{message}
	}

	// Get suggestions based on levenshtein distance.
	suggestions := []string{}
	const minDistance = 2
	op := levenshtein.DefaultOptions
	for _, entry := range entries {
		distance := levenshtein.DistanceForStrings([]rune(templateName), []rune(entry.Name()), op)
		if distance <= minDistance {
			suggestions = append(suggestions, entry.Name())
		}
	}

	// Build-up error message with suggestions.
	if len(suggestions) > 0 {
		message = message + "\n\nDid you mean this?\n"
		for _, suggestion := range suggestions {
			message = message + fmt.Sprintf("\t%s\n", suggestion)
		}
	}

	return TemplateNotFoundError{message}
}

// transform returns a new string with ${PROJECT} and ${DESCRIPTION} replaced by
// the value of projectName and projectDescription.
func transform(content string, projectName string, projectDescription string) string {
	// On Windows, we need to replace \n with \r\n because go-git does not currently handle it.
	if runtime.GOOS == "windows" {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	content = strings.ReplaceAll(content, "${PROJECT}", projectName)
	content = strings.ReplaceAll(content, "${DESCRIPTION}", projectDescription)
	return content
}

// writeAllBytes writes the bytes to the specified file, with an option to overwrite.
func writeAllBytes(filename string, bytes []byte, overwrite bool, mode os.FileMode) error {
	flag := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flag = flag | os.O_TRUNC
	} else {
		flag = flag | os.O_EXCL
	}

	if overwrite {
		info, _ := os.Stat(filename)
		if info != nil && info.IsDir() {
			err := os.RemoveAll(filename)
			if err != nil {
				return err
			}
		}
	}

	f, err := os.OpenFile(filename, flag, mode)
	if err != nil {
		return err
	}
	defer contract.IgnoreClose(f)

	_, err = f.Write(bytes)
	return err
}

// isBinary returns true if a zero byte occurs within the first
// 8000 bytes (or the entire length if shorter). This is the
// same approach that git uses to determine if a file is binary.
func isBinary(bytes []byte) bool {
	const firstFewBytes = 8000

	length := len(bytes)
	if firstFewBytes < length {
		length = firstFewBytes
	}

	for i := 0; i < length; i++ {
		if bytes[i] == 0 {
			return true
		}
	}

	return false
}
