package pcl

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

var fileToMockPlugins = map[string]map[string]string{
	"aws-resource-options.pp": {
		"aws": "4.38.0",
	},
}

func TestBindProgram(t *testing.T) {
	t.Parallel()

	testdata, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, v := range testdata {
		v := v
		if !v.IsDir() {
			continue
		}
		folderPath := filepath.Join(testdataPath, v.Name())
		files, err := os.ReadDir(folderPath)
		if err != nil {
			t.Fatalf("could not read test data: %v", err)
		}
		for _, fileName := range files {
			fileName := fileName.Name()
			if filepath.Ext(fileName) != ".pp" {
				continue
			}

			t.Run(fileName, func(t *testing.T) {
				t.Parallel()

				path := filepath.Join(folderPath, fileName)
				contents, err := ioutil.ReadFile(path)
				if err != nil {
					t.Fatalf("could not read %v: %v", path, err)
				}

				parser := syntax.NewParser()
				err = parser.ParseFile(bytes.NewReader(contents), fileName)
				if err != nil {
					t.Fatalf("could not read %v: %v", path, err)
				}
				if parser.Diagnostics.HasErrors() {
					t.Fatalf("failed to parse files: %v", parser.Diagnostics)
				}

				var bindError error
				var diags hcl.Diagnostics
				if fileName == "simple-range.pp" {
					// simple-range.pp requires AllowMissingVariables
					// TODO: remove this once we have a better way to handle this
					// https://github.com/pulumi/pulumi/issues/10985
					_, diags, bindError = BindProgram(
						parser.Files,
						PluginHost(utils.NewHost(testdataPath, fileToMockPlugins[fileName])),
						AllowMissingVariables)
				} else {
					// all other PCL files use a more restrict program bind
					_, diags, bindError = BindProgram(
						parser.Files,
						PluginHost(utils.NewHost(testdataPath, fileToMockPlugins[fileName])))
				}

				assert.NoError(t, bindError)
				if diags.HasErrors() {
					t.Fatalf("failed to bind program: %v", diags)
				}
			})
		}
	}
}
