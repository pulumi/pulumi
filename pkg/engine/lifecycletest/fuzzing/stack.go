// Copyright 2024, Pulumi Corporation.
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

package fuzzing

// A set of options for configuring stacks used by rapid.Generators in this package.
type StackSpecOptions struct {
	Project string
	Stack   string
}

// Returns a copy of the StackSpecOptions with the given overrides applied.
func (sso StackSpecOptions) With(overrides StackSpecOptions) StackSpecOptions {
	if overrides.Project != "" {
		sso.Project = overrides.Project
	}
	if overrides.Stack != "" {
		sso.Stack = overrides.Stack
	}

	return sso
}

// A default set of StackSpecOptions.
var defaultStackSpecOptions = StackSpecOptions{
	Project: "test-project",
	Stack:   "test-stack",
}
