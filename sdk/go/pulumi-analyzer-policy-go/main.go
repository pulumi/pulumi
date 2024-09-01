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

package main

import (
	gocompiler "github.com/pulumi/pulumi/sdk/go/pulumi-analyzer-policy-go/v3/compiler"
	policyAnalyzer "github.com/pulumi/pulumi/sdk/v3/go/analyzer-policy-common"
)

// Launches the language host, which in turn fires up an RPC server implementing the LanguageRuntimeServer endpoint.
func main() {
	policyAnalyzer.Main(&policyAnalyzer.MainConfig{
		GetAnalyzer: policyAnalyzer.GetAnalyzerWithCompilerFunc(gocompiler.CompileProgram),
	})
}
