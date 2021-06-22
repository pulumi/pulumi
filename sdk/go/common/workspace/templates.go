// Copyright 2016-2018, Pulumi Corporation.
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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/texttheater/golang-levenshtein/levenshtein"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
)

const (
	defaultProjectName = "project"

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
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return []Template{template}, nil
	}

	// Otherwise, read all subdirectories to find the ones
	// that contain a Pulumi.yaml.
	infos, err := ioutil.ReadDir(path)
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
			if err != nil && !os.IsNotExist(err) {
				return nil, err
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
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return []PolicyPackTemplate{template}, nil
	}

	// Otherwise, read all subdirectories to find the ones
	// that contain a PulumiPolicy.yaml.
	infos, err := ioutil.ReadDir(path)
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
			if err != nil && !os.IsNotExist(err) {
				return nil, err
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
	Important   bool                                  // Indicates whether the template should be listed by default.

	ProjectName        string // Name of the project.
	ProjectDescription string // Optional description of the project.
}

// PolicyPackTemplate represents a Policy Pack template.
type PolicyPackTemplate struct {
	Dir         string // The directory containing PulumiPolicy.yaml.
	Name        string // The name of the template.
	Description string // Description of the template.
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

// IsTemplateURL returns true if templateNamePathOrURL starts with "https://".
func IsTemplateURL(templateNamePathOrURL string) bool {
	return strings.HasPrefix(templateNamePathOrURL, "https://")
}

// isTemplateFileOrDirectory returns true if templateNamePathOrURL is the name of a valid file or directory.
func isTemplateFileOrDirectory(templateNamePathOrURL string) bool {
	_, err := os.Stat(templateNamePathOrURL)
	return err == nil
}

// RetrieveTemplates retrieves a "template repository" based on the specified name, path, or URL.
func RetrieveTemplates(templateNamePathOrURL string, offline bool,
	templateKind TemplateKind) (TemplateRepository, error) {

	if IsTemplateURL(templateNamePathOrURL) {
		return retrieveURLTemplates(templateNamePathOrURL, offline, templateKind)
	}
	if isTemplateFileOrDirectory(templateNamePathOrURL) {
		return retrieveFileTemplates(templateNamePathOrURL)
	}
	return retrievePulumiTemplates(templateNamePathOrURL, offline, templateKind)
}

// retrieveURLTemplates retrieves the "template repository" at the specified URL.
func retrieveURLTemplates(rawurl string, offline bool, templateKind TemplateKind) (TemplateRepository, error) {
	if offline {
		return TemplateRepository{}, errors.Errorf("cannot use %s offline", rawurl)
	}

	var err error

	// Create a temp dir.
	var temp string
	if temp, err = ioutil.TempDir("", "pulumi-template-"); err != nil {
		return TemplateRepository{}, err
	}

	var fullPath string
	if fullPath, err = RetrieveGitFolder(rawurl, temp); err != nil {
		return TemplateRepository{}, err
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
func retrievePulumiTemplates(templateName string, offline bool, templateKind TemplateKind) (TemplateRepository, error) {
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
	if err := os.MkdirAll(templateDir, 0700); err != nil {
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
		err := gitutil.GitCloneOrPull(repo, branch, templateDir, false /*shallow*/)
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
			if os.IsNotExist(err) {
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
func RetrieveGitFolder(rawurl string, path string) (string, error) {
	url, urlPath, err := gitutil.ParseGitRepoURL(rawurl)
	if err != nil {
		return "", err
	}

	ref, commit, subDirectory, err := gitutil.GetGitReferenceNameOrHashAndSubDirectory(url, urlPath)
	if err != nil {
		return "", err
	}

	if ref != "" {
		if cloneErr := gitutil.GitCloneOrPull(url, ref, path, true /*shallow*/); cloneErr != nil {
			return "", cloneErr
		}
	} else {
		if cloneErr := gitutil.GitCloneAndCheckoutCommit(url, commit, path); cloneErr != nil {
			return "", cloneErr
		}
	}

	// Verify the sub directory exists.
	fullPath := filepath.Join(path, filepath.FromSlash(subDirectory))
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.Errorf("%s is not a directory", fullPath)
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
		return Template{}, errors.Errorf("%s is not a directory", path)
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
		template.Important = proj.Template.Important
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
		func(info os.FileInfo, source string, dest string) error {
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

// CopyTemplateFiles does the actual copy operation to a destination directory.
func CopyTemplateFiles(
	sourceDir, destDir string, force bool, projectName string, projectDescription string) error {

	return walkFiles(sourceDir, destDir, projectName,
		func(info os.FileInfo, source string, dest string) error {
			if info.IsDir() {
				// Create the destination directory.
				return os.Mkdir(dest, 0700)
			}

			// Read the source file.
			b, err := ioutil.ReadFile(source)
			if err != nil {
				return err
			}

			// Transform only if it isn't a binary file.
			result := b
			if !isBinary(b) {
				transformed := transform(string(b), projectName, projectDescription)
				result = []byte(transformed)
			}

			// Originally we just wrote in 0600 mode, but
			// this does not preserve the executable bit.
			// With the new logic below, we try to be at
			// least as permissive as 0600 and whathever
			// permissions the source file or symlink had.
			var mode os.FileMode
			sourceStat, err := os.Lstat(source)
			if err != nil {
				return err
			}
			mode = sourceStat.Mode().Perm() | 0600

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
		return PolicyPackTemplate{}, errors.Errorf("%s is not a directory", path)
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

// Naming rules are backend-specific. However, we provide baseline sanitization for project names
// in this file. Though the backend may enforce stronger restrictions for a project name or description
// further down the line.
var (
	validProjectNameRegexp = regexp.MustCompile("^[A-Za-z0-9_.-]{1,100}$")
)

// ValidateProjectName ensures a project name is valid, if it is not it returns an error with a message suitable
// for display to an end user.
func ValidateProjectName(s string) error {
	if s == "" {
		return errors.New("A project name may not be empty")
	}

	if len(s) > 100 {
		return errors.New("A project name must be 100 characters or less")
	}

	if !validProjectNameRegexp.MatchString(s) {
		return errors.New("A project name may only contain alphanumeric, hyphens, underscores, and periods")
	}

	// This is needed to stop cyclic imports in DotNet projects
	if strings.ToLower(s) == "pulumi" || strings.HasPrefix(strings.ToLower(s), "pulumi.") {
		return errors.New("A project name must not be `Pulumi` and must not start with the prefix `Pulumi.` " +
			"to avoid collision with standard libraries")
	}

	return nil
}

// ValidateProjectDescription ensures a project description name is valid, if it is not it returns an error with a
// message suitable for display to an end user.
func ValidateProjectDescription(s string) error {
	const maxTagValueLength = 256

	if len(s) > maxTagValueLength {
		return errors.New("A project description must be 256 characters or less")
	}

	return nil
}

// ValueOrSanitizedDefaultProjectName returns the value or a sanitized valid project name
// based on defaultNameToSanitize.
func ValueOrSanitizedDefaultProjectName(name string, projectName string, defaultNameToSanitize string) string {
	// If we have a name, use it.
	if name != "" {
		return name
	}

	// If the project already has a name that isn't a replacement string, use it.
	if projectName != "${PROJECT}" {
		return projectName
	}

	// Otherwise, get a sanitized version of `defaultNameToSanitize`.
	return getValidProjectName(defaultNameToSanitize)
}

// ValueOrDefaultProjectDescription returns the value or defaultDescription.
func ValueOrDefaultProjectDescription(
	description string, projectDescription string, defaultDescription string) string {

	// If we have a description, use it.
	if description != "" {
		return description
	}

	// If the project already has a description that isn't a replacement string, use it.
	if projectDescription != "${DESCRIPTION}" {
		return projectDescription
	}

	// Otherwise, use the default, which may be an empty string.
	return defaultDescription
}

// getValidProjectName returns a valid project name based on the passed-in name.
func getValidProjectName(name string) string {
	// If the name is valid, return it.
	if ValidateProjectName(name) == nil {
		return name
	}

	// Otherwise, try building-up the name, removing any invalid chars.
	var result string
	for i := 0; i < len(name); i++ {
		temp := result + string(name[i])
		if ValidateProjectName(temp) == nil {
			result = temp
		}
	}

	// If we couldn't come up with a valid project name, fallback to a default.
	if result == "" {
		result = defaultProjectName
	}

	return result
}

// walkFiles is a helper that walks the directories/files in a source directory
// and performs an action for each item.
func walkFiles(sourceDir string, destDir string, projectName string,
	actionFn func(info os.FileInfo, source string, dest string) error) error {

	contract.Require(sourceDir != "", "sourceDir")
	contract.Require(destDir != "", "destDir")
	contract.Require(actionFn != nil, "actionFn")

	infos, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, info := range infos {
		name := info.Name()
		source := filepath.Join(sourceDir, name)
		dest := filepath.Join(destDir, name)

		if info.IsDir() {
			// Ignore the .git directory.
			if name == GitDir {
				continue
			}

			if err := actionFn(info, source, dest); err != nil {
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

			if err := actionFn(info, source, newDest); err != nil {
				return err
			}
		}
	}

	return nil
}

// newExistingFilesError returns a new error from a list of existing file names
// that would be overwritten.
func newExistingFilesError(existing []string) error {
	contract.Assert(len(existing) > 0)
	message := "creating this template will make changes to existing files:\n"
	for _, file := range existing {
		message = message + fmt.Sprintf("  overwrite   %s\n", file)
	}
	message = message + "\nrerun the command and pass --force to accept and create"
	return errors.New(message)
}

// newTemplateNotFoundError returns an error for when the template doesn't exist,
// offering distance-based suggestions in the error message.
func newTemplateNotFoundError(templateDir string, templateName string) error {
	message := fmt.Sprintf("template '%s' not found", templateName)

	// Attempt to read the directory to offer suggestions.
	infos, err := ioutil.ReadDir(templateDir)
	if err != nil {
		contract.IgnoreError(err)
		return errors.New(message)
	}

	// Get suggestions based on levenshtein distance.
	suggestions := []string{}
	const minDistance = 2
	op := levenshtein.DefaultOptions
	for _, info := range infos {
		distance := levenshtein.DistanceForStrings([]rune(templateName), []rune(info.Name()), op)
		if distance <= minDistance {
			suggestions = append(suggestions, info.Name())
		}
	}

	// Build-up error message with suggestions.
	if len(suggestions) > 0 {
		message = message + "\n\nDid you mean this?\n"
		for _, suggestion := range suggestions {
			message = message + fmt.Sprintf("\t%s\n", suggestion)
		}
	}

	return errors.New(message)
}

// transform returns a new string with ${PROJECT} and ${DESCRIPTION} replaced by
// the value of projectName and projectDescription.
func transform(content string, projectName string, projectDescription string) string {
	// On Windows, we need to replace \n with \r\n because go-git does not currently handle it.
	if runtime.GOOS == "windows" {
		content = strings.Replace(content, "\n", "\r\n", -1)
	}
	content = strings.Replace(content, "${PROJECT}", projectName, -1)
	content = strings.Replace(content, "${DESCRIPTION}", projectDescription, -1)
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
