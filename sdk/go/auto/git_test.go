package auto

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

// This takes the unusual step of testing an unexported func. The rationale is to be able to test
// git code in isolation; testing the user of the unexported func (NewLocalWorkspace) drags in lots
// of other factors.

func TestGitBranch(t *testing.T) {
	t.Parallel()

	// This makes a git repo to clone from, so to avoid relying on something at GitHub that could
	// change or be inaccessible.
	tmpDir, err := os.MkdirTemp("", "pulumi-git-test")
	assert.NoError(t, err)
	assert.True(t, len(tmpDir) > 1)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	originDir := filepath.Join(tmpDir, "origin")

	origin, err := git.PlainInit(originDir, false)
	assert.NoError(t, err)
	w, err := origin.Worktree()
	assert.NoError(t, err)
	nondefaultHead, err := w.Commit("nondefault branch", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "testo",
			Email: "testo@example.com",
		},
	})
	assert.NoError(t, err)

	// this sets up two branches: `nondefault` and `default`, with `default` becoming the "default"
	// branch when cloning, since it's left as the HEAD of the repo.
	assert.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("nondefault"),
		Create: true,
	}))

	assert.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("default"),
		Create: true,
	}))
	defaultHead, err := w.Commit("default branch", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "testo",
			Email: "testo@example.com",
		},
	})
	assert.NoError(t, err)

	type testcase struct {
		branchName   string
		expectedHead plumbing.Hash
	}

	for _, tc := range []testcase{
		{"default", defaultHead},
		{"nondefault", nondefaultHead},
	} {
		tc := tc
		t.Run(tc.branchName, func(t *testing.T) {
			t.Parallel()
			repo := &GitRepo{
				URL:    originDir,
				Branch: tc.branchName,
			}

			tmp, err := os.MkdirTemp(tmpDir, "testcase") // i.e., under the tmp dir from earlier
			assert.NoError(t, err)

			_, err = setupGitRepo(context.TODO(), tmp, repo)
			assert.NoError(t, err)

			r, err := git.PlainOpen(tmp)
			assert.NoError(t, err)
			head, err := r.Head()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedHead, head.Hash())
		})
	}
}
