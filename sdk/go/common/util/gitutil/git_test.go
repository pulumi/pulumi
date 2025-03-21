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
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func TestParseGitRepoURL(t *testing.T) {
	t.Parallel()

	test := func(expectedURL, expectedURLPath string, rawurl string) {
		actualURL, actualURLPath, err := ParseGitRepoURL(rawurl)
		assert.NoError(t, err)
		assert.Equal(t, expectedURL, actualURL)
		assert.Equal(t, expectedURLPath, actualURLPath)
	}

	// Azure DevOps.
	{
		// Azure DevOps uses a different scheme for URLs.
		// specifying a subdir and git ref is not currently supported.
		{
			url := "https://dev.azure.com/account-name/project-name/_git/repo-name"
			test(url, "", url)
		}

		{
			url := "https://dev.azure.com/tree/tree/_git/repo-name"
			test(url, "", url)
		}
	}

	// GitHub.
	pre := "https://github.com/pulumi/templates"
	exp := pre + ".git"
	test(exp, "", pre+".git")
	test(exp, "", pre)
	test(exp, "", pre+"/")
	test(exp, "templates", pre+"/templates")
	test(exp, "templates", pre+"/templates/")
	test(exp, "templates/javascript", pre+"/templates/javascript")
	test(exp, "templates/javascript", pre+"/templates/javascript/")
	test(exp, "tree/master/templates", pre+"/tree/master/templates")
	test(exp, "tree/master/templates/python", pre+"/tree/master/templates/python")
	test(exp, "tree/929b6e4c5c39196ae2482b318f145e0d765e9608/templates",
		pre+"/tree/929b6e4c5c39196ae2482b318f145e0d765e9608/templates")
	test(exp, "tree/929b6e4c5c39196ae2482b318f145e0d765e9608/templates/python",
		pre+"/tree/929b6e4c5c39196ae2482b318f145e0d765e9608/templates/python")

	// Gists.
	pre = "https://gist.github.com/user/1c8c6e43daf20924287c0d476e17de9a"
	exp = "https://gist.github.com/1c8c6e43daf20924287c0d476e17de9a.git"
	test(exp, "", pre)
	test(exp, "", pre+"/")

	// GitLab.
	pre = "https://gitlab.com/my-org/proj/subproj/doot.git/poc/waka-waka"
	exp = "https://gitlab.com/my-org/proj/subproj/doot.git"
	test(exp, "poc/waka-waka", pre)
	test(exp, "poc/waka-waka", pre+"/")
	pre = "https://gitlab.com/pulumi/platform/templates.git/templates/javascript"
	exp = "https://gitlab.com/pulumi/platform/templates.git"
	test(exp, "templates/javascript", pre)
	test(exp, "templates/javascript", pre+"///")
	pre = "https://gitlab.com/a/b/c/d/e/f/g/finally.git/1/2/3/4/5"
	exp = "https://gitlab.com/a/b/c/d/e/f/g/finally.git"
	test(exp, "1/2/3/4/5", pre)
	test(exp, "1/2/3/4/5", pre+"/")
	pre = "https://gitlab.com/dotgit/.git.git"
	exp = "https://gitlab.com/dotgit/.git.git"
	test(exp, "", pre)
	test(exp, "foobar", pre+"/foobar")
	pre = "https://user1:12345@gitlab.com/proj/finally.git"
	exp = "https://user1:12345@gitlab.com/proj/finally.git"
	test(exp, "", pre)
	test(exp, "foobar", pre+"/foobar")
	pre = "https://user1@gitlab.com/proj/finally.git"
	exp = "https://user1@gitlab.com/proj/finally.git"
	test(exp, "", pre)
	test(exp, "foobar", pre+"/foobar")
	pre = "https://user1:12345@gitlab.com/proj/finally"
	exp = pre + ".git"
	test(exp, "", pre)
	test(exp, "foobar", pre+"/foobar")

	// SSH URLs.
	pre = "git@github.com:acmecorp/templates"
	exp = "ssh://git@github.com/acmecorp/templates.git"
	test(exp, "", pre)
	test(exp, "", pre+"/")
	pre = "ssh://git@github.com/acmecorp/templates"
	exp = "ssh://git@github.com/acmecorp/templates.git"
	test(exp, "", pre)
	test(exp, "", pre+"/")
	pre = "git@github.com:acmecorp/templates/website"
	exp = "ssh://git@github.com/acmecorp/templates.git"
	test(exp, "website", pre)
	test(exp, "website", pre+"/")
	pre = "git@github.com:acmecorp/templates/somewhere/in/there/is/a/website"
	exp = "ssh://git@github.com/acmecorp/templates.git"
	test(exp, "somewhere/in/there/is/a/website", pre)
	test(exp, "somewhere/in/there/is/a/website", pre+"/")
	pre = "gitolite@acmecorp.com:acmecorp/templates/somewhere/in/there/is/a/website"
	exp = "ssh://gitolite@acmecorp.com/acmecorp/templates.git"
	test(exp, "somewhere/in/there/is/a/website", pre)
	test(exp, "somewhere/in/there/is/a/website", pre+"/")

	testError := func(rawurl string, expectedError string) {
		_, _, err := ParseGitRepoURL(rawurl)
		assert.EqualError(t, err, expectedError)
	}

	// No owner.
	testError("https://github.com", "invalid Git URL")
	testError("https://github.com/", "invalid Git URL")
	testError("git@github.com", "invalid URL scheme: ")
	testError("git@github.com/", "invalid URL scheme: ")
	testError("ssh://git@github.com", "invalid Git URL")
	testError("ssh://git@github.com/", "invalid Git URL")

	// No repo.
	testError("https://github.com/pulumi", "invalid Git URL")
	testError("https://github.com/pulumi/", "invalid Git URL; no repository")
	testError("git@github.com:pulumi", "invalid Git URL")
	testError("git@github.com:pulumi/", "invalid Git URL; no repository")
	testError("ssh://git@github.com/pulumi", "invalid Git URL")
	testError("ssh://git@github.com/pulumi/", "invalid Git URL; no repository")

	// Not HTTPS.
	testError("http://github.com/pulumi/templates.git", "invalid URL scheme: http")
	testError("http://github.com/pulumi/templates", "invalid URL scheme: http")
}

func TestGetGitReferenceNameOrHashAndSubDirectory(t *testing.T) {
	t.Parallel()

	e := ptesting.NewEnvironment(t)
	defer e.DeleteIfNotFailed()

	// Create local test repository.
	repoPath := filepath.Join(e.RootPath, "repo")
	err := os.MkdirAll(repoPath, os.ModePerm)
	assert.NoError(e, err, "making repo dir %s", repoPath)
	e.CWD = repoPath
	createTestRepo(e)

	// Create temp directory to clone to.
	cloneDir := filepath.Join(e.RootPath, "temp")
	err = os.MkdirAll(cloneDir, os.ModePerm)
	assert.NoError(e, err, "making clone dir %s", cloneDir)

	test := func(expectedHashOrBranch string, expectedSubDirectory string, urlPath string) {
		ref, hash, subDirectory, err := GetGitReferenceNameOrHashAndSubDirectory(repoPath, urlPath)
		assert.NoError(t, err)

		if ref != "" {
			assert.True(t, hash.IsZero())
			shortNameWithoutOrigin := strings.TrimPrefix(ref.Short(), "origin/")
			assert.Equal(t, expectedHashOrBranch, shortNameWithoutOrigin)
		} else {
			assert.False(t, hash.IsZero())
			assert.Equal(t, expectedHashOrBranch, hash.String())
		}

		assert.Equal(t, expectedSubDirectory, subDirectory)
	}

	// No ref or path.
	test("HEAD", "", "")
	test("HEAD", "", "/")

	// No "tree" path component.
	test("HEAD", "foo", "foo")
	test("HEAD", "foo", "foo/")
	test("HEAD", "content/foo", "content/foo")
	test("HEAD", "content/foo", "content/foo/")

	// master.
	test("master", "", "tree/master")
	test("master", "", "tree/master/")
	test("master", "foo", "tree/master/foo")
	test("master", "foo", "tree/master/foo/")
	test("master", "content/foo", "tree/master/content/foo")
	test("master", "content/foo", "tree/master/content/foo/")

	// HEAD.
	test("HEAD", "", "tree/HEAD")
	test("HEAD", "", "tree/HEAD/")
	test("HEAD", "foo", "tree/HEAD/foo")
	test("HEAD", "foo", "tree/HEAD/foo/")
	test("HEAD", "content/foo", "tree/HEAD/content/foo")
	test("HEAD", "content/foo", "tree/HEAD/content/foo/")

	// Tag.
	test("my", "", "tree/my")
	test("my", "", "tree/my/")
	test("my", "foo", "tree/my/foo")
	test("my", "foo", "tree/my/foo/")

	// Commit SHA.
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076")
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076/")
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "foo",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076/foo")
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "foo",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076/foo/")
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "content/foo",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076/content/foo")
	test("2ba6921f3163493809bcbb0ec7283a0446048076", "content/foo",
		"tree/2ba6921f3163493809bcbb0ec7283a0446048076/content/foo/")

	// The longest ref is matched, so we should get "my/content" as the expected ref
	// and "foo" as the path (instead of "my" and "content/foo").
	test("my/content", "foo", "tree/my/content/foo")
	test("my/content", "foo", "tree/my/content/foo/")

	testError := func(urlPath string, expectedError string) {
		_, _, _, err := GetGitReferenceNameOrHashAndSubDirectory(repoPath, urlPath)
		assert.EqualError(t, err, expectedError)
	}

	// No ref specified.
	testError("tree", "invalid Git URL")
	testError("tree/", "invalid Git URL")

	// Invalid casing.
	testError("tree/Master", "invalid Git URL")
	testError("tree/head", "invalid Git URL")
	testError("tree/My", "invalid Git URL")

	// Path components cannot contain "." or "..".
	testError(".", "invalid Git URL")
	testError("./", "invalid Git URL")
	testError("..", "invalid Git URL")
	testError("../", "invalid Git URL")
	testError("foo/.", "invalid Git URL")
	testError("foo/./", "invalid Git URL")
	testError("foo/..", "invalid Git URL")
	testError("foo/../", "invalid Git URL")
	testError("content/./foo", "invalid Git URL")
	testError("content/./foo/", "invalid Git URL")
	testError("content/../foo", "invalid Git URL")
	testError("content/../foo/", "invalid Git URL")
}

func createTestRepo(e *ptesting.Environment) {
	e.RunCommand("git", "init", "-b", "master")
	e.RunCommand("git", "config", "user.name", "test")
	e.RunCommand("git", "config", "user.email", "test@test.org")

	e.WriteTestFile("README.md", "test repo")
	e.RunCommand("git", "add", "*")
	e.RunCommand("git", "commit", "-m", "'Initial commit'")

	e.WriteTestFile("foo/bar.md", "foo-bar.md")
	e.RunCommand("git", "add", "*")
	e.RunCommand("git", "commit", "-m", "'foo dir'")

	e.WriteTestFile("content/foo/bar.md", "content-foo-bar.md")
	e.RunCommand("git", "add", "*")
	e.RunCommand("git", "commit", "-m", "'content-foo dir'")

	e.RunCommand("git", "branch", "my/content")
	e.RunCommand("git", "tag", "my")
}

func TestTryGetVCSInfoFromSSHRemote(t *testing.T) {
	t.Parallel()

	gitTests := []struct {
		Remote      string
		WantVCSInfo *VCSInfo
	}{
		// SSH remotes
		{
			"git@gitlab.com:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: GitLabHostName},
		},
		{
			"git@github.com:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: GitHubHostName},
		},
		{
			"git@bitbucket.org:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: BitbucketHostName},
		},
		{
			"git@ssh.dev.azure.com:v3/owner-name/project/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "project/_git/repo-name", Kind: AzureDevOpsHostName},
		},
		{
			"git@gitlab.com:owner-name/group/sub-group/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "group/sub-group/repo-name", Kind: GitLabHostName},
		},
		{
			"git@github.foo.acme.com:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: "github.foo.acme.com"},
		},
		{
			"git@github.foo.acme.org:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: "github.foo.acme.org"},
		},
		{
			"git@github.foo-acme.com:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: "github.foo-acme.com"},
		},
		{
			"git@github.foo-acme.org:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: "github.foo-acme.org"},
		},
		{
			"git@git.foo-acme.net:owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: "git.foo-acme.net"},
		},

		// HTTPS remotes
		{
			"https://gitlab-ci-token:dummytoken@gitlab.com/owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: GitLabHostName},
		},
		{
			"https://github.com/owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: GitHubHostName},
		},
		{
			"https://ploke@bitbucket.org/owner-name/repo-name.git",
			&VCSInfo{Owner: "owner-name", Repo: "repo-name", Kind: BitbucketHostName},
		},
		{
			"https://user@dev.azure.com/owner-name/project/_git/repo-name",
			&VCSInfo{Owner: "owner-name", Repo: "project/_git/repo-name", Kind: AzureDevOpsHostName},
		},
		{
			"https://owner-name.visualstudio.com/project/_git/repo-name",
			&VCSInfo{Owner: "owner-name", Repo: "project/_git/repo-name", Kind: AzureDevOpsHostName},
		},

		// Unknown or bad remotes
		{"", nil},
		{"asdf", nil},
		{"invalid ://../remote-url", nil},
		{"svn:something.com/owner/repo", nil},
		{"https://bitbucket.org/foo.git", nil},
		{"git@github.foo.acme.bad-tld:owner-name/repo-name.git", nil},
	}

	for _, test := range gitTests {
		got, err := TryGetVCSInfo(test.Remote)
		// Only assert the returned error if we don't expect to get an error.
		if test.WantVCSInfo != nil {
			assert.NoError(t, err)
		}
		assert.Equal(t, test.WantVCSInfo, got)
	}
}

// mockSSHConfig allows tests to mock SSH key paths.
type mockSSHConfig struct {
	path string
	err  error
}

// GetKeyPath returns a canned response for SSH config.
func (c *mockSSHConfig) GetStrict(host, key string) (string, error) {
	return c.path, c.err
}

//nolint:paralleltest // modifies environment variables
func TestParseAuthURL(t *testing.T) {
	//nolint: gosec
	generateSSHKey := func(t *testing.T, passphrase string) string {
		r := rand.New(rand.NewSource(0))
		key, err := rsa.GenerateKey(r, 256)
		require.NoError(t, err)

		block := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}

		if passphrase != "" {
			//nolint: staticcheck
			block, err = x509.EncryptPEMBlock(r, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
			require.NoError(t, err)
		}

		path := filepath.Join(t.TempDir(), "test-key")
		err = os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
		require.NoError(t, err)

		return path
	}

	t.Run("with no auth", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GITLAB_TOKEN", "")

		_, auth, err := getAuthForURL("http://github.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Nil(t, auth)
	})

	t.Run("with basic auth user", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GITLAB_TOKEN", "")

		url, auth, err := getAuthForURL("http://user@github.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Equal(t, &http.BasicAuth{Username: "user"}, auth)
		assert.Equal(t, "http://github.com/pulumi/templates", url)
	})

	t.Run("with basic auth user/password", func(t *testing.T) {
		t.Parallel()
		url, auth, err := getAuthForURL("http://user:password@github.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Equal(t, &http.BasicAuth{Username: "user", Password: "password"}, auth)
		assert.Equal(t, "http://github.com/pulumi/templates", url)
	})

	t.Run("with GITHUB_TOKEN set in environment", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "token-1")
		_, auth, err := getAuthForURL("http://github.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Equal(t, &http.BasicAuth{Username: "x-access-token", Password: "token-1"}, auth)

		_, auth, err = getAuthForURL("http://gitlab.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Nil(t, auth)
	})

	t.Run("with GITLAB_TOKEN set in environment", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", "token-1")
		t.Setenv("GITHUB_TOKEN", "")
		_, auth, err := getAuthForURL("http://gitlab.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Equal(t, &http.BasicAuth{Username: "oauth2", Password: "token-1"}, auth)

		_, auth, err = getAuthForURL("http://github.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Nil(t, auth)
	})

	t.Run("with GIT_USERNAME/GIT_PASSWORD set in environment", func(t *testing.T) {
		t.Setenv("GIT_USERNAME", "user")
		t.Setenv("GIT_PASSWORD", "password")
		_, auth, err := getAuthForURL("http://example.com/pulumi/templates")
		assert.NoError(t, err)
		assert.Equal(t, &http.BasicAuth{Username: "user", Password: "password"}, auth)
	})

	//nolint:paralleltest // global environment variables
	t.Run("with passphrase-protected key and environment variable", func(t *testing.T) {
		passphrase := "foobar"
		t.Setenv(env.GitSSHPassphrase.Var().Name(), passphrase)

		parser := urlAuthParser{
			sshConfig: &mockSSHConfig{path: generateSSHKey(t, passphrase)},
		}

		_, auth, err := parser.Parse("git@github.com:pulumi/templates.git")
		assert.NoError(t, err)
		assert.NotNil(t, auth)
		assert.Equal(t, "user: git, name: ssh-public-keys", auth.String())
		assert.Contains(t, parser.sshKeys, "github.com")
	})

	//nolint:paralleltest // global environment variables
	t.Run("with passphrase-protected key and wrong environment variable (agent available)", func(t *testing.T) {
		l, err := nettest.NewLocalListener("unix")
		defer contract.IgnoreClose(l)
		require.NoError(t, err)

		t.Setenv(env.GitSSHPassphrase.Var().Name(), "incorrect passphrase")
		t.Setenv("SSH_AUTH_SOCK", l.Addr().String())

		parser := urlAuthParser{
			sshConfig: &mockSSHConfig{path: generateSSHKey(t, "correct passphrase")},
		}

		_, auth, err := parser.Parse("git@github.com:pulumi/templates.git")
		// This isn't an error because the connection should fall back to the
		// SSH agent for auth.
		assert.NoError(t, err)
		assert.NotNil(t, auth)
	})

	//nolint:paralleltest // global environment variables
	t.Run("with passphrase-protected key and wrong environment variable (agent unavailable)", func(t *testing.T) {
		t.Setenv(env.GitSSHPassphrase.Var().Name(), "incorrect passphrase")
		t.Setenv("SSH_AUTH_SOCK", "")
		os.Unsetenv("SSH_AUTH_SOCK") // t.Setenv above will restore it after the test

		parser := urlAuthParser{
			sshConfig: &mockSSHConfig{path: generateSSHKey(t, "correct passphrase")},
		}

		_, auth, err := parser.Parse("git@github.com:pulumi/templates.git")
		assert.ErrorContains(t, err, "SSH_AUTH_SOCK not-specified")
		assert.Nil(t, auth)
	})

	t.Run("with memoized auth", func(t *testing.T) {
		t.Parallel()
		parser := urlAuthParser{
			sshConfig: &mockSSHConfig{err: errors.New("should not be called")},
			sshKeys: map[string]transport.AuthMethod{
				"github.com": &http.BasicAuth{Username: "foo"},
			},
		}

		_, auth, err := parser.Parse("git@github.com:pulumi/templates.git")
		assert.NoError(t, err)
		assert.NotNil(t, auth)
		assert.Equal(t, "http-basic-auth - foo:<empty>", auth.String())
	})

	//nolint:paralleltest // modifies environment variables
	t.Run("Don't cache on error", func(t *testing.T) {
		// Regression test for https://github.com/pulumi/pulumi/issues/16637
		t.Setenv(env.GitSSHPassphrase.Var().Name(), "incorrect passphrase")
		t.Setenv("SSH_AUTH_SOCK", "")
		os.Unsetenv("SSH_AUTH_SOCK") // t.Setenv above will restore it after the test

		parser := urlAuthParser{
			sshConfig: &mockSSHConfig{path: generateSSHKey(t, "correct passphrase")},
		}

		_, auth, err := parser.Parse("git@github.com:pulumi/templates.git")
		assert.ErrorContains(t, err, "SSH_AUTH_SOCK not-specified")
		assert.Nil(t, auth)

		// Retry, should still fail.
		_, auth, err = parser.Parse("git@github.com:pulumi/templates.git")
		assert.ErrorContains(t, err, "SSH_AUTH_SOCK not-specified")
		assert.Nil(t, auth)
	})
}

func TestGitCloneAndCheckoutRevision(t *testing.T) {
	t.Parallel()

	// Check that we can clone a repository and checkout a
	// specific revision.  We use a test repository that contains
	// a single file, "test".  It has two revisions, the initial
	// commit, where the file contains "initial", and the HEAD
	// commit, where the file contains "HEAD".  So just cloning
	// the repo should result in "HEAD", while checking out the
	// initial commit should result in "initial".
	cases := []struct {
		name            string
		revision        plumbing.Revision
		expectedContent string
		expectedError   string
	}{
		{
			name:            "full-hash",
			revision:        plumbing.Revision("c9bcda983f3078cf622b02acc625f6a2127f4e0a"),
			expectedContent: "initial\n",
		},
		{
			name:            "short-hash",
			revision:        plumbing.Revision("c9bcda"),
			expectedContent: "initial\n",
		},
		{
			name:            "full-hash",
			revision:        plumbing.Revision("HEAD"),
			expectedContent: "HEAD\n",
		},
		{
			name:          "unknown-revision",
			revision:      plumbing.Revision("deadbeef"),
			expectedError: "reference not found",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			err := GitCloneAndCheckoutRevision(context.Background(), "testdata/revision-test.git", c.revision, dir)
			if c.expectedError != "" {
				require.ErrorContains(t, err, c.expectedError)
				return
			}
			require.NoError(t, err)
			content, err := os.ReadFile(filepath.Join(dir, "test"))
			if err != nil {
				panic(err)
			}
			assert.Equal(t, c.expectedContent, string(content))
		})
	}
}

func TestGetLatestTagOrHash(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		expected semver.Version
		dataDir  string
	}{
		{
			name:     "no tags",
			expected: semver.MustParse("0.0.0-x044e5858f018bd6fa666da9adbff645f581fb91b"),
			dataDir:  "testdata/commit-only.git",
		},
		{
			name:     "tags",
			expected: semver.MustParse("0.1.1"),
			dataDir:  "testdata/tags.git",
		},
		{
			name:     "alpha version",
			expected: semver.MustParse("0.1.0-alpha"),
			dataDir:  "testdata/alpha-tag.git",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			v, err := GetLatestTagOrHash(context.Background(), c.dataDir)
			assert.NoError(t, err)
			assert.Equal(t, c.expected.String(), v.String())
		})
	}
}
