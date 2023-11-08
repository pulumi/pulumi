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

	"github.com/pulumi/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"golang.org/x/exp/maps"
)

func getEnvironmentVariables(env *esc.Environment, quote, redact bool) (environ, secrets []string) {
	vars := env.GetEnvironmentVariables()
	keys := maps.Keys(vars)
	sort.Strings(keys)

	for _, k := range keys {
		v := vars[k]
		s := v.Value.(string)

		if v.Secret {
			secrets = append(secrets, s)
			if redact {
				s = "[secret]"
			}
		}
		if quote {
			s = strconv.Quote(s)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", k, s))
	}
	return environ, secrets
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

func createTemporaryFiles(e *esc.Environment, opts PrepareOptions) (paths, environ, secrets []string, err error) {
	files := e.GetTemporaryFiles()
	keys := maps.Keys(files)
	sort.Strings(keys)

	for _, k := range keys {
		v := files[k]
		s := v.Value.(string)

		if v.Secret {
			secrets = append(secrets, s)
		}

		path := "[unknown]"
		if !opts.Pretend {
			path, err = createTemporaryFile(opts.fs, []byte(s))
			if err != nil {
				removeTemporaryFiles(opts.fs, paths)
				return nil, nil, nil, err
			}
			paths = append(paths, path)
		}
		if opts.Quote {
			path = strconv.Quote(path)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", k, path))
	}
	return paths, environ, secrets, nil
}

// PrepareOptions contains options for PrepareEnvironment.
type PrepareOptions struct {
	Quote   bool // True to quote environment variable values
	Pretend bool // True to skip actually writing temporary files
	Redact  bool // True to redact secrets. Ignored unless Pretend is set.

	fs escFS // The filesystem for temporary files
}

// PrepareEnvironment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs, and secret values.
func PrepareEnvironment(e *esc.Environment, opts *PrepareOptions) (files, environ, secrets []string, err error) {
	if opts == nil {
		opts = &PrepareOptions{}
	}
	if opts.fs == nil {
		opts.fs = newFS()
	}

	envVars, envSecrets := getEnvironmentVariables(e, opts.Quote, opts.Redact)

	filePaths, fileVars, fileSecrets, err := createTemporaryFiles(e, *opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating temporary files: %v", err)
	}

	environ = append(envVars, fileVars...)
	secrets = append(envSecrets, fileSecrets...)
	return filePaths, environ, secrets, nil
}
