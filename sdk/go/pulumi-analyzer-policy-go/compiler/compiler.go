// Copyright 2016-2023, Pulumi Corporation.
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

package compiler

import (
	"fmt"
	policyAnalyzer "github.com/pulumi/pulumi/sdk/v3/go/analyzer-policy-common"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/executable"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"os"
	"os/exec"
	"path/filepath"
)

// This function takes a file target to specify where to compile to.
// If `outfile` is "", the binary is compiled to a new temporary file.
// This function returns the path of the file that was produced.
func CompileProgram(cnf *policyAnalyzer.CompileConfig) (*policyAnalyzer.CompileResult, error) {
	goFileSearchPattern := filepath.Join(cnf.ProgramDirectory, "*.go")
	if matches, err := filepath.Glob(goFileSearchPattern); err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("Failed to find go files for 'go build' matching %s", goFileSearchPattern)
	}

	if cnf.OutFile == "" {
		// If no outfile is supplied, write the Go binary to a temporary file.
		f, err := os.CreateTemp("", "pulumi-go.*")
		if err != nil {
			return nil, fmt.Errorf("unable to create go program temp file: %w", err)
		}

		if err := f.Close(); err != nil {
			return nil, fmt.Errorf("unable to close go program temp file: %w", err)
		}
		cnf.OutFile = f.Name()
	}

	gobin, err := executable.FindExecutable("go")
	if err != nil {
		return nil, fmt.Errorf("unable to find 'go' executable: %w", err)
	}
	logging.V(5).Infof("Attempting to build go program in %s with: %s build -o %s", cnf.ProgramDirectory, gobin, cnf.OutFile)
	buildCmd := exec.Command(gobin, "build", "-o", cnf.OutFile)
	buildCmd.Dir = cnf.ProgramDirectory
	buildCmd.Stdout, buildCmd.Stderr = os.Stdout, os.Stderr

	if err := buildCmd.Run(); err != nil {
		return nil, fmt.Errorf("unable to run `go build`: %w", err)
	}

	return &policyAnalyzer.CompileResult{
		Program: cnf.OutFile,
	}, nil
}
