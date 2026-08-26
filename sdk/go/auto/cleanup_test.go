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

package auto

import (
	"context"
	"strings"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/auto/optremove"
)

// removeStackOnCleanup removes the stack when the test ends; register it immediately after
// creating one. Forced, because a test aborting between `up` and `destroy` leaves resources in the
// checkpoint and an unforced removal is then rejected. Nothing here manages real infrastructure.
func removeStackOnCleanup(t *testing.T, s *Stack) {
	t.Helper()
	t.Cleanup(func() {
		//nolint:usetesting // t.Context is already canceled by the time cleanup functions run.
		err := s.Workspace().RemoveStack(context.Background(), s.Name(), optremove.Force())
		if err != nil && !strings.Contains(err.Error(), "no stack named") {
			t.Errorf("failed to remove stack %q, resources have leaked: %v", s.Name(), err)
		}
	})
}
