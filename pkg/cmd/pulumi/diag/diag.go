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

package diag

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// PrintDiagnostics prints the given diagnostics to the given diagnostic sink.
func PrintDiagnostics(sink diag.Sink, diagnostics hcl.Diagnostics) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == hcl.DiagError {
			sink.Errorf(diag.Message("", "%s"), diagnostic)
		} else {
			sink.Warningf(diag.Message("", "%s"), diagnostic)
		}
	}
}

func FormatCyclicInstallError(
	ctx context.Context, err packageinstallation.ErrorCyclicDependencies,
	wd string,
) error {
	cyclePath := make([]string, len(err.Cycle))
	for i, n := range err.Cycle {
		name := n.Name
		if plugin.IsLocalPluginPath(ctx, n.Name) {
			rel, err := filepath.Rel(wd, n.Name)
			if err == nil {
				name = rel
			}
		}
		if n.Version != nil {
			name += "@" + n.Version.String()
		}
		cyclePath[i] = name
	}
	return fmt.Errorf("cycle found: %s", strings.Join(cyclePath, " -> "))
}
