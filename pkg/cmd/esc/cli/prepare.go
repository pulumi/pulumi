// Copyright 2023, Pulumi Corporation.
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

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/exp/maps"
)

// ProjectedVariable is a projected `environmentVariables` entry: the name of an environment variable and
// the string value a process would see for it.
type ProjectedVariable struct {
	Name   string
	Value  string
	Secret bool
}

// ProjectedFile is a projected `files` entry: the name of the environment variable that holds the path to
// the materialized temporary file. Path is that path, or "[unknown]" when files are not materialized.
type ProjectedFile struct {
	Name   string
	Path   string
	Secret bool
}

// EnvironmentProjection is the process-environment projection of a resolved environment: the
// environmentVariables and files reserved properties rendered as the string-valued variables a process
// consumes. It carries structure (per-variable secret flag, env-var vs file) that the flat dotenv/shell
// encodings discard.
type EnvironmentProjection struct {
	// Variables are the projected environment variables, sorted by name.
	Variables []ProjectedVariable
	// Files are the projected files, sorted by name.
	Files []ProjectedFile
	// Paths are the temporary files created on disk, for later cleanup.
	Paths []string
	// Secrets are the raw secret values (env var values and file contents), for redaction of command output.
	Secrets []string
}

func createTemporaryFile(fs escFS, content []byte) (string, error) {
	filename, f, err := fs.CreateTemp("", "esc-*")
	if err != nil {
		return "", err
	}
	defer contract.IgnoreClose(f)

	if _, err = f.Write(content); err != nil {
		contract.IgnoreClose(f)
		rmErr := fs.Remove(filename)
		contract.IgnoreError(rmErr)
		return "", err
	}
	return filename, nil
}

func removeTemporaryFiles(fs escFS, paths []string) {
	for _, path := range paths {
		err := fs.Remove(path)
		contract.IgnoreError(err)
	}
}

// PrepareOptions contains options for PrepareEnvironment.
type PrepareOptions struct {
	Quote   bool // True to quote environment variable values
	Pretend bool // True to skip actually writing temporary files
	Redact  bool // True to redact secrets. Ignored unless Pretend is set.

	fs escFS // The filesystem for temporary files
}

// projectEnvironment computes the process-environment projection of e, materializing temporary files unless
// opts.Pretend is set. Values are raw: quoting and redaction are presentation concerns applied by callers.
func projectEnvironment(e *esc.Environment, opts *PrepareOptions) (*EnvironmentProjection, error) {
	if opts == nil {
		opts = &PrepareOptions{}
	}
	if opts.fs == nil {
		opts.fs = newFS()
	}

	proj := &EnvironmentProjection{}

	vars := e.GetEnvironmentVariables()
	varKeys := maps.Keys(vars)
	sort.Strings(varKeys)
	for _, k := range varKeys {
		v := vars[k]
		s := v.Value.(string)
		if v.Secret {
			proj.Secrets = append(proj.Secrets, s)
		}
		proj.Variables = append(proj.Variables, ProjectedVariable{Name: k, Value: s, Secret: v.Secret})
	}

	files := e.GetTemporaryFiles()
	fileKeys := maps.Keys(files)
	sort.Strings(fileKeys)
	for _, k := range fileKeys {
		v := files[k]
		s := v.Value.(string)
		if v.Secret {
			proj.Secrets = append(proj.Secrets, s)
		}
		path := "[unknown]"
		if !opts.Pretend {
			p, err := createTemporaryFile(opts.fs, []byte(s))
			if err != nil {
				removeTemporaryFiles(opts.fs, proj.Paths)
				return nil, err
			}
			proj.Paths = append(proj.Paths, p)
			path = p
		}
		proj.Files = append(proj.Files, ProjectedFile{Name: k, Path: path, Secret: v.Secret})
	}

	return proj, nil
}

// PrepareEnvironment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs, and secret values.
func PrepareEnvironment(e *esc.Environment, opts *PrepareOptions) (files, environ, secrets []string, err error) {
	if opts == nil {
		opts = &PrepareOptions{}
	}

	proj, err := projectEnvironment(e, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating temporary files: %v", err)
	}

	for _, v := range proj.Variables {
		s := v.Value
		if v.Secret && opts.Redact {
			s = "[secret]"
		}
		if opts.Quote {
			s = strconv.Quote(s)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", v.Name, s))
	}
	for _, f := range proj.Files {
		s := f.Path
		if opts.Quote {
			s = strconv.Quote(s)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", f.Name, s))
	}

	return proj.Paths, environ, proj.Secrets, nil
}
