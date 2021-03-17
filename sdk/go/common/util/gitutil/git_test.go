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
	"os"
	"path/filepath"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/assert"
)

func TestParseGitRepoURL(t *testing.T) {
	test := func(expectedURL string, expectedURLPath string, rawurl string) {
		actualURL, actualURLPath, err := ParseGitRepoURL(rawurl)
		assert.NoError(t, err)
		assert.Equal(t, expectedURL, actualURL)
		assert.Equal(t, expectedURLPath, actualURLPath)
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

	testError := func(rawurl string) {
		_, _, err := ParseGitRepoURL(rawurl)
		assert.Error(t, err)
	}

	// No owner.
	testError("https://github.com")
	testError("https://github.com/")

	// No repo.
	testError("https://github.com/pulumi")
	testError("https://github.com/pulumi/")

	// Not HTTPS.
	testError("http://github.com/pulumi/templates.git")
	testError("http://github.com/pulumi/templates")
}

func TestGetGitReferenceNameOrHashAndSubDirectory(t *testing.T) {
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

	testError := func(urlPath string) {
		_, _, _, err := GetGitReferenceNameOrHashAndSubDirectory(repoPath, urlPath)
		assert.Error(t, err)
	}

	// No ref specified.
	testError("tree")
	testError("tree/")

	// Invalid casing.
	testError("tree/Master")
	testError("tree/head")
	testError("tree/My")

	// Path components cannot contain "." or "..".
	testError(".")
	testError("./")
	testError("..")
	testError("../")
	testError("foo/.")
	testError("foo/./")
	testError("foo/..")
	testError("foo/../")
	testError("content/./foo")
	testError("content/./foo/")
	testError("content/../foo")
	testError("content/../foo/")
}

func createTestRepo(e *ptesting.Environment) {
	e.RunCommand("git", "init")

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

		//HTTPS remotes
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

		//Unknown or bad remotes
		{"", nil},
		{"asdf", nil},
		{"svn:something.com/owner/repo", nil},
	}

	for _, test := range gitTests {
		got, err := TryGetVCSInfo(test.Remote)
		// Only assert the returned error if we don't expect to get an error.
		if test.WantVCSInfo != nil {
			assert.Nil(t, err)
		}
		assert.Equal(t, test.WantVCSInfo, got)
	}
}
