package integration

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"

	"github.com/pulumi/pulumi/pkg/testing"
	"github.com/pulumi/pulumi/pkg/workspace"
	"github.com/stretchr/testify/assert"
)

// GetSettings returns the contents of the settings.json in the .pulumi folder located in the
// current working directory of the provided environment. Any IO errors fails the test.
func GetSettings(e *testing.Environment) workspace.Repository {
	if !e.PathExists(".pulumi/settings.json") {
		e.Fatalf("did not find .pulumi/settings.json")
	}

	path := path.Join(e.CWD, ".pulumi/settings.json")
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		e.Fatalf("error reading settings.json: %v", err)
	}

	var settings workspace.Repository
	err = json.Unmarshal(contents, &settings)
	if err != nil {
		e.Fatalf("error unmarshalling JSON: %v", err)
	}

	return settings
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
