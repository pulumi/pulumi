// Copyright 2025, Pulumi Corporation.
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

package packageworkspace_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageinstallation"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packageworkspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Check that [packageworkspace.Workspace] implements [packageinstallation.Context]
// without importing [packageinstallation] from [packageworkspace].
var _ packageinstallation.Context = packageworkspace.Workspace{}

func TestProjectOperationsRequireRuntime(t *testing.T) {
	t.Parallel()

	w := packageworkspace.Workspace{}
	runtime := &workspace.ProjectRuntimeInfo{}

	_, err := w.GenerateLocalSDK(t.Context(), runtime, ".", nil)
	require.EqualError(t, err, "cannot generate an SDK for a project without a runtime")

	err = w.LinkIntoProject(t.Context(), runtime, ".", nil)
	require.EqualError(t, err, "cannot link packages into a project without a runtime")
}
