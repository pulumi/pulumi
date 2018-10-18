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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

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
	repo, err := git.PlainOpen(path.Join(gitRoot, ".."))
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
func GetGitHubProjectForOrigin(dir string) (string, string, error) {
	repo, err := GetGitRepository(dir)
	if repo == nil {
		return "", "", fmt.Errorf("no git repository found from %v", dir)
	}
	if err != nil {
		return "", "", err
	}
	return GetGitHubProjectForOriginByRepo(repo)
}

// GetGitHubProjectForOriginByRepo returns the GitHub login, and GitHub repo name if the "origin" remote is
// a GitHub URL.
func GetGitHubProjectForOriginByRepo(repo *git.Repository) (string, string, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", errors.Wrap(err, "could not read origin information")
	}

	remoteURL := ""
	if len(remote.Config().URLs) > 0 {
		remoteURL = remote.Config().URLs[0]
	}
	project := ""

	const GitHubSSHPrefix = "git@github.com:"
	const GitHubHTTPSPrefix = "https://github.com/"
	const GitHubRepositorySuffix = ".git"

	if strings.HasPrefix(remoteURL, GitHubSSHPrefix) {
		project = trimGitRemoteURL(remoteURL, GitHubSSHPrefix, GitHubRepositorySuffix)
	} else if strings.HasPrefix(remoteURL, GitHubHTTPSPrefix) {
		project = trimGitRemoteURL(remoteURL, GitHubHTTPSPrefix, GitHubRepositorySuffix)
	}

	split := strings.Split(project, "/")

	if len(split) != 2 {
		return "", "", errors.Errorf("could not detect GitHub project from url: %v", remote)
	}

	return split[0], split[1], nil
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

func trimGitRemoteURL(url string, prefix string, suffix string) string {
	return strings.TrimSuffix(strings.TrimPrefix(url, prefix), suffix)
}

type byShortNameLengthDesc []plumbing.ReferenceName

func (r byShortNameLengthDesc) Len() int      { return len(r) }
func (r byShortNameLengthDesc) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byShortNameLengthDesc) Less(i, j int) bool {
	return len(r[j].Short()) < len(r[i].Short())
}
