package pcl

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
)

func TestBindProgram(t *testing.T) {
	t.Parallel()

	testdata, err := ioutil.ReadDir(testdataPath)
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
		files, err := ioutil.ReadDir(folderPath)
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

				_, diags, err := BindProgram(parser.Files, PluginHost(utils.NewHost(testdataPath)))
				assert.NoError(t, err)
				if diags.HasErrors() {
					t.Fatalf("failed to bind program: %v", diags)
				}
			})
		}
	}
}
