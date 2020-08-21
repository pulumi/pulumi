package auto

import (
	"path/filepath"

	"github.com/pkg/errors"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func setupGitRepo(workDir string, repoArgs *GitRepo) (string, error) {
	// clone
	repo, err := git.PlainClone(workDir, false, &git.CloneOptions{URL: repoArgs.URL})
	if err != nil {
		return "", errors.Wrap(err, "unable to clone repo")
	}

	// checkout branch if specified
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	var hash string
	if repoArgs.CommitHash != "" {
		hash = repoArgs.CommitHash
	}
	var branch string
	if repoArgs.Branch != "" {
		branch = repoArgs.Branch
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
	if repoArgs.ProjectPath != "" {
		relPath = repoArgs.ProjectPath
	}

	workDir = filepath.Join(workDir, relPath)

	// setup
	if repoArgs.Setup != nil {
		err = repoArgs.Setup(workDir)
		if err != nil {
			return "", errors.Wrap(err, "error while running setup function")
		}
	}

	return workDir, nil
}
