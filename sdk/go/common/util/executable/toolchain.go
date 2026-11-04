// Copyright 2026, Pulumi Corporation.
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

package executable

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToolchainNotFoundError reports that a language runtime could not find the toolchain it needs on the PATH.
type ToolchainNotFoundError struct {
	// Runtime is the project's Pulumi runtime, for example "go".
	Runtime string
	// Toolchain names what has to be installed, for example "Go" or "Python 3.10 or later".
	Toolchain string
	// Programs lists the executables that were looked for.
	Programs []string
	// InstallURL points at installation instructions.
	InstallURL string
}

func (e *ToolchainNotFoundError) Error() string {
	quoted := make([]string, len(e.Programs))
	for i, program := range e.Programs {
		quoted[i] = "'" + program + "'"
	}
	missing := strings.Join(quoted, " or ") + " was not found"
	if len(quoted) > 1 {
		missing = "none of " + strings.Join(quoted, ", ") + " was found"
	}
	return fmt.Sprintf("the %s runtime requires %s, but %s on your PATH. See %s for installation instructions",
		e.Runtime, e.Toolchain, missing, e.InstallURL)
}

// GRPCStatus reports the error as a failed precondition, so that the engine shows the message as is instead of
// attributing it to the operation it was running.
func (e *ToolchainNotFoundError) GRPCStatus() *status.Status {
	return status.New(codes.FailedPrecondition, e.Error())
}
