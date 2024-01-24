// Copyright 2016-2024, Pulumi Corporation.
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

package workspace

// FixupPythonDefaultVirtualenv sets the default virtualenv for Python projects.
//
// This is a workaround for Python projects, most of our templates don't specify a venv but we want
// to use one by default.
func (r *ProjectRuntimeInfo) FixupPythonDefaultVirtualenv() {
	if r.Name() == "python" {
		// If the template does give virtualenv use it, else default to "venv"
		if _, has := r.Options()["virtualenv"]; !has {
			r.SetOption("virtualenv", "venv")
		}
	}
}
