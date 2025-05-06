// Copyright 2020-2024, Pulumi Corporation.
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

package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/pgavlin/goldmark/ast"
	"github.com/pgavlin/goldmark/testutil"
	"github.com/pulumi/pulumi/pkg/v3/codegen/testing/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note to future engineers: keep each file tested as a single test, do not use `t.Run` in the inner
// loops.
//
// Time to complete on these tests increases from ~2s to 30s or more and the number of lines logged
// to stdout from 46 lines to over 1,000,000 lines of output. This corresponds to the roughly 1
// million doc items tested across each file.
//
// Aside from just being verbose, the voluminous output makes `gotestsum` analysis less useful and
// prevents use of the `ci-matrix` tool.

var testdataPath = filepath.Join("..", "testing", "test", "testdata")

var nodeAssertions = testutil.DefaultNodeAssertions().Union(testutil.NodeAssertions{
	KindShortcode: func(t *testing.T, sourceExpected, sourceActual []byte, expected, actual ast.Node) bool {
		shortcodeExpected, shortcodeActual := expected.(*Shortcode), actual.(*Shortcode)
		return testutil.AssertEqualBytes(t, shortcodeExpected.Name, shortcodeActual.Name)
	},
})

type doc struct {
	entity  string
	content string
}

func getDocsForProperty(parent string, p *Property) []doc {
	entity := path.Join(parent, p.Name)
	return []doc{
		{entity: entity + "/description", content: p.Comment},
		{entity: entity + "/deprecationMessage", content: p.DeprecationMessage},
	}
}

func getDocsForObjectType(path string, t *ObjectType) []doc {
	if t == nil {
		return nil
	}

	docs := []doc{{entity: path + "/description", content: t.Comment}}
	for _, p := range t.Properties {
		docs = append(docs, getDocsForProperty(path+"/properties", p)...)
	}
	return docs
}

func getDocsForFunction(f *Function) []doc {
	entity := "#/functions/" + url.PathEscape(f.Token)
	docs := []doc{
		{entity: entity + "/description", content: f.Comment},
		{entity: entity + "/deprecationMessage", content: f.DeprecationMessage},
	}
	docs = append(docs, getDocsForObjectType(entity+"/inputs/properties", f.Inputs)...)

	if f.ReturnType != nil {
		if objectType, ok := f.ReturnType.(*ObjectType); ok && objectType != nil {
			docs = append(docs, getDocsForObjectType(entity+"/outputs/properties", objectType)...)
		}
	}

	return docs
}

func getDocsForResource(r *Resource, isProvider bool) []doc {
	var entity string
	if isProvider {
		entity = "#/provider"
	} else {
		entity = "#/resources/" + url.PathEscape(r.Token)
	}

	docs := []doc{
		{entity: entity + "/description", content: r.Comment},
		{entity: entity + "/deprecationMessage", content: r.DeprecationMessage},
	}
	for _, p := range r.InputProperties {
		docs = append(docs, getDocsForProperty(entity+"/inputProperties", p)...)
	}
	for _, p := range r.Properties {
		docs = append(docs, getDocsForProperty(entity+"/properties", p)...)
	}
	docs = append(docs, getDocsForObjectType(entity+"/stateInputs", r.StateInputs)...)
	return docs
}

func getDocsForPackage(pkg *Package) []doc {
	var allDocs []doc
	for _, p := range pkg.Config {
		allDocs = append(allDocs, getDocsForProperty("#/config/variables", p)...)
	}
	for _, f := range pkg.Functions {
		allDocs = append(allDocs, getDocsForFunction(f)...)
	}
	allDocs = append(allDocs, getDocsForResource(pkg.Provider, true)...)
	for _, r := range pkg.Resources {
		allDocs = append(allDocs, getDocsForResource(r, false)...)
	}
	for _, t := range pkg.Types {
		if obj, ok := t.(*ObjectType); ok {
			allDocs = append(allDocs, getDocsForObjectType("#/types", obj)...)
		}
	}
	return allDocs
}

//nolint:paralleltest // needs to set plugin acquisition env var
func TestParseAndRenderDocs(t *testing.T) {
	files, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	//nolint:paralleltest // needs to set plugin acquisition env var
	for _, f := range files {
		f := f
		if filepath.Ext(f.Name()) != ".json" || strings.Contains(f.Name(), "awsx") {
			continue
		}

		t.Run(f.Name(), func(t *testing.T) {
			t.Setenv("PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION", "false")

			path := filepath.Join(testdataPath, f.Name())
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read %v: %v", path, err)
			}

			var spec PackageSpec
			if err = json.Unmarshal(contents, &spec); err != nil {
				t.Fatalf("could not unmarshal package spec: %v", err)
			}
			pkg, err := ImportSpec(spec, nil, ValidationOptions{
				AllowDanglingReferences: true,
			})
			if err != nil {
				t.Fatalf("could not import package: %v", err)
			}

			//nolint:paralleltest // these are large, compute heavy tests. keep them in a single thread
			for _, doc := range getDocsForPackage(pkg) {
				doc := doc
				original := []byte(doc.content)
				expected := ParseDocs(original)
				rendered := []byte(RenderDocsToString(original, expected))
				actual := ParseDocs(rendered)
				if !testutil.AssertSameStructure(t, original, rendered, expected, actual, nodeAssertions) {
					t.Logf("original: %v", doc.content)
					t.Logf("rendered: %v", string(rendered))
				}
			}
		})
	}
}

func pkgInfo(t *testing.T, filename string) (string, *semver.Version) {
	filename = strings.TrimSuffix(filename, ".json")

	for idx := range len(filename) {
		i := strings.IndexByte(filename[idx:], '-') + idx
		require.Truef(t, i != -1, "Could not parse %q into (pkg, version)", filename)
		name := filename[:i]
		version := filename[i+1:]
		if v, err := semver.Parse(version); err == nil {
			return name, &v
		}
	}

	require.Failf(t, "invalid filename", "%q is not suffixed with a semver version", filename)
	return "", nil
}

func TestReferenceRenderer(t *testing.T) {
	t.Parallel()

	files, err := os.ReadDir(testdataPath)
	if err != nil {
		t.Fatalf("could not read test data: %v", err)
	}

	seenNames := map[string]struct{}{}

	//nolint:paralleltest // false positive because range var isn't used directly in t.Run(name) arg
	for _, f := range files {
		f := f
		if filepath.Ext(f.Name()) != ".json" || f.Name() == "types.json" {
			continue
		}
		name, version := pkgInfo(t, f.Name())

		if _, ok := seenNames[name]; ok {
			continue
		}
		seenNames[name] = struct{}{}

		t.Run(f.Name(), func(t *testing.T) {
			t.Parallel()

			host := utils.NewHost(testdataPath)
			defer host.Close()
			loader := NewPluginLoader(host)
			pkg, err := loader.LoadPackage(name, version)
			if err != nil {
				t.Fatalf("could not import package %s,%s: %v", name, version, err)
			}

			//nolint:paralleltest // these are large, compute heavy tests. keep them in a single thread
			for _, doc := range getDocsForPackage(pkg) {
				doc := doc

				text := []byte(fmt.Sprintf("[entity](%s)", doc.entity))
				expected := strings.ReplaceAll(doc.entity, "/", "_") + "\n"

				parsed := ParseDocs(text)
				actual := []byte(RenderDocsToString(text, parsed, WithReferenceRenderer(
					func(r *Renderer, w io.Writer, src []byte, l *ast.Link, enter bool) (ast.WalkStatus, error) {
						if !enter {
							return ast.WalkContinue, nil
						}

						replaced := bytes.Replace(l.Destination, []byte{'/'}, []byte{'_'}, -1)
						if _, err := r.MarkdownRenderer().Write(w, replaced); err != nil {
							return ast.WalkStop, err
						}

						return ast.WalkSkipChildren, nil
					})))

				assert.Equal(t, expected, string(actual))
			}
		})
	}
}
