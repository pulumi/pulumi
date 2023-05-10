package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ptesting "github.com/pulumi/pulumi/sdk/v3/go/common/testing"
	"github.com/stretchr/testify/require"
)

var Runtimes = []string{"python", "java", "go", "yaml", "nodejs", "dotnet"}

// Mapping from the language runtime names to the common language name used by templates and the like.
var Languages = map[string]string{
	"python": "python",
	"java":   "java",
	"go":     "go",
	"yaml":   "yaml",
	"nodejs": "typescript",
	"dotnet": "csharp",
}

// Quick sanity tests for each downstream language to check that a minimal example can be created and run.
//
//nolint:paralleltest // pulumi new is not parallel safe
func TestLanguageNewSmoke(t *testing.T) {
	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local filestate means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "new", "random-"+Languages[runtime], "--yes")
			e.RunCommand("pulumi", "up", "--yes")
			e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// Quick sanity tests for each downstream language to check that convert works.
func TestLanguageConvertSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// We need PULUMI_DEV set to use `--from pcl` in convert.
			e.SetEnvVars("PULUMI_DEV=true")

			e.ImportDirectory("testdata/random_pp")

			// Make sure random is installed
			out, _ := e.RunCommand("pulumi", "plugin", "ls")
			if !strings.Contains(out, "random  resource  4.13.0") {
				e.RunCommand("pulumi", "plugin", "install", "resource", "random", "4.13.0")
			}

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "convert", "--language", Languages[runtime], "--from", "pcl", "--out", "out")
			e.CWD = filepath.Join(e.RootPath, "out")
			e.RunCommand("pulumi", "stack", "init", "test")

			// TODO: Skipping `up` until we have a way to tell the language host to install dependencies.
			// e.RunCommand("pulumi", "up", "--yes")
			// e.RunCommand("pulumi", "destroy", "--yes")
		})
	}
}

// Quick sanity tests for each downstream language to check that sdk-gen works.
func TestLanguageGenerateSmoke(t *testing.T) {
	t.Parallel()

	for _, runtime := range Runtimes {
		if runtime == "yaml" {
			// yaml doesn't support sdks
			continue
		}

		runtime := runtime
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			e.ImportDirectory("testdata/simple_schema")
			e.RunCommand("pulumi", "package", "gen-sdk", "--language", runtime, "schema.json")
		})
	}
}

// Quick sanity tests for each downstream language to check that import works.
//
//nolint:paralleltest // pulumi new is not parallel safe
func TestLanguageImportSmoke(t *testing.T) {
	for _, runtime := range Runtimes {
		t.Run(runtime, func(t *testing.T) {
			//nolint:paralleltest

			e := ptesting.NewEnvironment(t)
			defer deleteIfNotFailed(e)

			// `new` wants to work in an empty directory but our use of local filestate means we have a
			// ".pulumi" directory at root.
			projectDir := filepath.Join(e.RootPath, "project")
			err := os.Mkdir(projectDir, 0o700)
			require.NoError(t, err)

			e.CWD = projectDir

			e.RunCommand("pulumi", "login", "--cloud-url", e.LocalURL())
			e.RunCommand("pulumi", "new", Languages[runtime], "--yes")
			e.RunCommand("pulumi", "import", "--yes", "random:index/randomId:RandomId", "identifier", "p-9hUg")
		})
	}
}
