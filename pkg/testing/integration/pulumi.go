package integration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

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
	out, err := e.RunCommand("pulumi", "stack", "ls")
	assert.Equal(e, "", err, "expected nothing written to stderr")

	outLines := strings.Split(out, "\n")

	var stackNames []string
	var currentStack *string
	if len(outLines) == 0 {
		e.Fatalf("command didn't output as expected")
	}
	// Confirm header row matches.
	// TODO(pulumi/pulumi/issues/496): Provide structured output for pulumi commands. e.g., so we can avoid this
	// err-prone scraping with just deserializings a JSON object.
	assert.Equal(e, outLines[0], "NAME                 LAST UPDATE                                      RESOURCE COUNT")

	if len(outLines) >= 2 {
		stackSummaries := outLines[1:]
		for _, summary := range stackSummaries {
			if summary == "" {
				continue // Last line of stdout is "".
			}
			stackName := strings.TrimSpace(summary[:20])
			if strings.HasSuffix(stackName, "*") {
				currentStack = &stackName
				stackName = strings.TrimSuffix(stackName, "*")
			}
			stackNames = append(stackNames, stackName)
		}
	}

	return stackNames, currentStack
}
