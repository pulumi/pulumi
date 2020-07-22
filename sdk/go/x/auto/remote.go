package auto

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func setupRemote(remote *RemoteArgs) (string, error) {
	// clone
	var enlistPath string
	if remote.WorkDir != nil {
		enlistPath = *remote.WorkDir
	} else {
		p, err := ioutil.TempDir("", "auto")
		enlistPath = p
		if err != nil {
			return "", errors.Wrap(err, "error enlisting in remote")
		}
	}

	repo, err := git.PlainClone(enlistPath, false, &git.CloneOptions{URL: remote.RepoURL})
	if err != nil {
		return "", errors.Wrap(err, "unable to clone repo")
	}

	// checkout branch if specified
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	var hash string
	if remote.CommitHash != nil {
		hash = *remote.CommitHash
	}
	var branch string
	if remote.Branch != nil {
		branch = *remote.Branch
	}

	err = w.Checkout(&git.CheckoutOptions{
		Hash:   plumbing.NewHash(hash),
		Branch: plumbing.ReferenceName(branch),
		Force:  true,
	})
	if err != nil {
		return "", errors.Wrap(err, "unable to checkout branch")
	}

	var relPath string
	if remote.ProjectPath != nil {
		relPath = *remote.ProjectPath
	}

	projectDiskPath := filepath.Join(enlistPath, relPath)

	// setup
	if remote.Setup != nil {
		err = remote.Setup(projectDiskPath)
		if err != nil {
			return "", errors.Wrap(err, "error while running setup function")
		}
	}

	return projectDiskPath, nil
}
