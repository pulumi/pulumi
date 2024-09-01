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

package policyAnalyzer

type GolangConfig struct{}

type DotnetConfig struct{}

type CompileConfig struct {
	ProgramDirectory, OutFile string

	GolangConfig GolangConfig
	DotnetConfig DotnetConfig
}

type CompileResult struct {
	Program string
}

// CompileProgramFunc function takes a file target to specify where to compile to.
// If `outfile` is "", the binary is compiled to a new temporary file.
// This function returns the path of the file that was produced.
type CompileProgramFunc func(*CompileConfig) (*CompileResult, error)
