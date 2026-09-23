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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperationPrivileged(t *testing.T) {
	t.Parallel()

	cases := map[Operation]bool{
		OperationPreview: false,
		OperationRefresh: false,
		OperationImport:  false,
		OperationLogs:    false,
		OperationConfig:  false,
		OperationUp:      true,
		OperationDestroy: true,
		OperationWatch:   true,
		OperationDo:      true,
	}
	for op, want := range cases {
		assert.Equal(t, want, op.Privileged(), "operation %q", op)
	}
}
