package auto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/gitutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

// This takes the unusual step of testing an unexported func. The rationale is to be able to test
// git code in isolation; testing the user of the unexported func (NewLocalWorkspace) drags in lots
// of other factors.

func TestGitClone(t *testing.T) {
	t.Parallel()

	// This makes a git repo to clone from, so to avoid relying on something at GitHub that could
	// change or be inaccessible.
	tmpDir := t.TempDir()
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
		AllowEmptyCommits: true,
	})
	assert.NoError(t, err)

	// The following sets up some tags and branches: with `default` becoming the "default" branch
	// when cloning, since it's left as the HEAD of the repo.

	assert.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("nondefault"),
		Create: true,
	}))

	// tag the nondefault head so we can test getting a tag too
	_, err = origin.CreateTag("v0.0.1", nondefaultHead, nil)
	assert.NoError(t, err)

	// make a branch with slashes in it, so that can be tested too
	assert.NoError(t, w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("branch/with/slashes"),
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
		AllowEmptyCommits: true,
	})
	assert.NoError(t, err)

	type testcase struct {
		branchName    string
		commitHash    string
		testName      string // use when supplying a hash, for a stable name
		expectedHead  plumbing.Hash
		expectedError string
	}

	for _, tc := range []testcase{
		{branchName: "default", expectedHead: defaultHead},
		{branchName: "nondefault", expectedHead: nondefaultHead},
		{branchName: "branch/with/slashes", expectedHead: nondefaultHead},
		// https://github.com/pulumi/pulumi-kubernetes-operator/issues/103#issuecomment-1107891475
		// advises using `refs/heads/<default>` for the default, and `refs/remotes/origin/<branch>`
		// for a non-default branch -- so we can expect all these varieties to be in use.
		{branchName: "refs/heads/default", expectedHead: defaultHead},
		{branchName: "refs/heads/nondefault", expectedHead: nondefaultHead},
		{branchName: "refs/heads/branch/with/slashes", expectedHead: nondefaultHead},

		{branchName: "refs/remotes/origin/default", expectedHead: defaultHead},
		{branchName: "refs/remotes/origin/nondefault", expectedHead: nondefaultHead},
		{branchName: "refs/remotes/origin/branch/with/slashes", expectedHead: nondefaultHead},
		// try the special tag case
		{branchName: "refs/tags/v0.0.1", expectedHead: nondefaultHead},
		// ask specifically for the commit hash
		{testName: "head of default as hash", commitHash: defaultHead.String(), expectedHead: defaultHead},
		{testName: "head of nondefault as hash", commitHash: nondefaultHead.String(), expectedHead: nondefaultHead},
	} {
		tc := tc
		if tc.testName == "" {
			tc.testName = tc.branchName
		}
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			repo := &GitRepo{
				URL:        originDir,
				Branch:     tc.branchName,
				CommitHash: tc.commitHash,
			}

			tmp, err := os.MkdirTemp(tmpDir, "testcase") // i.e., under the tmp dir from earlier
			assert.NoError(t, err)

			_, err = setupGitRepo(context.Background(), tmp, repo)
			assert.NoError(t, err)

			r, err := git.PlainOpen(tmp)
			assert.NoError(t, err)
			head, err := r.Head()
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedHead, head.Hash())
		})
	}

	// test that these result in errors
	for _, tc := range []testcase{
		{
			testName:      "simple branch doesn't exist",
			branchName:    "doesnotexist",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "full branch doesn't exist",
			branchName:    "refs/heads/doesnotexist",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "malformed branch name",
			branchName:    "refs/notathing/default",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:      "simple tag name won't work",
			branchName:    "v1.0.0",
			expectedError: "unable to clone repo: reference not found",
		},
		{
			testName:   "wrong remote",
			branchName: "refs/remotes/upstream/default",
			expectedError: "a remote ref must begin with 'refs/remote/origin/', " +
				"but got \"refs/remotes/upstream/default\"",
		},
	} {
		tc := tc
		if tc.testName == "" {
			tc.testName = tc.branchName
		}
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			repo := &GitRepo{
				URL:        originDir,
				Branch:     tc.branchName,
				CommitHash: tc.commitHash,
			}

			tmp, err := os.MkdirTemp(tmpDir, "testcase") // i.e., under the tmp dir from earlier
			assert.NoError(t, err)

			_, err = setupGitRepo(context.Background(), tmp, repo)
			assert.EqualError(t, err, tc.expectedError)
		})
	}
}

//nolint:paralleltest // global environment variables
func TestGitAuthParse(t *testing.T) {
	// Set up a fake SSH_AUTH_SOCK for testing
	l, err := nettest.NewLocalListener("unix")
	defer contract.IgnoreClose(l)
	require.NoError(t, err)

	type testcase struct {
		name              string
		url               string
		auth              *GitAuth
		setSSHAgentSocket bool

		expectedURL                 string
		expectedError               string
		expectedPublicKeysTransport bool

		// Expect a basic auth instance with the provided username and password
		expectedBasicAuth *http.BasicAuth
		// The same as the above, but expects a TransportAuth wrapper
		expectedWrappedBasicAuth *http.BasicAuth
	}

	for _, tc := range []testcase{
		{
			name: "private key path",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				SSHPrivateKeyPath: generateSSHKeyFile(t),
			},
			expectedURL:                 "git@example.com:repo.git",
			expectedPublicKeysTransport: true,
		},
		{
			name: "invalid private key path",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				SSHPrivateKeyPath: "/invalid/path",
			},
			expectedError: "unable to use SSH Private Key Path: open /invalid/path",
		},
		{
			name: "private key value",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				SSHPrivateKey: generateSSHKeyString(t),
			},
			expectedURL:                 "git@example.com:repo.git",
			expectedPublicKeysTransport: true,
		},
		{
			name: "invalid private key value",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				SSHPrivateKey: "invalid-string",
			},
			expectedError: "unable to use SSH Private Key: ssh: no key found",
		},
		{
			name: "personal access token",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				PersonalAccessToken: "dummy-token",
			},
			expectedURL:       "git@example.com:repo.git",
			expectedBasicAuth: &http.BasicAuth{Username: "git", Password: "dummy-token"},
		},
		{
			name: "username and password",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				Username: "user",
				Password: "pass",
			},
			expectedURL:       "git@example.com:repo.git",
			expectedError:     "",
			expectedBasicAuth: &http.BasicAuth{Username: "user", Password: "pass"},
		},
		{
			name: "incompatible auth options",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				SSHPrivateKeyPath: "/path/to/private/key",
				Username:          "user",
			},
			expectedError: "please specify one authentication option",
		},
		{
			name: "incompatible auth options",
			url:  "git@example.com:repo.git",
			auth: &GitAuth{
				PersonalAccessToken: "dummy-token",
				Username:            "user",
			},
			expectedError: "please specify one authentication option",
		},
		{
			url:               "git@example.com:repo.git",
			name:              "default auth with SSH agent",
			setSSHAgentSocket: true,

			expectedURL: "git@example.com:repo.git",
		},
		{
			name: "url with basic auth",
			url:  "https://user:password@example.com/repo.git",

			expectedURL: "https://example.com/repo.git",
			expectedWrappedBasicAuth: &http.BasicAuth{
				Username: "user",
				Password: "password",
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.setSSHAgentSocket {
				t.Setenv("SSH_AUTH_SOCK", l.Addr().String())
			} else {
				os.Unsetenv("SSH_AUTH_SOCK")
			}

			url, authMethod, err := func() (string, transport.AuthMethod, error) {
				var _ context.Context = context.Background()
				return setupGitRepoAuth(tc.url, tc.auth)
			}()

			if tc.expectedError != "" {
				if err == nil {
					t.Fatalf("expected an error but got nil")
				}
				if !strings.Contains(err.Error(), tc.expectedError) {
					t.Fatalf("expected error %q to contain %q", err.Error(), tc.expectedError)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if url != tc.expectedURL {
				t.Errorf("expected url %q, got %q", tc.expectedURL, url)
			}

			if _, ok := authMethod.(*ssh.PublicKeys); ok != tc.expectedPublicKeysTransport {
				condition := "to be"
				if !tc.expectedPublicKeysTransport {
					condition = "not to be"
				}
				t.Errorf("expected transport %s of type *ssh.PublicKeys, got %T", condition, authMethod)
			}

			if tc.expectedBasicAuth != nil {
				basicAuth, ok := authMethod.(*http.BasicAuth)
				require.Truef(t, ok, "expected Auth to be of type *http.BasicAuth, got %T", authMethod)
				if basicAuth.Username != tc.expectedBasicAuth.Username ||
					basicAuth.Password != tc.expectedBasicAuth.Password {
					t.Errorf("expected BasicAuth to have username %q and password %q, got username %q and password %q",
						tc.expectedBasicAuth.Username, tc.expectedBasicAuth.Password,
						basicAuth.Username, basicAuth.Password)
				}
			}

			if tc.expectedWrappedBasicAuth != nil {
				wrapper, ok := authMethod.(*gitutil.TransportAuth)
				require.Truef(t, ok, "expected transport to be of type *gitutil.TransportAuth, got %T", authMethod)
				basicAuth, ok := wrapper.AuthMethod.(*http.BasicAuth)
				require.Truef(t, ok, "expected Auth to be of type *http.BasicAuth, got %T", wrapper.AuthMethod)
				if basicAuth.Username != tc.expectedWrappedBasicAuth.Username ||
					basicAuth.Password != tc.expectedWrappedBasicAuth.Password {
					t.Errorf("expected BasicAuth to have username %q and password %q, got username %q and password %q",
						tc.expectedWrappedBasicAuth.Username, tc.expectedWrappedBasicAuth.Password,
						basicAuth.Username, basicAuth.Password)
				}
			}
		})
	}
}

func generateSSHKeyBlock(t *testing.T) *pem.Block {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	return &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
}

func generateSSHKeyFile(t *testing.T) string {
	block := generateSSHKeyBlock(t)

	path := filepath.Join(t.TempDir(), "test-key")
	err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
	t.Cleanup(func() {
		os.Remove(path)
	})
	require.NoError(t, err)

	return path
}

func generateSSHKeyString(t *testing.T) string {
	block := generateSSHKeyBlock(t)

	return string(pem.EncodeToMemory(block))
}
