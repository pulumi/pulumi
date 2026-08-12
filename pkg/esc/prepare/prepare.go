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

// Package prepare computes the environment variable and temporary file projections of an open ESC environment.
package prepare

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Options contains options for Environment and EnvironmentMap.
type Options struct {
	Quote   bool // True to quote environment variable values. Ignored by EnvironmentMap.
	Pretend bool // True to skip actually writing temporary files
	Redact  bool // True to redact secrets. Ignored unless Pretend is set.

	FS FS // The filesystem for temporary files. Defaults to the OS filesystem.
}

// keyValue is a single environment variable projection.
type keyValue struct {
	key   string
	value string
}

func getEnvironmentVariables(env *esc.Environment, redact bool) (vars []keyValue, secrets []string) {
	values := env.GetEnvironmentVariables()
	for _, k := range slices.Sorted(maps.Keys(values)) {
		v := values[k]
		s, ok := v.Value.(string)
		if !ok {
			continue
		}

		if v.Secret {
			secrets = append(secrets, s)
			if redact {
				s = "[secret]"
			}
		}
		vars = append(vars, keyValue{k, s})
	}
	return vars, secrets
}

func createTemporaryFile(fs FS, content []byte) (string, error) {
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

// RemoveTemporaryFiles removes the temporary files created by Environment or EnvironmentMap.
func RemoveTemporaryFiles(fs FS, paths []string) {
	for _, path := range paths {
		err := fs.Remove(path)
		contract.IgnoreError(err)
	}
}

func createTemporaryFiles(
	e *esc.Environment, opts Options,
) (paths []string, vars []keyValue, secrets []string, err error) {
	files := e.GetTemporaryFiles()
	for _, k := range slices.Sorted(maps.Keys(files)) {
		v := files[k]
		s, ok := v.Value.(string)
		if !ok {
			continue
		}

		if v.Secret {
			secrets = append(secrets, s)
		}

		path := "[unknown]"
		if !opts.Pretend {
			path, err = createTemporaryFile(opts.FS, []byte(s))
			if err != nil {
				RemoveTemporaryFiles(opts.FS, paths)
				return nil, nil, nil, err
			}
			paths = append(paths, path)
		}
		vars = append(vars, keyValue{k, path})
	}
	return paths, vars, secrets, nil
}

// prepare projects an environment into sorted environment variable pairs, the paths of any temporary files that were
// created, and the environment's secret values.
func prepare(e *esc.Environment, opts *Options) (files []string, vars []keyValue, secrets []string, err error) {
	if opts == nil {
		opts = &Options{}
	}
	if opts.FS == nil {
		opts.FS = NewFS()
	}

	envVars, envSecrets := getEnvironmentVariables(e, opts.Redact)

	filePaths, fileVars, fileSecrets, err := createTemporaryFiles(e, *opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating temporary files: %v", err)
	}

	return filePaths, append(envVars, fileVars...), append(envSecrets, fileSecrets...), nil
}

// Environment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs of the form "NAME=value", and secret values.
func Environment(e *esc.Environment, opts *Options) (files, environ, secrets []string, err error) {
	quote := opts != nil && opts.Quote

	files, vars, secrets, err := prepare(e, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, kv := range vars {
		v := kv.value
		if quote {
			v = strconv.Quote(v)
		}
		environ = append(environ, fmt.Sprintf("%v=%v", kv.key, v))
	}
	return files, environ, secrets, nil
}

// EnvironmentMap is Environment, but returns the environment variables as a map.
func EnvironmentMap(e *esc.Environment, opts *Options) (envVars map[string]string, secrets, files []string, err error) {
	files, vars, secrets, err := prepare(e, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	if len(vars) > 0 {
		envVars = make(map[string]string, len(vars))
		for _, kv := range vars {
			envVars[kv.key] = kv.value
		}
	}
	return envVars, secrets, files, nil
}
