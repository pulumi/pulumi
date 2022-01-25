package pcl

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/pkg/v3/codegen/hcl2/syntax"
	"github.com/pulumi/pulumi/pkg/v3/codegen/internal/utils"
)

func TestBindProgram(t *testing.T) {
	testdata, err := ioutil.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	for _, v := range testdata {
		if !v.IsDir() {
			continue
		}
		folderPath := filepath.Join(testdataPath, v.Name())
		files, err := ioutil.ReadDir(folderPath)
		if err != nil {
			t.Fatalf("could not read test data: %v", err)
		}
		for _, f := range files {
			fileName := f.Name()
			if filepath.Ext(fileName) != ".pp" {
				continue
			}

			t.Run(fileName, func(t *testing.T) {
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
