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
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

// VCSKind represents the hostname of a specific type of VCS.
// For eg., github.com, gitlab.com etc.
type VCSKind = string

// Constants related to detecting the right type of source control provider for git.
const (
	defaultGitCloudRepositorySuffix = ".git"

	// The host name for GitLab.
	GitLabHostName VCSKind = "gitlab.com"
	// The host name for GitHub.
	GitHubHostName VCSKind = "github.com"
	// The host name for Azure DevOps
	AzureDevOpsHostName VCSKind = "dev.azure.com"
	// The host name for Bitbucket
	BitbucketHostName VCSKind = "bitbucket.org"
)

// The pre-compiled regex used to extract owner and repo name from an SSH git remote URL.
// Note: If you are renaming any of the group names in the regex (the ?P<group_name> part) to something else,
// be sure to update its usage elsewhere in the code as well.
// The nolint instruction prevents gometalinter from complaining about the length of the line.
var (
	cloudSourceControlSSHRegex    = regexp.MustCompile(`git@(?P<host_name>[a-zA-Z]*\.com|[a-zA-Z]*\.org):(?P<owner_and_repo>.*)`)                           //nolint
	azureSourceControlSSHRegex    = regexp.MustCompile(`git@([a-zA-Z]+\.)?(?P<host_name>([a-zA-Z]+\.)*[a-zA-Z]*\.com):(v[0-9]{1}/)?(?P<owner_and_repo>.*)`) //nolint
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
		return nil, errors.Wrapf(err, "searching for git repository from %v", dir)
	}
	if gitRoot == "" {
		return nil, nil
	}

	// Open the git repo in the .git folder's parent, not the .git folder itself.
	repo, err := git.PlainOpen(filepath.Dir(gitRoot))
	if err == git.ErrRepositoryNotExists {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "reading git repository")
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
		return "", errors.Wrap(err, "could not read origin information")
	}

	remoteURL := ""
	if len(remote.Config().URLs) > 0 {
		remoteURL = remote.Config().URLs[0]
	}

	return remoteURL, nil
}

// IsGitOriginURLGitHub returns true if the provided remoteURL is detected as GitHub.
func IsGitOriginURLGitHub(remoteURL string) bool {
	return strings.Contains(remoteURL, GitHubHostName)
}

// TryGetVCSInfo attempts to detect whether the provided remoteURL
// is an SSH or an HTTPS remote URL. It then extracts the repo, owner name,
// and the type (kind) of VCS from it.
func TryGetVCSInfo(remoteURL string) (*VCSInfo, error) {
	project := ""
	vcsKind := ""

	// If the remote is using git SSH, then we extract the named groups by matching
	// with the pre-compiled regex pattern.
	if strings.HasPrefix(remoteURL, "git@") {
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
	} else if strings.HasPrefix(remoteURL, "http") {
		// This could be an HTTP(S)-based remote.
		if parsedURL, err := url.Parse(remoteURL); err == nil {
			vcsKind = parsedURL.Host
			project = parsedURL.Path
			// Replace the .git extension from the path.
			project = strings.TrimSuffix(project, defaultGitCloudRepositorySuffix)
			// Remove the prefix "/". TrimPrefix returns the same value if there is no prefix.
			// So it is safe to use it instead of doing any sort of substring matches.
			project = strings.TrimPrefix(project, "/")
		}
	}

	if project == "" {
		return nil, errors.Errorf("detecting the VCS info from the remote URL %v", remoteURL)
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
		return nil, errors.Errorf("could not detect VCS project from url: %v", remoteURL)
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

// GitCloneAndCheckoutCommit clones the Git repository and checkouts the specified commit.
func GitCloneAndCheckoutCommit(url string, commit plumbing.Hash, path string) error {
	repo, err := git.PlainClone(path, false, &git.CloneOptions{
		URL: url,
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

// GitCloneOrPull clones or updates the specified referenceName (branch or tag) of a Git repository.
func GitCloneOrPull(url string, referenceName plumbing.ReferenceName, path string, shallow bool) error {
	// For shallow clones, use a depth of 1.
	depth := 0
	if shallow {
		depth = 1
	}

	// Attempt to clone the repo.
	_, cloneErr := git.PlainClone(path, false, &git.CloneOptions{
		URL:           url,
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

			if err = w.Pull(&git.PullOptions{
				ReferenceName: referenceName,
				SingleBranch:  true,
				Force:         true,
			}); err != nil && err != git.NoErrAlreadyUpToDate {
				return err
			}
		} else {
			return cloneErr
		}
	}

	return nil
}

// ParseGitRepoURL returns the URL to the Git repository and path from a raw URL.
// For example, an input of "https://github.com/pulumi/templates/templates/javascript" returns
// "https://github.com/pulumi/templates.git" and "templates/javascript".
func ParseGitRepoURL(rawurl string) (string, string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", "", err
	}

	if u.Scheme != "https" {
		return "", "", errors.New("invalid URL scheme")
	}

	path := strings.TrimPrefix(u.Path, "/")

	// Special case Gists.
	if u.Hostname() == "gist.github.com" {
		// We currently accept Gist URLs in the form: https://gist.github.com/owner/id.
		// We may want to consider supporting https://gist.github.com/id at some point,
		// as well as arbitrary revisions, e.g. https://gist.github.com/owner/id/commit.
		path = strings.TrimSuffix(path, "/")
		paths := strings.Split(path, "/")
		if len(paths) != 2 {
			return "", "", errors.New("invalid Gist URL")
		}

		owner := paths[0]
		if owner == "" {
			return "", "", errors.New("invalid Gist URL; no owner")
		}

		id := paths[1]
		if id == "" {
			return "", "", errors.New("invalid Gist URL; no id")
		}

		if !strings.HasSuffix(id, ".git") {
			id = id + ".git"
		}

		resultURL := u.Scheme + "://" + u.Host + "/" + id
		return resultURL, "", nil
	}

	paths := strings.Split(path, "/")
	if len(paths) < 2 {
		return "", "", errors.New("invalid Git URL")
	}

	owner := paths[0]
	if owner == "" {
		return "", "", errors.New("invalid Git URL; no owner")
	}

	repo := paths[1]
	if repo == "" {
		return "", "", errors.New("invalid Git URL; no repository")
	}

	if !strings.HasSuffix(repo, ".git") {
		repo = repo + ".git"
	}

	resultURL := u.Scheme + "://" + u.Host + "/" + owner + "/" + repo
	resultPath := strings.TrimSuffix(strings.Join(paths[2:], "/"), "/")
	return resultURL, resultPath, nil
}

var gitSHARegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// GetGitReferenceNameOrHashAndSubDirectory returns the reference name or hash, and sub directory path.
// The sub directory path always uses "/" as the separator.
func GetGitReferenceNameOrHashAndSubDirectory(url string, urlPath string) (
	plumbing.ReferenceName, plumbing.Hash, string, error) {

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

	refs, err := remote.List(&git.ListOptions{})
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
