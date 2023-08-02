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

package gitutil

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// VCSKind represents the hostname of a specific type of VCS.
// For eg., github.com, gitlab.com etc.
type VCSKind = string

// Constants related to detecting the right type of source control provider for git.
const (
	defaultGitCloudRepositorySuffix = ".git"

	// GitLabHostName The host name for GitLab.
	GitLabHostName VCSKind = "gitlab.com"
	// GitHubHostName The host name for GitHub.
	GitHubHostName VCSKind = "github.com"
	// AzureDevOpsHostName The host name for Azure DevOps
	AzureDevOpsHostName VCSKind = "dev.azure.com"
	// BitbucketHostName The host name for Bitbucket
	BitbucketHostName VCSKind = "bitbucket.org"
)

// The pre-compiled regex used to extract owner and repo name from an SSH git remote URL.
// Note: If you are renaming any of the group names in the regex (the ?P<group_name> part) to something else,
// be sure to update its usage elsewhere in the code as well.
// The nolint instruction prevents gometalinter from complaining about the length of the line.
var (
	cloudSourceControlSSHRegex    = regexp.MustCompile(`git@(?P<host_name>[a-zA-Z.-]*\.[a-zA-Z]+):(?P<owner_and_repo>[^/]+/[^/]+\.git).?$`)                       //nolint
	azureSourceControlSSHRegex    = regexp.MustCompile(`git@([a-zA-Z]+\.)?(?P<host_name>([a-zA-Z]+\.)*[a-zA-Z]*\.[a-zA-Z]+):(v[0-9]{1}/)?(?P<owner_and_repo>.*)`) //nolint
	legacyAzureSourceControlRegex = regexp.MustCompile("(?P<owner>[a-zA-Z0-9-]*).visualstudio.com$")
)

// VCSInfo describes a cloud-hosted version control system.
// Cloud hosted VCS' typically have an owner (could be an organization),
// to whom the repo belongs.
type VCSInfo struct {
	Owner string
	Repo  string
	Kind  VCSKind
}

// GetGitRepository returns the git repository by walking up from the provided directory.
// If no repository is found, will return (nil, nil).
func GetGitRepository(dir string) (*git.Repository, error) {
	gitRoot, err := fsutil.WalkUp(dir, func(s string) bool { return filepath.Base(s) == ".git" }, nil)
	if err != nil {
		return nil, fmt.Errorf("searching for git repository from %v: %w", dir, err)
	}
	if gitRoot == "" {
		return nil, nil
	}

	// Open the git repo in the .git folder's parent, not the .git folder itself.
	repo, err := git.PlainOpenWithOptions(filepath.Dir(gitRoot), &git.PlainOpenOptions{
		EnableDotGitCommonDir: true,
	})
	if err == git.ErrRepositoryNotExists {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading git repository: %w", err)
	}
	return repo, nil
}

// GetGitHubProjectForOrigin returns the GitHub login, and GitHub repo name if the "origin" remote is
// a GitHub URL.
func GetGitHubProjectForOrigin(dir string) (*VCSInfo, error) {
	repo, err := GetGitRepository(dir)
	if repo == nil {
		return nil, fmt.Errorf("no git repository found from %v", dir)
	}
	if err != nil {
		return nil, err
	}
	remoteURL, err := GetGitRemoteURL(repo, "origin")
	if err != nil {
		return nil, err
	}
	return TryGetVCSInfo(remoteURL)
}

// GetGitRemoteURL returns the remote URL for the given remoteName in the repo.
func GetGitRemoteURL(repo *git.Repository, remoteName string) (string, error) {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("could not read origin information: %w", err)
	}

	remoteURL := ""
	if len(remote.Config().URLs) > 0 {
		remoteURL = remote.Config().URLs[0]
	}

	return remoteURL, nil
}

// IsGitOriginURLGitHub returns true if the provided remoteURL is detected as GitHub.
//
// Deprecated: Use `strings.Contains(remoteURL, "github.com")` instead.
func IsGitOriginURLGitHub(remoteURL string) bool {
	return strings.Contains(remoteURL, GitHubHostName)
}

// TryGetVCSInfo attempts to detect whether the provided remoteURL
// is an SSH or an HTTPS remote URL. It then extracts the repo, owner name,
// and the type (kind) of VCS from it.
func TryGetVCSInfo(remoteURL string) (_ *VCSInfo, err error) {
	var project, vcsKind string

	defer func() {
		if err != nil {
			err = fmt.Errorf("detecting VCS info from remote URL %q: %w", remoteURL, err)
		}
	}()

	endpoint, err := transport.NewEndpoint(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}

	// If the remote is using git SSH, then we extract the named groups by matching
	// with the pre-compiled regex pattern.
	switch endpoint.Protocol {
	case "ssh":
		// Most cloud-hosted VCS have the ssh URL of the format git@somehostname.com:owner/repo
		if cloudSourceControlSSHRegex.MatchString(remoteURL) {
			groups := getMatchedGroupsFromRegex(cloudSourceControlSSHRegex, remoteURL)
			vcsKind = groups["host_name"]
			project = groups["owner_and_repo"]
			project = strings.TrimSuffix(project, defaultGitCloudRepositorySuffix)
		} else if azureSourceControlSSHRegex.MatchString(remoteURL) {
			// Azure's DevOps service uses a git SSH url, that is completely different
			// from the rest of the services.
			groups := getMatchedGroupsFromRegex(azureSourceControlSSHRegex, remoteURL)
			vcsKind = groups["host_name"]
			project = groups["owner_and_repo"]
			project = strings.TrimSuffix(project, defaultGitCloudRepositorySuffix)
		}
	case "http", "https":
		vcsKind = endpoint.Host
		project = endpoint.Path
		// Replace the .git extension from the path.
		project = strings.TrimSuffix(project, defaultGitCloudRepositorySuffix)
		// Remove the prefix "/". TrimPrefix returns the same value if there is no prefix.
		// So it is safe to use it instead of doing any sort of substring matches.
		project = strings.TrimPrefix(project, "/")
	default:
		return nil, fmt.Errorf("unsupported protocol %q", endpoint.Protocol)
	}

	// We had a valid endpoint but didn't match any known VCS.
	if project == "" {
		return nil, errors.New("project name not found in URL")
	}

	// For Azure, we will have more than 2 parts in the array.
	// Ex: owner/project/repo.git
	if vcsKind == AzureDevOpsHostName {
		azureSplit := strings.SplitN(project, "/", 2)

		// Azure DevOps repo links are in the format `owner/project/_git/repo`. Some remote URLs do
		// not include the `_git` piece, which results in the reconstructed URL linking to the
		// project dashboard. To remedy this, we will add the _git portion to the URL if its
		// missing.
		project = azureSplit[1]
		if !strings.Contains(project, "_git") {
			projectSplit := strings.SplitN(project, "/", 2)
			project = projectSplit[0] + "/_git/" + projectSplit[1]
		}

		return &VCSInfo{
			Owner: azureSplit[0],
			Repo:  project,
			Kind:  vcsKind,
		}, nil
	}

	// Legacy Azure URLs have the owner as part of the host name. We will convert the Git info to
	// reflect the newer Azure DevOps URLs. This allows the UI to properly construct the repo URL
	// and group it with other projects/stacks that have been pulled with a newer version of the Git
	// URL.
	if legacyAzureSourceControlRegex.MatchString(vcsKind) {
		groups := getMatchedGroupsFromRegex(legacyAzureSourceControlRegex, vcsKind)

		return &VCSInfo{
			Owner: groups["owner"],
			Repo:  project,
			Kind:  AzureDevOpsHostName,
		}, nil
	}

	// Since the vcsKind is not Azure, we can try to detect the other kinds of VCS.
	// We are splitting in two because some VCS providers (e.g. GitLab) allow for
	// subgroups.
	split := strings.SplitN(project, "/", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("project %q must include a '/'", project)
	}

	return &VCSInfo{
		Owner: split[0],
		Repo:  split[1],
		Kind:  vcsKind,
	}, nil
}

func getMatchedGroupsFromRegex(regex *regexp.Regexp, remoteURL string) map[string]string {
	// Get all matching groups.
	matches := regex.FindAllStringSubmatch(remoteURL, -1)[0]
	// Get the named groups in our regex.
	groupNames := regex.SubexpNames()

	groups := map[string]string{}
	for i, value := range matches {
		groups[groupNames[i]] = value
	}

	return groups
}

type urlAuthParser struct {
	mu sync.Mutex // guards sshKeys

	// sshKeys memoizes keys we've loaded for given host URLs, to avoid needing to
	// re-fetch public keys.
	sshKeys map[string]transport.AuthMethod
	// sshConfig allows us to inject config for testing.
	sshConfig sshUserSettings
}

// defaultURLAuthParser uses the host's SSH configuration.
var defaultURLAuthParser = &urlAuthParser{
	sshConfig: ssh_config.DefaultUserSettings,
}

// Parse parses a given URL and returns relevant auth. For SSH URLs, keys are
// read from the provided sshUserSettings.
func (p *urlAuthParser) Parse(remoteURL string) (string, transport.AuthMethod, error) {
	endpoint, err := transport.NewEndpoint(remoteURL)
	if err != nil {
		return "", nil, err
	}

	if endpoint.Protocol == "ssh" {
		var auth transport.AuthMethod

		p.mu.Lock()
		defer p.mu.Unlock()
		defer func() {
			// Memoize the key when we're done, if there was one.
			if auth == nil {
				return
			}
			if p.sshKeys == nil {
				p.sshKeys = make(map[string]transport.AuthMethod)
			}
			p.sshKeys[endpoint.Host] = auth
		}()

		// See if we've encountered this host before; if yes, use the existing key.
		if existing, ok := p.sshKeys[endpoint.Host]; ok {
			return remoteURL, existing, nil
		}

		auth, err = getSSHPublicKeys(endpoint.User, endpoint.Host, p.sshConfig)
		if err == nil {
			return remoteURL, auth, nil
		}

		// If we could't acquire a key (most likely because there is no
		// config defined for the host), we still treat the URL as valid
		// and attempt to use the SSH agent for auth.
		logging.V(10).Infof("%s: using agent auth instead", err)
		auth, err = gitssh.DefaultAuthBuilder(endpoint.User)
		return remoteURL, auth, err

	}

	// For non-SSH URLs, see if there is basic auth info. Strip it from the
	// endpoint as we go in order to remove it from the string output.
	var auth *http.BasicAuth
	if u, p := endpoint.User, endpoint.Password; u != "" || p != "" {
		auth = &http.BasicAuth{Username: u, Password: p}
		endpoint.User, endpoint.Password = "", ""
	}
	return endpoint.String(), auth, nil
}

// parseAuthURL extracts HTTP basic auth parameters if provided in the URL.
//
// If the URL uses SSH, the user's SSH configuration is parsed and relevant
// public keys are returned for authentication.
func parseAuthURL(url string) (string, transport.AuthMethod, error) {
	return defaultURLAuthParser.Parse(url)
}

// sshUserSettings allows us to ingect mock SSH config.
type sshUserSettings interface {
	GetStrict(alias, key string) (string, error)
}

var _ sshUserSettings = (*ssh_config.UserSettings)(nil)

// getSSHPublicKeys reads from the user's SSH configuration and returns public
// keys for the given host.
//
// The `PULUMI_GITSSH_PASSPHRASE` environment variable can be provided if the
// relevant key is passphrase protected, or (if in an interactive session) the
// user will be prompted to input a passphrase.
//
// TODO: Integrate with GCM when https://github.com/go-git/go-git/issues/490
// lands.
//
// This method handles `~/.ssh/config`, `/etc/host/ssh`, and `Include`
// directives in the SSH configuration as you would expect.
func getSSHPublicKeys(user string, host string, sshConfig sshUserSettings) (*gitssh.PublicKeys, error) {
	if sshConfig == nil {
		sshConfig = ssh_config.DefaultUserSettings
	}
	privateKeyPath, err := sshConfig.GetStrict(host, "IdentityFile")
	if err != nil {
		return nil, err
	}
	// Expand tilde (~) if present in the path.
	privateKeyPath, err = expandHomeDir(privateKeyPath)
	if err != nil {
		return nil, err
	}
	logging.V(10).Infof("Inferred SSH key '%s' for Git host %s", privateKeyPath, host)

	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	// Attempt to load the key. If this is an interactive session and the key
	// is passphrase-protected we will prompt the user to enter a passphrase.
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if errors.As(err, new(*ssh.PassphraseMissingError)) {
		passphrase := env.GitSSHPassphrase.Value()

		if passphrase == "" && cmdutil.Interactive() {
			passphrase, err = cmdutil.ReadConsoleNoEcho(
				fmt.Sprintf("Enter passphrase for SSH key '%s'", privateKeyPath),
			)
			if err != nil {
				return nil, err
			}
		}

		signer, err = ssh.ParsePrivateKeyWithPassphrase(privateKeyBytes, []byte(passphrase))
	}
	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{User: user, Signer: signer}, nil
}

// expandHomeDir expands file paths relative to the user's home directory (~) into absolute paths.
func expandHomeDir(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] != '~' {
		// Not a "~/foo" path.
		return path, nil
	}

	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		// We won't expand "~user"-style paths.
		return "", errors.New("cannot expand user-specific home dir")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, path[1:]), nil
}

// GitCloneAndCheckoutCommit clones the Git repository and checkouts the specified commit.
func GitCloneAndCheckoutCommit(url string, commit plumbing.Hash, path string) error {
	logging.V(10).Infof("Attempting to clone from %s at commit %v and path %s", url, commit, path)

	u, auth, err := parseAuthURL(url)
	if err != nil {
		return err
	}
	repo, err := git.PlainClone(path, false, &git.CloneOptions{
		URL:  u,
		Auth: auth,
	})
	if err != nil {
		return err
	}

	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	return w.Checkout(&git.CheckoutOptions{
		Hash:  commit,
		Force: true,
	})
}

func GitCloneOrPull(rawurl string, referenceName plumbing.ReferenceName, path string, shallow bool) error {
	logging.V(10).Infof("Attempting to clone from %s at ref %s", rawurl, referenceName)

	// TODO: https://github.com/go-git/go-git/pull/613 should have resolved the issue preventing this from cloning.
	if u, err := parseGitRepoURLParts(rawurl); err == nil && u.Hostname == AzureDevOpsHostName {
		// system-installed git is used to clone Azure DevOps repositories
		// due to https://github.com/go-git/go-git/issues/64
		return gitCloneOrPullSystemGit(rawurl, referenceName, path, shallow)
	}
	return gitCloneOrPull(rawurl, referenceName, path, shallow)
}

// GitCloneOrPull clones or updates the specified referenceName (branch or tag) of a Git repository.
func gitCloneOrPull(url string, referenceName plumbing.ReferenceName, path string, shallow bool) error {
	// For shallow clones, use a depth of 1.
	depth := 0
	if shallow {
		depth = 1
	}

	u, auth, err := parseAuthURL(url)
	if err != nil {
		return err
	}
	// Attempt to clone the repo.
	_, cloneErr := git.PlainClone(path, false, &git.CloneOptions{
		URL:           u,
		Auth:          auth,
		ReferenceName: referenceName,
		SingleBranch:  true,
		Depth:         depth,
		Tags:          git.NoTags,
	})
	if cloneErr != nil {
		// If the repo already exists, open it and pull.
		if cloneErr == git.ErrRepositoryAlreadyExists {
			repo, err := git.PlainOpen(path)
			if err != nil {
				return err
			}

			w, err := repo.Worktree()
			if err != nil {
				return err
			}

			// There are cases where go-git gets confused about files that were included in .gitignore
			// and then later removed from .gitignore and added to the repository, leaving unstaged
			// changes in the working directory after a pull. To address this, we'll first do a hard
			// reset of the worktree before pulling to ensure it's in a good state.
			if err := w.Reset(&git.ResetOptions{
				Mode: git.HardReset,
			}); err != nil {
				return err
			}

			if cloneErr = w.Pull(&git.PullOptions{
				ReferenceName: referenceName,
				SingleBranch:  true,
				Force:         true,
			}); cloneErr == git.NoErrAlreadyUpToDate {
				return nil
			}
		}
	}

	if cloneErr == git.ErrUnstagedChanges {
		// See https://github.com/pulumi/pulumi/issues/11121. We seem to be getting intermittent unstaged
		// changes errors, which is very hard to reproduce. This block of code catches this error and tries to
		// do a diff to see what the unstaged change is and tells the user to report this error to the above
		// ticket.

		repo, err := git.PlainOpen(path)
		if err != nil {
			return fmt.Errorf(
				"GitCloneOrPull reported unstaged changes, but the repo couldn't be opened to check: %w\n"+
					"Please report this to https://github.com/pulumi/pulumi/issues/11121.", err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf(
				"GitCloneOrPull reported unstaged changes, but the worktree couldn't be opened to check: %w\n"+
					"Please report this to https://github.com/pulumi/pulumi/issues/11121.", err)
		}

		status, err := worktree.Status()
		if err != nil {
			return fmt.Errorf(
				"GitCloneOrPull reported unstaged changes, but the worktree status couldn't be fetched to check: %w\n"+
					"Please report this to https://github.com/pulumi/pulumi/issues/11121.", err)
		}

		messages := make([]string, 0)
		for path, stat := range status {
			if stat.Worktree != git.Unmodified {
				messages = append(messages, fmt.Sprintf("%s was %c", path, rune(stat.Worktree)))
			}
		}

		return fmt.Errorf("GitCloneOrPull reported unstaged changes: %s\n"+
			"Please report this to https://github.com/pulumi/pulumi/issues/11121.",
			strings.Join(messages, "\n"))
	}

	return cloneErr
}

// gitCloneOrPullSystemGit uses the `git` command to pull or clone repositories.
func gitCloneOrPullSystemGit(url string, referenceName plumbing.ReferenceName, path string, shallow bool) error {
	// Assume repo already exists, pull changes.
	gitArgs := []string{
		"pull",
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); os.IsNotExist(err) {
		// Repo does not exist, clone it.
		gitArgs = []string{
			"clone", url, ".",
		}
		// For shallow clones, use a depth of 1.
		if shallow {
			gitArgs = append(gitArgs, "--depth")
			gitArgs = append(gitArgs, "1")
		}
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = path

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run `git %v`", strings.Join(gitArgs, " "))
	}
	return nil
}

// We currently accept Gist URLs in the form: https://gist.github.com/owner/id.
// We may want to consider supporting https://gist.github.com/id at some point,
// as well as arbitrary revisions, e.g. https://gist.github.com/owner/id/commit.
func parseGistURL(u *url.URL) (string, error) {
	path := strings.Trim(u.Path, "/")
	paths := strings.Split(path, "/")
	if len(paths) != 2 {
		return "", errors.New("invalid Gist URL")
	}

	owner := paths[0]
	if owner == "" {
		return "", errors.New("invalid Gist URL; no owner")
	}

	id := paths[1]
	if id == "" {
		return "", errors.New("invalid Gist URL; no id")
	}

	if !strings.HasSuffix(id, ".git") {
		id = id + ".git"
	}

	resultURL := u.Scheme + "://" + u.Host + "/" + id
	return resultURL, nil
}

func parseHostAuth(u *url.URL) string {
	if u.User == nil {
		return u.Host
	}
	user := u.User.Username()
	p, ok := u.User.Password()
	if !ok {
		return user + "@" + u.Host
	}
	return user + ":" + p + "@" + u.Host
}

type gitRepoURLParts struct {
	// URL is the base URL, without a path.
	URL string
	// Hostname is the actual hostname for the URL.
	Hostname string
	// Path is the path part of the URL, if any.
	Path string
}

func parseGitRepoURLParts(rawurl string) (gitRepoURLParts, error) {
	endpoint, err := transport.NewEndpoint(rawurl)
	if err != nil {
		return gitRepoURLParts{}, err
	}

	if endpoint.Protocol == "ssh" {
		// Normalize SSH URLs (including scp-style git@github.com URLs) into
		// ssh:// format so we can parse them the same as https:// URLs.
		rawurl = endpoint.String()
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return gitRepoURLParts{}, err
	}

	if u.Scheme != "https" && u.Scheme != "ssh" {
		return gitRepoURLParts{}, fmt.Errorf("invalid URL scheme: %s", u.Scheme)
	}

	hostname := u.Hostname()

	// Special case Gists.
	if u.Hostname() == "gist.github.com" {
		repo, err := parseGistURL(u)
		if err != nil {
			return gitRepoURLParts{}, err
		}
		return gitRepoURLParts{
			URL:      repo,
			Hostname: hostname,
		}, nil
	}

	// Special case Azure DevOps.
	if u.Hostname() == AzureDevOpsHostName {
		// Specifying branch/ref and subpath is currently unsupported.
		return gitRepoURLParts{
			URL:      rawurl,
			Hostname: hostname,
		}, nil
	}

	path := strings.TrimPrefix(u.Path, "/")
	paths := strings.Split(path, "/")
	if len(paths) < 2 {
		return gitRepoURLParts{}, errors.New("invalid Git URL")
	}

	// Shortcut for general case: URI Path contains '.git'
	// Cleave URI into what comes before and what comes after.
	if loc := strings.LastIndex(path, defaultGitCloudRepositorySuffix); loc != -1 {
		extensionOffset := loc + len(defaultGitCloudRepositorySuffix)
		resultURL := u.Scheme + "://" + parseHostAuth(u) + "/" + path[:extensionOffset]
		gitRepoPath := path[extensionOffset:]
		resultPath := strings.Trim(gitRepoPath, "/")
		return gitRepoURLParts{
			URL:      resultURL,
			Hostname: hostname,
			Path:     resultPath,
		}, nil
	}

	owner := paths[0]
	if owner == "" {
		return gitRepoURLParts{}, errors.New("invalid Git URL; no owner")
	}

	repo := paths[1]
	if repo == "" {
		return gitRepoURLParts{}, errors.New("invalid Git URL; no repository")
	}

	if !strings.HasSuffix(repo, ".git") {
		repo = repo + ".git"
	}

	resultURL := u.Scheme + "://" + parseHostAuth(u) + "/" + owner + "/" + repo
	resultPath := strings.TrimSuffix(strings.Join(paths[2:], "/"), "/")

	return gitRepoURLParts{
		URL:      resultURL,
		Hostname: hostname,
		Path:     resultPath,
	}, nil
}

// ParseGitRepoURL returns the URL to the Git repository and path from a raw URL.
// For example, an input of "https://github.com/pulumi/templates/templates/javascript" returns
// "https://github.com/pulumi/templates.git" and "templates/javascript".
// Additionally, it supports nested git projects, as used by GitLab.
// For example, "https://github.com/pulumi/platform-team/templates.git/templates/javascript"
// returns "https://github.com/pulumi/platform-team/templates.git" and "templates/javascript"
//
// Note: URL with a hostname of `dev.azure.com`, are currently treated as a raw git clone url
// and currently do not support subpaths.
func ParseGitRepoURL(rawurl string) (string, string, error) {
	parts, err := parseGitRepoURLParts(rawurl)
	if err != nil {
		return "", "", err
	}
	return parts.URL, parts.Path, err
}

var gitSHARegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// GetGitReferenceNameOrHashAndSubDirectory returns the reference name or hash, and sub directory path.
// The sub directory path always uses "/" as the separator.
func GetGitReferenceNameOrHashAndSubDirectory(url string, urlPath string) (
	plumbing.ReferenceName, plumbing.Hash, string, error,
) {
	// If path is empty, use HEAD.
	if urlPath == "" {
		return plumbing.HEAD, plumbing.ZeroHash, "", nil
	}

	// Trim leading/trailing separator(s).
	urlPath = strings.TrimPrefix(urlPath, "/")
	urlPath = strings.TrimSuffix(urlPath, "/")

	paths := strings.Split(urlPath, "/")

	// Ensure the path components are not "." or "..".
	for _, path := range paths {
		if path == "." || path == ".." {
			return "", plumbing.ZeroHash, "", errors.New("invalid Git URL")
		}
	}

	if paths[0] == "tree" {
		if len(paths) >= 2 {
			// If it looks like a SHA, use that.
			if gitSHARegex.MatchString(paths[1]) {
				return "", plumbing.NewHash(paths[1]), strings.Join(paths[2:], "/"), nil
			}

			// Otherwise, try matching based on the repo's refs.

			// Get the list of refs sorted by length.
			refs, err := GitListBranchesAndTags(url)
			if err != nil {
				return "", plumbing.ZeroHash, "", err
			}

			// Try to find the matching ref, checking the longest names first, so
			// if there are multiple refs that would match, we pick the longest.
			path := strings.Join(paths[1:], "/") + "/"
			for _, ref := range refs {
				shortName := ref.Short()
				prefix := shortName + "/"
				if strings.HasPrefix(path, prefix) {
					subDir := strings.TrimPrefix(path, prefix)
					return ref, plumbing.ZeroHash, strings.TrimSuffix(subDir, "/"), nil
				}
			}
		}

		// If there aren't any path components after "tree", it's an error.
		return "", plumbing.ZeroHash, "", errors.New("invalid Git URL")
	}

	// If there wasn't "tree" in the path, just use HEAD.
	return plumbing.HEAD, plumbing.ZeroHash, strings.Join(paths, "/"), nil
}

// GitListBranchesAndTags fetches a remote Git repository's branch and tag references
// (including HEAD), sorted by the length of the short name descending.
func GitListBranchesAndTags(url string) ([]plumbing.ReferenceName, error) {
	// We're only listing the references, so just use in-memory storage.
	repo, err := git.Init(memory.NewStorage(), nil)
	if err != nil {
		return nil, err
	}

	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})
	if err != nil {
		return nil, err
	}

	_, auth, err := parseAuthURL(url)
	if err != nil {
		return nil, err
	}

	refs, err := remote.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return nil, err
	}

	var results []plumbing.ReferenceName
	for _, ref := range refs {
		name := ref.Name()
		if name == plumbing.HEAD || name.IsBranch() || name.IsTag() {
			results = append(results, name)
		}
	}

	sort.Sort(byShortNameLengthDesc(results))

	return results, nil
}

type byShortNameLengthDesc []plumbing.ReferenceName

func (r byShortNameLengthDesc) Len() int      { return len(r) }
func (r byShortNameLengthDesc) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byShortNameLengthDesc) Less(i, j int) bool {
	return len(r[j].Short()) < len(r[i].Short())
}
