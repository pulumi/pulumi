// Copyright 2016-2020, Pulumi Corporation.
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

package python

import (
	"fmt"
	"os"
	"os/exec"
)

func Command(arg ...string) (*exec.Cmd, error) {
	var err error
	var pythonCmds []string
	var pythonPath string

	if pythonCmd := os.Getenv("PULUMI_PYTHON_CMD"); pythonCmd != "" {
		pythonCmds = []string{pythonCmd}
	} else {
		// Look for "python3" by default, but fallback to `python` if not found as some Python 3
		// distributions (in particular the default python.org Windows installation) do not include
		// a `python3` binary.
		pythonCmds = []string{"python3", "python"}
	}

	for _, pythonCmd := range pythonCmds {
		pythonPath, err = exec.LookPath(pythonCmd)
		// Break on the first cmd we find on the path (if any)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf(
			"Failed to locate any of %q on your PATH.  Have you installed Python 3.6 or greater?",
			pythonCmds)
	}

	return exec.Command(pythonPath, arg...), nil
}
