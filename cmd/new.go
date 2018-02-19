// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/go-multierror"

	"github.com/pulumi/pulumi/pkg/tokens"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"golang.org/x/oauth2"

	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/spf13/cobra"

	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	githttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

const (
	// Set this environment variable to a GitHub personal access token to access private repos.
	// nolint: gas
	githubAccessTokenEnvVar = "PULUMI_GITHUB_TOKEN"

	githubTemplateOrg            = "pulumi-templates"
	githubTemplateCloneURLFormat = "https://github.com/" + githubTemplateOrg + "/%s.git"

	defaultProjectName        = "project"
	defaultProjectDescription = "A Pulumi project."
)

// Template represents a project template.
type Template struct {
	Name        string
	Description string
}

func newNewCmd() *cobra.Command {
	var name string
	var description string
	var force bool

	cmd := &cobra.Command{
		Use:   "new <template>",
		Short: "Create a new Pulumi project",
		Args:  cmdutil.MaximumNArgs(1),
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {

			var template Template
			if len(args) > 0 {
				templateName := strings.ToLower(args[0])
				if !isValidTemplateName(templateName) {
					return fmt.Errorf("'%s' is not a valid template name", templateName)
				}
				template = Template{Name: templateName}
			} else {
				t, err := chooseTemplate()
				if err != nil {
					return err
				}
				template = t
			}

			templateDir, err := getTemplateDir()
			if err != nil {
				return err
			}

			repoDir := filepath.Join(templateDir, template.Name)

			err = fetchOrUpdateTemplate(repoDir, template.Name)
			if err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			if name == "" {
				// Use the name of the current working directory as the default name, tweaking
				// it as needed to ensure it's a valid project name.
				name = getValidProjectName(filepath.Base(cwd))
			} else {
				if !tokens.IsPackageName(name) {
					return fmt.Errorf("'%s' is not a valid project name", name)
				}
			}

			if description == "" {
				if template.Description != "" {
					description = template.Description
				} else {
					description = defaultProjectDescription
				}
			}

			if !force {
				// Do a dry run to ensure the directory doesn't contain any existing files.
				err = copyFilesDryRun(repoDir, cwd)
				if err != nil {
					return err
				}
			}

			// Actually copy the files.
			err = copyFiles(repoDir, cwd, force, func(content string) string {
				content = strings.Replace(content, "${PROJECT}", name, -1)
				content = strings.Replace(content, "${DESCRIPTION}", description, -1)
				return content
			})
			if err != nil {
				return err
			}

			fmt.Println("Your project was created successfully.")
			return nil
		}),
	}

	cmd.PersistentFlags().StringVarP(
		&name, "name", "n", "",
		"The project name. If not specified, the name of the current working directory is used.")
	cmd.PersistentFlags().StringVarP(
		&description, "description", "d", "",
		"The project description. If not specified, a default description is used.")
	cmd.PersistentFlags().BoolVarP(
		&force, "force", "f", false,
		"Forces content to be generated even if it would change existing files.")

	return cmd
}

// We'll limit template names to the following pattern, to avoid attempting to clone repositories
// that contain characters that would mess up the file system.
var templateNameRegexp = regexp.MustCompile("[a-z0-9_.-]*")

func isValidTemplateName(name string) bool {
	return name != "" && templateNameRegexp.FindString(name) == name
}

// getValidProjectName returns a valid project name based on the passed-in name.
func getValidProjectName(name string) string {
	// If the name is valid, return it.
	if tokens.IsPackageName(name) {
		return name
	}

	// Otherwise, try building-up the name, removing any invalid chars.
	var result string
	for i := 0; i < len(name); i++ {
		temp := result + string(name[i])
		if tokens.IsPackageName(temp) {
			result = temp
		}
	}

	// If we couldn't come up with a valid project name, fallback to a default.
	if result == "" {
		return defaultProjectName
	}

	return result
}

func fetchOrUpdateTemplate(dir string, template string) error {
	contract.Require(dir != "", "dir")

	if d, err := os.Stat(dir); err != nil {
		// If the directory does not exist, clone it from GitHub.
		if os.IsNotExist(err) {
			url := fmt.Sprintf(githubTemplateCloneURLFormat, url.PathEscape(template))
			if _, err := git.PlainClone(dir, false, &git.CloneOptions{URL: url, Auth: getAuthMethod()}); err != nil {
				// An empty directory may be left behind when there is a clone error.
				// Remove it to avoid filling up the template dir with non-existent templates.
				if removeErr := removeZombieCloneDir(dir); removeErr != nil {
					err = multierror.Append(err, removeErr)
				}
				return errors.Wrapf(err, "could not clone template '%s' from %s", template, url)
			}
		}
	} else if d.IsDir() {
		// If the directory is a git repository, pull from origin.
		if gitDir, err := os.Stat(filepath.Join(dir, workspace.GitDir)); err != nil || !gitDir.IsDir() {
			return nil
		}
		if r, err := git.PlainOpen(dir); err == nil {
			if w, err := r.Worktree(); err == nil {
				opt := &git.PullOptions{RemoteName: "origin"}

				// Include credentials (if set) when origin is the pulumi-templates org.
				if org, _, err := getGitHubProjectForOriginByRepo(r); err == nil && org == githubTemplateOrg {
					opt.Auth = getAuthMethod()
				}

				if err := w.Pull(opt); err != nil && err != git.NoErrAlreadyUpToDate {
					// Print a warning instead of failing if the pull fails, to allow
					// the use of the template when offline.
					fmt.Fprintf(os.Stderr, "warning: could not update template '%s': %v\n", template, err)
				}
			}
		}
	} else {
		// If it's a file, fail.
		return fmt.Errorf("'%s' is not a directory", dir)
	}

	return nil
}

func removeZombieCloneDir(dir string) error {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(infos) == 0 || (len(infos) == 1 && infos[0].IsDir() && infos[0].Name() == workspace.GitDir) {
		return os.RemoveAll(dir)
	}

	return nil
}

func getAuthMethod() transport.AuthMethod {
	accessToken := os.Getenv(githubAccessTokenEnvVar)
	if accessToken != "" {
		return &githttp.BasicAuth{Username: accessToken}
	}
	return nil
}

func newExistingFilesError(existing []string) error {
	contract.Assert(len(existing) > 0)
	message := "creating this template will make changes to existing files:\n"
	for _, file := range existing {
		message = message + fmt.Sprintf("  overwrite   %s\n", file)
	}
	message = message + "\nrerun the command and pass --force to accept and create"
	return errors.New(message)
}

func copyFiles(sourceDir string, destDir string, force bool, transformFn func(string) string) error {
	return walkTemplateFiles(sourceDir, destDir, func(info os.FileInfo, source string, dest string) error {
		if info.IsDir() {
			// Create the destination directory.
			return os.Mkdir(dest, 0700)
		}

		// Read the source file.
		b, err := ioutil.ReadFile(source)
		if err != nil {
			return err
		}

		// We assume all template files are text files.
		transformed := transformFn(string(b))

		// Write to the destination file.
		err = writeAllText(dest, transformed, force)
		if err != nil {
			// An existing file has shown up in between the dry run and the actual copy operation.
			if os.IsExist(err) {
				return newExistingFilesError([]string{filepath.Base(dest)})
			}
		}
		return err
	})
}

func walkTemplateFiles(sourceDir string, destDir string,
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
			// Ignore the .git directory
			if name == workspace.GitDir {
				continue
			}

			if err := actionFn(info, source, dest); err != nil {
				return err
			}

			if err := walkTemplateFiles(source, dest, actionFn); err != nil {
				return err
			}
		} else {
			// Ignore .gitattributes
			if name == ".gitattributes" {
				continue
			}

			if err := actionFn(info, source, dest); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeAllText(filename string, text string, overwrite bool) error {
	flag := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flag = flag | os.O_TRUNC
	} else {
		flag = flag | os.O_EXCL
	}

	f, err := os.OpenFile(filename, flag, 0600)
	if err != nil {
		return err
	}

	_, err = f.WriteString(text)

	if err1 := f.Close(); err == nil {
		err = err1
	}

	return err
}

func copyFilesDryRun(sourceDir string, destDir string) error {
	var existing []string

	err := walkTemplateFiles(sourceDir, destDir, func(info os.FileInfo, source string, dest string) error {
		if f, err := os.Stat(dest); err == nil && !f.IsDir() {
			existing = append(existing, filepath.Base(dest))
		}
		return nil
	})
	contract.IgnoreError(err)

	if len(existing) > 0 {
		return newExistingFilesError(existing)
	}

	return nil
}

func getTemplateDir() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", errors.Wrapf(err, "getting user home directory")
	}

	pulumiTemplateDir := filepath.Join(user.HomeDir, workspace.BookkeepingDir, workspace.TemplateDir)
	if err := os.MkdirAll(pulumiTemplateDir, 0700); err != nil {
		return "", errors.Wrapf(err, "failed to create '%s'", pulumiTemplateDir)
	}

	return pulumiTemplateDir, nil
}

// chooseTemplate will prompt the user to choose amongst the available templates.
func chooseTemplate() (Template, error) {
	const chooseTemplateErr = "no template selected; please use `pulumi new` to choose one"
	if !cmdutil.Interactive() {
		return Template{}, errors.New(chooseTemplateErr)
	}

	templates, err := fetchGitHubTemplates()
	if err != nil || len(templates) == 0 {
		// If we couldn't fetch the list of templates from GitHub, see if any
		// local templates are present.
		localTemplates, localErr := getLocalTemplates()
		if localErr != nil || len(localTemplates) == 0 {
			// If there aren't any local templates, return the original error.
			return Template{}, errors.Wrap(err, chooseTemplateErr)
		}

		// Proceed with the list of local templates. Print a warning with the original error.
		templates = localTemplates
		fmt.Fprintf(os.Stderr, "warning: could not fetch list of remote templates; using local templates: %v\n", err)
	}

	// Customize the prompt a little bit (and disable color since it doesn't match our scheme).
	surveycore.DisableColor = true
	surveycore.QuestionIcon = ""
	surveycore.SelectFocusIcon = colors.ColorizeText(colors.BrightGreen + ">" + colors.Reset)
	message := "\rPlease choose a template:"
	message = colors.ColorizeText(colors.BrightWhite + message + colors.Reset)

	var options []string
	nameToTemplateMap := make(map[string]Template)
	for _, template := range templates {
		options = append(options, template.Name)
		nameToTemplateMap[template.Name] = template
	}
	sort.Strings(options)

	var option string
	if err := survey.AskOne(&survey.Select{
		Message: message,
		Options: options,
	}, &option, nil); err != nil {
		return Template{}, errors.New(chooseTemplateErr)
	}

	return nameToTemplateMap[option], nil
}

func getLocalTemplates() ([]Template, error) {
	templateDir, err := getTemplateDir()
	if err != nil {
		return nil, err
	}

	infos, err := ioutil.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	var templates []Template
	for _, info := range infos {
		if info.IsDir() {
			templates = append(templates, Template{Name: info.Name()})
		}
	}
	return templates, nil
}

func fetchGitHubTemplates() ([]Template, error) {
	var repositoryType = "public"
	var tc *http.Client

	ctx := context.Background()

	// If an access token is available, use it to authenticate with GitHub
	// and request both public and private repositories.
	accessToken := os.Getenv(githubAccessTokenEnvVar)
	if accessToken != "" {
		repositoryType = "all"
		tc = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken}))
	}

	client := github.NewClient(tc)

	// For now we'll always make a regular request. Consider caching the response
	// and making a conditional request.
	opt := &github.RepositoryListByOrgOptions{Type: repositoryType}
	repos, _, err := client.Repositories.ListByOrg(ctx, githubTemplateOrg, opt)
	if err != nil {
		return nil, err
	}

	var templates []Template
	for _, repo := range repos {
		templates = append(templates, Template{
			Name:        repo.GetName(),
			Description: repo.GetDescription(),
		})
	}

	return templates, nil
}
