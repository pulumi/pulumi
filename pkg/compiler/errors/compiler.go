// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

// Compiler errors are in the [100-200) range.
var (
	ErrorIO                        = newError(100, "An IO error occurred during the current operation: %v")
	ErrorMissingProject            = newError(101, "No project was found underneath the given path: %v")
	ErrorCouldNotReadProject       = newError(102, "An IO error occurred while reading the project: %v")
	ErrorCouldNotReadPackage       = newError(103, "An IO error occurred while reading the package: %v")
	ErrorIllegalProjectSyntax      = newError(104, "A syntax error was detected while parsing the project: %v")
	ErrorIllegalWorkspaceSyntax    = newError(105, "A syntax error was detected while parsing workspace settings: %v")
	WarningIllegalMarkupFileCasing = newWarning(106, "A %v-like file was located, but it has incorrect casing")
	WarningIllegalMarkupFileExt    = newWarning(
		107, "A %v-like file was located, but %v isn't a valid file extension (expected .json or .yaml)")
)
