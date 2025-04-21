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

package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProjectCmd(t *testing.T) {
	t.Parallel()
	cmd := NewProjectCmd()
	assert.Equal(t, "project", cmd.Use)
	assert.Equal(t, "Manage Pulumi projects", cmd.Short)
	assert.Equal(t, 1, len(cmd.Commands()))

	// Check ls command
	lsCmd := cmd.Commands()[0]
	assert.Equal(t, "ls", lsCmd.Use)
	assert.Equal(t, "List your Pulumi projects", lsCmd.Short)
}
