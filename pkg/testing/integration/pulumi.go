package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// CreateBasicPulumiRepo will initialize the environment with a basic Pulumi repository and
// project file definition. Returns the repo owner and name used.
func CreateBasicPulumiRepo(e *testing.Environment) (string, string) {
	e.RunCommand("git", "init")

	// We need to specify a well-known owner so that commands don't fail for reasons like
	// the organization isn't in the private beta. The repository name is randomly generated
	// so that testcases can assume no stacks, etc.
	const owner = "moolumi"
	name := fmt.Sprintf("integration-test-%x", time.Now().Unix())
	e.RunCommand("pulumi", "init", "--owner", owner, "--name", name)

	contents := "name: pulumi-test\ndescription: a test\nruntime: nodejs\n"
	filePath := fmt.Sprintf("%s.yaml", workspace.ProjectFile)
	filePath = path.Join(e.CWD, filePath)
	err := ioutil.WriteFile(filePath, []byte(contents), os.ModePerm)
	assert.NoError(e, err, "writing %s file", filePath)

	return owner, name
}

// GetRepository returns the contents of the workspace's repository settings file. Assumes the
// bookkeeping dir (.pulumi) is in the CWD. Any IO errors fails the test.
func GetRepository(e *testing.Environment) workspace.Repository {
	relativePathToRepoFile := fmt.Sprintf("%s/%s", workspace.BookkeepingDir, workspace.RepoFile)
	if !e.PathExists(relativePathToRepoFile) {
		e.Fatalf("did not find .pulumi/settings.json")
	}

	path := path.Join(e.CWD, relativePathToRepoFile)
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		e.Fatalf("error reading %s: %v", workspace.RepoFile, err)
	}

	var repo workspace.Repository
	err = json.Unmarshal(contents, &repo)
	if err != nil {
		e.Fatalf("error unmarshalling JSON: %v", err)
	}

	return repo
}

// GetStacks returns the list of stacks and current stack by scraping `pulumi stack ls`.
// Assumes .pulumi is in the current working directory. Fails the test on IO errors.
func GetStacks(e *testing.Environment) ([]string, *string) {
	out, _ := e.RunCommand("pulumi", "stack", "ls")

	outLines := strings.Split(out, "\n")
	if len(outLines) == 0 {
		e.Fatalf("command didn't output as expected")
	}

	// Confirm header row matches.
	// TODO(pulumi/pulumi/issues/496): Provide structured output for pulumi commands. e.g., so we can avoid this
	// err-prone scraping with just deserializings a JSON object.
	assert.True(e, strings.HasPrefix(outLines[0], "NAME"))

	var stackNames []string
	var currentStack *string
	stackSummaries := outLines[1:]
	for _, summary := range stackSummaries {
		if summary == "" {
			break
		}
		firstSpace := strings.Index(summary, " ")
		if firstSpace != -1 {
			stackName := strings.TrimSpace(summary[:firstSpace])
			if strings.HasSuffix(stackName, "*") {
				currentStack = &stackName
				stackName = strings.TrimSuffix(stackName, "*")
			}
			stackNames = append(stackNames, stackName)
		}
	}

	return stackNames, currentStack
}

// TestAccountAccessToken is a Pulumi access token of a well-known test account. Most importantly, we can
// rely on this test account existing in all Pulumi environments. GitHub user "lumi-test-1". See
// pulumi-service/cmd/service/model/test.go for more information.
// nolint: gas
const TestAccountAccessToken = "gGo1WLk2Jh_NZlqo2FFvjPtBvo73IW3xsJvBRmDOPW4="
