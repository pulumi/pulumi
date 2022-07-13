package auto

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// This takes the unusual step of testing an unexported func. The rationale is to be able to test
// git code in isolation; testing the user of the unexported func (NewLocalWorkspace) drags in lots
// of other factors.

func TestGitShortBranch(t *testing.T) {
	t.Parallel()
	repo := &GitRepo{
		URL:    "https://github.com/pulumi/test-repo.git",
		Branch: "master",
	}
	tmp, err := os.MkdirTemp("", "pulumi-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmp)
	_, err = setupGitRepo(context.TODO(), tmp, repo)
	assert.NoError(t, err)

	r, err := git.PlainOpen(tmp)
	assert.NoError(t, err)
	head, err := r.Head()
	assert.NoError(t, err)
	assert.Equal(t, plumbing.NewBranchReferenceName("master"), head.Name())
}
