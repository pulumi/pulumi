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

package httpstate

import (
	"math"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func applyStackOperationDefaults(kind apitype.UpdateKind, opts *backend.UpdateOptions,
	defaults apitype.StackOperationDefaults,
) {
	local := opts.StackOperationOverrides
	if local == nil {
		return
	}
	switch kind {
	case apitype.UpdateUpdate, apitype.PreviewUpdate, apitype.DestroyUpdate, apitype.RefreshUpdate:
		if local.Parallel != nil {
			opts.Engine.Parallel = int32(*local.Parallel)
		} else if defaults.Parallel != nil && *defaults.Parallel > 0 && *defaults.Parallel <= math.MaxInt32 {
			opts.Engine.Parallel = int32(*defaults.Parallel)
		}
	case apitype.ResourceImportUpdate, apitype.StackImportUpdate, apitype.RenameUpdate:
	}
	apply := func(value *bool, override, fallback *bool) {
		if override != nil {
			*value = *override
		} else if fallback != nil {
			*value = *fallback
		}
	}
	switch kind {
	case apitype.UpdateUpdate, apitype.PreviewUpdate:
		apply(&opts.Engine.Refresh, local.Refresh, defaults.Refresh)
		apply(&opts.Engine.RefreshProgram, local.RunProgram, defaults.RunProgram)
		if kind == apitype.UpdateUpdate {
			apply(&opts.Engine.ContinueOnError, local.ContinueOnError, defaults.ContinueOnError)
		}
	case apitype.DestroyUpdate:
		apply(&opts.Engine.Refresh, local.Refresh, defaults.Refresh)
		apply(&opts.Engine.DestroyProgram, local.RunProgram, defaults.RunProgram)
		apply(&opts.Engine.ContinueOnError, local.ContinueOnError, defaults.ContinueOnError)
	case apitype.RefreshUpdate:
		apply(&opts.Engine.RefreshProgram, local.RunProgram, defaults.RunProgram)
	case apitype.ResourceImportUpdate, apitype.StackImportUpdate, apitype.RenameUpdate:
		// Stack defaults do not apply to imports or renames.
	}
}
